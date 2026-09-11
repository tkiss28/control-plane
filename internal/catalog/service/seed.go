package service

import (
	"context"

	"github.com/dcm-project/control-plane/api/catalog/v1alpha1/servicetypes/container"
	"github.com/dcm-project/control-plane/api/catalog/v1alpha1/servicetypes/database"
	"github.com/dcm-project/control-plane/api/catalog/v1alpha1/servicetypes/network"
	"github.com/dcm-project/control-plane/api/catalog/v1alpha1/servicetypes/three_tier_app_demo"
	"github.com/dcm-project/control-plane/api/catalog/v1alpha1/servicetypes/vm"
	"github.com/dcm-project/control-plane/internal/catalog/store/model"
)

// Seed ensures required service types and default catalog items exist.
func (s *service) Seed(ctx context.Context) error {
	s.logger.InfoContext(ctx, "Seeding database with defaults")
	if err := s.store.ServiceType().SeedMissing(ctx, defaultServiceTypes()); err != nil {
		return err
	}
	return s.store.CatalogItem().SeedIfEmpty(ctx, s.defaultCatalogItems())
}

func defaultServiceTypes() []model.ServiceType {
	emptyNetwork := &three_tier_app_demo.Network{}
	return []model.ServiceType{
		{
			ID:          "three-tier-app-demo",
			ApiVersion:  "v1alpha1",
			ServiceType: "three-tier-app-demo",
			Spec: map[string]any{
				"database": three_tier_app_demo.DatabaseTier{Engine: three_tier_app_demo.DefaultDatabaseEngine, Version: three_tier_app_demo.DefaultDatabaseVersion, Network: emptyNetwork},
				"app":      three_tier_app_demo.AppTier{Image: "", Network: emptyNetwork},
				"web":      three_tier_app_demo.WebTier{Image: "", Network: emptyNetwork},
			},
			// three-tier-app-demo is a composite demo type; no cross-provider output fields.
			OutputSpec: nil,
			Path:       "service-types/three-tier-app-demo",
		},
		{
			ID:          "vm",
			ApiVersion:  "v1alpha1",
			ServiceType: "vm",
			Spec: map[string]any{
				"vcpu":     vm.Vcpu{},
				"memory":   vm.Memory{},
				"storage":  vm.Storage{},
				"guest_os": vm.GuestOS{},
				"access":   vm.Access{},
				"ip":       []vm.VmEndpoint{},
			},
			OutputSpec: outputSpecVM(),
			Path:       "service-types/vm",
		},
		{
			ID:          "container",
			ApiVersion:  "v1alpha1",
			ServiceType: "container",
			Spec: map[string]any{
				"image":     container.Image{},
				"resources": container.ContainerResources{},
				"process":   container.Process{},
				"network":   container.Network{},
				"endpoints": []container.ContainerEndpoint{},
			},
			OutputSpec: outputSpecContainer(),
			Path:       "service-types/container",
		},
		{
			ID:          "database",
			ApiVersion:  "v1alpha1",
			ServiceType: "database",
			Spec: map[string]any{
				"engine":            "",
				"version":           "",
				"resources":         database.DatabaseResources{},
				"connection_string": "",
			},
			OutputSpec: outputSpecDatabase(),
			Path:       "service-types/database",
		},
		{
			ID:          "cluster",
			ApiVersion:  "v1alpha1",
			ServiceType: "cluster",
			Spec: map[string]any{
				"version":      "",
				"api_endpoint": "",
				"console_url":  "",
				"kubeconfig":   "",
			},
			OutputSpec: outputSpecCluster(),
			Path:       "service-types/cluster",
		},
		{
			ID:          "storage",
			ApiVersion:  "v1alpha1",
			ServiceType: "storage",
			Spec: map[string]any{
				"capacity":    "",
				"volume_name": "",
			},
			OutputSpec: outputSpecStorage(),
			Path:       "service-types/storage",
		},
		{
			ID:          "network",
			ApiVersion:  "v1alpha1",
			ServiceType: "network",
			Spec: map[string]any{
				"ports":         []network.NetworkPort{},
				"routing_level": "",
				"endpoints":     []network.NetworkEndpoint{},
			},
			OutputSpec: outputSpecNetwork(),
			Path:       "service-types/network",
		},
	}
}

// outputSpecStorage returns the curated output definition for the storage service type.
// Derived from readOnly fields in api/catalog/v1alpha1/servicetypes/storage/spec.yaml.
func outputSpecStorage() map[string]any {
	return map[string]any{
		"volume_name": map[string]any{
			"type":        "string",
			"description": "Bound volume identifier assigned by the provider (e.g. PersistentVolume name)",
		},
	}
}

// outputSpecDatabase returns the curated output definition for the database service type.
// Derived from readOnly fields in api/catalog/v1alpha1/servicetypes/database/spec.yaml.
func outputSpecDatabase() map[string]any {
	return map[string]any{
		"connection_string": map[string]any{
			"type":        "string",
			"description": "Database connection URI (format: scheme://user:pass@host:port/db)",
		},
	}
}

