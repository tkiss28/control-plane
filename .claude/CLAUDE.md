# DCM Control-Plane

## Architecture Overview

The **control-plane** is a **monolith** that consolidates four previously separate microservices into one binary, one database, and one HTTP API. It manages the lifecycle of cloud resources through a catalog-driven approach.

## Main Components

### 1. Catalog Manager (`internal/catalog/`)

**What it does:**
- Manages the **service catalog** - the "shopping menu" of available services
- Stores **catalog items** (templates like "create a container", "provision a VM")
- Stores **catalog item instances** (actual user requests)
- Acts as the **entry point** for users creating resources

**Key concepts:**
- **Service Types**: Categories of services (cluster, container, database, VM)
- **Catalog Items**: Specific offerings (e.g., "PostgreSQL Database", "RHEL 9 VM")
- **Catalog Item Instances**: User's actual requests (e.g., "Give me a PostgreSQL DB with 10GB storage")

**API endpoints:**
- `GET /api/v1alpha1/service-types` - List available service types
- `GET /api/v1alpha1/catalog-items` - List catalog items
- `POST /api/v1alpha1/catalog-item-instances` - **Create a resource**
- `GET /api/v1alpha1/catalog-item-instances/{id}` - Get instance status

### 2. Placement Manager (`internal/placement/`)

**What it does:**
- Decides **where** and **how** to provision a resource
- Coordinates policy evaluation and resource provisioning
- Translates user requests into provider-specific specs

**Key responsibilities:**
- Evaluates policies to select the right provider
- Calls Policy Manager for approval
- Calls Service Provider Manager to provision the resource
- Tracks resource lifecycle

**Does NOT have its own HTTP API** (internal only) - called by Catalog Manager via local client

### 3. Policy Manager (`internal/policy/`)

**What it does:**
- **Validates** resource requests against governance rules
- **Selects** the appropriate service provider
- Uses **Open Policy Agent (OPA)** with **Rego** policies

**Key concepts:**
- **Policies**: Rego rules that determine if a request is allowed and which provider to use
- **Evaluation**: Takes a resource spec, returns approval status + selected provider

**API endpoints:**
- `GET /api/v1alpha1/policies` - List policies
- `POST /api/v1alpha1/policies` - Create a policy
- `PATCH /api/v1alpha1/policies/{id}` - Update a policy
- **Internal only**: `evaluateRequest` - called by Placement Manager

**Example policy logic:**
```rego
# If requesting a container in region "us-east"
# → Use "k8s-container-provider"
# → Status: APPROVED
```

### 4. Service Provider (SP) Manager (`internal/sp/`)

**What it does:**
- Manages the **registry** of service providers
- Tracks **service instances** (actual provisioned resources)
- Monitors provider **health**
- Handles instance **lifecycle** (create, delete, status updates)

**Two sub-APIs:**

1. **Provider API** - Service providers register themselves
   - `POST /api/v1alpha1/providers` - Register a provider
   - `GET /api/v1alpha1/providers` - List providers

2. **Resource Manager API** - Internal provisioning
   - `POST /api/v1alpha1/service-type-instances` - Provision a resource
   - `GET /api/v1alpha1/service-type-instances/{id}` - Get status
   - `DELETE /api/v1alpha1/service-type-instances/{id}` - Delete resource

**Background jobs:**
- **Health monitor**: Polls registered providers for liveness
- **Status consumer**: Listens to NATS for async status updates from providers
- **Cleanup scheduler**: Retries failed deletions

### 5. Other Components

**NATS** (Message Queue)
- Service providers publish status updates (e.g., "container is running")
- SP Manager consumes these updates asynchronously
- Decouples providers from the control-plane

**PostgreSQL** (Database)
- Stores all state: catalog items, instances, policies, providers, resources
- Each domain has its own tables but shares one DB

**dcm-ui** (Frontend)
- Web UI for browsing the catalog and creating resources
- Backstage-based (Red Hat Developer Hub)
- Talks to control-plane API at `/api/v1alpha1`

## Request Flow: Creating a Container

When a user creates a containerized workload:

```
1. USER/UI
   POST /api/v1alpha1/catalog-item-instances
   {
     "spec": {
       "catalog_item_id": "postgres-container",
       "user_values": {"storage": "10Gi"}
     }
   }
   ↓

2. CATALOG MANAGER (internal/catalog/handlers/v1alpha1/)
   a) Validate request
   b) Look up catalog item "postgres-container"
   c) Merge user_values with catalog item defaults
   d) Build full resource spec
   e) Save to DB: catalog_item_instances table
   f) Call Placement Manager →

3. PLACEMENT MANAGER (internal/placement/service/)
   a) Receive resource spec
   b) Call Policy Manager for evaluation →

4. POLICY MANAGER (internal/policy/service/)
   a) Load all active policies from DB
   b) Run OPA evaluation with Rego rules
   c) Determine:
      - approval_status: "APPROVED" / "REJECTED"
      - selected_provider: "k8s-container-provider"
      - evaluated_spec: spec with policy-injected values
   d) Return result ←

5. PLACEMENT MANAGER (continued)
   c) Save to DB: resources table
   d) Call SP Resource Manager →

6. SP MANAGER - Resource Manager (internal/sp/service/resource_manager/)
   a) Look up provider "k8s-container-provider" in registry
   b) Create service_type_instances DB record
   c) Send HTTP request to external provider:
      POST http://k8s-container-provider:8080/.../instances
   d) Provider creates the actual Kubernetes Pod
   e) Return initial status ←

7. RESPONSE
   SP Manager → Placement → Catalog → User
   
   User receives:
   {
     "id": "abc-123",
     "state": "PROVISIONING",
     "spec": {...}
   }
```

### Async Status Updates

After the initial response, status updates happen asynchronously:

```
Service Provider (k8s-container-provider)
  → Kubernetes Pod created
  → Publish to NATS topic: "service-type-instances.status"

NATS
  ↓

SP Manager - Status Consumer (background goroutine)
  a) Consume message
  b) Update DB: state = "READY"

User polls: GET /catalog-item-instances/abc-123
  → Returns updated state: "READY"
```

## Database Schema (Simplified)

```
catalog_items
├── id (PK)
├── display_name
├── service_type_id
└── spec (JSONB)

catalog_item_instances
├── id (PK)
├── catalog_item_id (FK)
├── resource_id
├── user_values (JSONB)
└── state

policies
├── id (PK)
├── display_name
├── rego_module (text)
└── compiled_module (bytes)

resources (placement)
├── id (PK)
├── catalog_item_instance_id
├── approval_status
├── provider_name
└── spec (JSONB)

providers
├── id (PK)
├── name
├── endpoint_url
└── health_status

service_type_instances
├── id (PK)
├── provider_id (FK)
├── spec (JSONB)
└── state
```

## Key Design Patterns

### 1. Local Clients for Inter-Service Communication

Since all services are in one binary, they call each other via **local in-memory clients** instead of HTTP:

```go
// In run.go
policyClient := placementpolicy.NewServiceClient(evaluationService)
sprmClient := placementsprm.NewServiceClient(spInstanceService)
placementService := placementservice.NewPlacementService(..., policyClient, sprmClient)
pmClient := catalogplacement.NewLocalClient(placementService, logger)
```

No network overhead - just function calls!

### 2. DB-First Pattern

Write to the database **before** calling external services. If the external call fails, rollback the DB:

```go
// Create in DB first
created, err := s.store.Create(ctx, model)
if err != nil { return err }

// Then call external service
_, err = s.pmClient.CreateResource(ctx, req)
if err != nil {
    // Rollback DB on failure
    s.rollbackDelete(created.ID)
    return err
}
```

### 3. Layered Architecture

- **Handler** → HTTP request/response, validation
- **Service** → Business logic, orchestration
- **Store** → Database operations

Each layer only talks to the layer below it.

## Monolith Consolidation History

Gloria Ciavarrini consolidated the control-plane in June 2026:

- **PR #1**: Imported catalog-manager
- **PR #4**: Imported service-provider-manager
- **PR #5**: **Major consolidation** - wired catalog, placement, policy, and SP into one binary
  - Removed separate `cmd/*-manager` binaries
  - Created unified `internal/app/` with shared config, database, and HTTP server
  - Added local clients for inter-service communication (in-process instead of HTTP)
  - Net result: -3,133 lines, +2,468 lines
- **PR #10**: Consolidated platform deployment (Docker Compose + Helm) from api-gateway repo

This transformation converted a **distributed microservices architecture into a monolith**, simplifying:
- Deployment (one container vs. four)
- Development (one database, one binary to debug)
- Operations (single process to monitor)
- Inter-service communication (in-process function calls vs. HTTP)

## Local Development

**Run locally:**
```bash
make run              # SQLite, no containers
make run-dev          # With Postgres/NATS (requires local Postgres/NATS)
make compose-up       # Full platform stack in containers
```

**Build:**
```bash
make build            # Build control-plane binary
make image-build      # Build container image
```

**Testing:**
```bash
make test             # Unit tests
make lint             # Run linters
```

## Deployment

**Docker Compose:**
- See `deploy/RUN.md` for local stack usage
- Control-plane API: `http://localhost:8080`
- DCM UI: `http://localhost:7007`

**Kubernetes/OpenShift:**
- Helm chart at `deploy/helm/dcm`
- See `deploy/helm/dcm/README.md` for cluster installs
- OpenShift Routes enabled by default

**Container Images:**
- CI pushes to `quay.io/dcm-project/control-plane`
- Tags: `main`, `v*` (semver), `<short-sha>`