// outputSpecCluster returns the curated output definition for the cluster service type.
// Derived from readOnly fields in api/catalog/v1alpha1/servicetypes/cluster/spec.yaml.
func outputSpecCluster() map[string]any {
	return map[string]any{
		"api_endpoint": map[string]any{
			"type":        "string",
			"description": "Kubernetes API server endpoint for the provisioned cluster",
		},
		"console_url": map[string]any{
			"type":        "string",
			"description": "Web console URL for the provisioned cluster",
		},
		"kubeconfig": map[string]any{
			"type":        "string",
			"description": "Base64-encoded kubeconfig granting admin access to the provisioned cluster",
		},
	}
}

// outputSpecContainer returns the curated output definition for the container service type.
// Derived from readOnly fields in api/catalog/v1alpha1/servicetypes/container/spec.yaml
// and metadata.namespace from api/catalog/v1alpha1/servicetypes/common.yaml (ServiceMetadata).
// ContainerEndpoint includes an optional protocol field (TCP/UDP).
func outputSpecContainer() map[string]any {
	return map[string]any{
		"endpoints": map[string]any{
			"type":        "array",
			"description": "Resolved connection endpoints exposed by the container",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"address":  map[string]any{"type": "string"},
					"port":     map[string]any{"type": "integer"},
					"scope":    map[string]any{"type": "string"},
					"protocol": map[string]any{"type": "string"},
				},
			},
		},
		// namespace is sourced from metadata.namespace (ServiceMetadata.namespace,
		// readOnly in common.yaml); surfaced as a top-level output field.
		"namespace": map[string]any{
			"type":        "string",
			"description": "Provider administrative namespace where the container was provisioned",
		},
	}
}

// outputSpecVM returns the curated output definition for the vm service type.
// Derived from readOnly fields in api/catalog/v1alpha1/servicetypes/vm/spec.yaml.
// VmEndpoint has address, port, scope (no protocol field unlike ContainerEndpoint).
func outputSpecVM() map[string]any {
	return map[string]any{
		"ip": map[string]any{
			"type":        "array",
			"description": "Resolved connection endpoints for the virtual machine",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"address": map[string]any{"type": "string"},
					"port":    map[string]any{"type": "integer"},
					"scope":   map[string]any{"type": "string"},
				},
			},
		},
	}
}

// outputSpecNetwork returns the curated output definition for the network service type.
// Derived from readOnly fields in api/catalog/v1alpha1/servicetypes/network/spec.yaml.
// NetworkEndpoint includes an optional name and protocol field.
func outputSpecNetwork() map[string]any {
	return map[string]any{
		"endpoints": map[string]any{
			"type":        "array",
			"description": "Resolved connection endpoints for the provisioned network service",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name":     map[string]any{"type": "string"},
					"address":  map[string]any{"type": "string"},
					"port":     map[string]any{"type": "integer"},
					"scope":    map[string]any{"type": "string"},
					"protocol": map[string]any{"type": "string"},
				},
			},
		},
	}
}

func (s *service) defaultCatalogItems() []model.CatalogItem {
	return []model.CatalogItem{
		s.petClinicCatalogItem(),
	}
}

func (s *service) petClinicCatalogItem() model.CatalogItem {
	return model.CatalogItem{
		ID:          "pet-clinic",
		ApiVersion:  "v1alpha1",
		DisplayName: "Pet Clinic",
		Path:        "catalog-items/pet-clinic",
		Spec: model.CatalogItemSpec{
			Resources: []model.CatalogResource{{
				Name:        "app",
				ServiceType: "three-tier-app-demo",
				Fields:      s.petClinicFields(),
			}},
		},
	}
}

func (s *service) petClinicFields() []model.FieldConfiguration {
	regionEnum := make([]any, len(s.seedConfig.RegionEnum))
	for i, v := range s.seedConfig.RegionEnum {
		regionEnum[i] = v
	}
	return []model.FieldConfiguration{
		fieldConfig("metadata.labels.region", "Region", true,
			optionalStringDefault(s.seedConfig.RegionDefault), map[string]any{"type": "string", "enum": regionEnum}, nil),
		fieldConfig("database.engine", "Database engine", true,
			three_tier_app_demo.DefaultDatabaseEngine,
			map[string]any{"type": "string", "enum": []any{"postgres", "mysql"}}, nil),
		fieldConfig("database.version", "Database version", true,
			three_tier_app_demo.DefaultDatabaseVersion,
			map[string]any{"type": "string"}, dependsOn("database.engine", map[string][]any{
				"postgres": {three_tier_app_demo.DefaultDatabaseVersion, "17"},
				"mysql":    {"8.4", "8.3", "8"},
			})),
		fieldConfig("app.image", "App image", false, three_tier_app_demo.AppImage, nil, nil),
		fieldConfig("web.image", "Web image", false, three_tier_app_demo.WebImage, nil, nil),
	}
}

func optionalStringDefault(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func fieldConfig(path, displayName string, editable bool, defaultVal any,
	validationSchema map[string]any, dependsOn *model.DependsOn,
) model.FieldConfiguration {
	return model.FieldConfiguration{
		Path:             path,
		DisplayName:      displayName,
		Editable:         editable,
		Default:          defaultVal,
		ValidationSchema: validationSchema,
		DependsOn:        dependsOn,
	}
}

func dependsOn(path string, allowedValues map[string][]any) *model.DependsOn {
	return &model.DependsOn{Path: path, AllowedValues: allowedValues}
}
