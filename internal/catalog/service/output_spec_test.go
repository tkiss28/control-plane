package service_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/getkin/kin-openapi/openapi3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/dcm-project/control-plane/internal/catalog/config"
	"github.com/dcm-project/control-plane/internal/catalog/service"
	"github.com/dcm-project/control-plane/internal/catalog/store"
	"github.com/dcm-project/control-plane/internal/catalog/store/model"
)

var _ = Describe("ServiceType OutputSpec (U1a)", func() {
	var (
		ctx context.Context
		db  *gorm.DB
		str store.Store
		svc service.Service
	)

	BeforeEach(func() {
		ctx = context.Background()
		var err error
		db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
			Logger: logger.Discard,
		})
		Expect(err).ToNot(HaveOccurred())

		err = db.Exec("PRAGMA foreign_keys = ON").Error
		Expect(err).ToNot(HaveOccurred())

		err = db.AutoMigrate(&model.ServiceType{}, &model.CatalogItem{})
		Expect(err).ToNot(HaveOccurred())

		str = store.NewStore(db, slog.Default())
		svc, err = service.NewService(str, &mockPMClient{}, config.DefaultSeedConfig(), slog.Default())
		Expect(err).ToNot(HaveOccurred())

		err = svc.Seed(ctx)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		if str != nil {
			Expect(str.Close()).To(Succeed())
		}
	})

	// TC-OUT-0101: database has connection_string (string) and no input fields.
	It("[TC-OUT-0101] database output_spec has connection_string and no input fields", func() {
		st, err := str.ServiceType().Get(ctx, "database")
		Expect(err).ToNot(HaveOccurred())
		Expect(st.OutputSpec).ToNot(BeNil())
		Expect(st.OutputSpec).To(HaveKey("connection_string"))

		def, ok := st.OutputSpec["connection_string"].(map[string]any)
		Expect(ok).To(BeTrue(), "connection_string definition should be a map")
		Expect(def["type"]).To(Equal("string"))

		// Input fields must NOT appear in the output definition.
		Expect(st.OutputSpec).ToNot(HaveKey("engine"))
		Expect(st.OutputSpec).ToNot(HaveKey("version"))
		Expect(st.OutputSpec).ToNot(HaveKey("resources"))
	})

	// TC-OUT-0102: container.output_spec.endpoints is an array with typed item properties.
	It("[TC-OUT-0102] container output_spec.endpoints is array with {address,port,scope} item props", func() {
		st, err := str.ServiceType().Get(ctx, "container")
		Expect(err).ToNot(HaveOccurred())
		Expect(st.OutputSpec).ToNot(BeNil())
		Expect(st.OutputSpec).To(HaveKey("endpoints"))

		ep, ok := st.OutputSpec["endpoints"].(map[string]any)
		Expect(ok).To(BeTrue(), "endpoints definition should be a map")
		Expect(ep["type"]).To(Equal("array"))

		items, ok := ep["items"].(map[string]any)
		Expect(ok).To(BeTrue(), "endpoints.items should be a map")
		Expect(items["type"]).To(Equal("object"))

		props, ok := items["properties"].(map[string]any)
		Expect(ok).To(BeTrue(), "endpoints.items.properties should be a map")
		for _, key := range []string{"address", "port", "scope"} {
			Expect(props).To(HaveKey(key), "endpoints item should have property %s", key)
		}

		addrDef, ok := props["address"].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(addrDef["type"]).To(Equal("string"))

		portDef, ok := props["port"].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(portDef["type"]).To(Equal("integer"))

		scopeDef, ok := props["scope"].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(scopeDef["type"]).To(Equal("string"))
	})

	// TC-OUT-0103: vm.output_spec.ip is a typed array (not a scalar string).
	It("[TC-OUT-0103] vm output_spec.ip is a typed array of endpoint objects (not scalar)", func() {
		st, err := str.ServiceType().Get(ctx, "vm")
		Expect(err).ToNot(HaveOccurred())
		Expect(st.OutputSpec).ToNot(BeNil())
		Expect(st.OutputSpec).To(HaveKey("ip"))

		ipDef, ok := st.OutputSpec["ip"].(map[string]any)
		Expect(ok).To(BeTrue(), "ip definition should be a map")
		Expect(ipDef["type"]).To(Equal("array"), "ip should be type array, not a scalar")

		items, ok := ipDef["items"].(map[string]any)
		Expect(ok).To(BeTrue(), "ip.items should be a map")
		Expect(items["type"]).To(Equal("object"))

		props, ok := items["properties"].(map[string]any)
		Expect(ok).To(BeTrue(), "ip.items.properties should be a map")
		for _, key := range []string{"address", "port", "scope"} {
			Expect(props).To(HaveKey(key))
		}
	})

	// TC-OUT-0104: cluster has api_endpoint, console_url, kubeconfig — all string type.
	It("[TC-OUT-0104] cluster output_spec has api_endpoint/console_url/kubeconfig as strings", func() {
		st, err := str.ServiceType().Get(ctx, "cluster")
		Expect(err).ToNot(HaveOccurred())
		Expect(st.OutputSpec).ToNot(BeNil())

		for _, field := range []string{"api_endpoint", "console_url", "kubeconfig"} {
			Expect(st.OutputSpec).To(HaveKey(field), "cluster output_spec should have field %s", field)
			def, ok := st.OutputSpec[field].(map[string]any)
			Expect(ok).To(BeTrue(), "cluster.%s definition should be a map", field)
			Expect(def["type"]).To(Equal("string"), "cluster.%s should have type string", field)
		}
	})

	// TC-OUT-0105: container output_spec surfaces namespace from common.yaml ServiceMetadata (readOnly).
	It("[TC-OUT-0105] container output_spec has namespace from common readOnly ServiceMetadata field", func() {
		st, err := str.ServiceType().Get(ctx, "container")
		Expect(err).ToNot(HaveOccurred())
		Expect(st.OutputSpec).To(HaveKey("namespace"))

		nsDef, ok := st.OutputSpec["namespace"].(map[string]any)
		Expect(ok).To(BeTrue(), "namespace definition should be a map")
		Expect(nsDef["type"]).To(Equal("string"))
	})

	// TC-OUT-0106: AutoMigrate adds output_spec column and is idempotent on re-run.
	It("[TC-OUT-0106] output_spec column exists in service_types table and AutoMigrate is idempotent", func() {
		// Verify the column was created by the AutoMigrate in BeforeEach.
		var cols []struct {
			Name string `gorm:"column:name"`
		}
		err := db.Raw(
			"SELECT name FROM pragma_table_info('service_types') WHERE name = 'output_spec'",
		).Scan(&cols).Error
		Expect(err).ToNot(HaveOccurred())
		Expect(cols).To(HaveLen(1), "output_spec column should exist in service_types table")

		// Second AutoMigrate must succeed without error (idempotent).
		err = db.AutoMigrate(&model.ServiceType{})
		Expect(err).ToNot(HaveOccurred())
	})

	// TC-OUT-0108: three-tier-app-demo has nil/empty output_spec (no cross-provider outputs).
	It("[TC-OUT-0108] three-tier-app-demo output_spec is nil (no cross-provider output fields)", func() {
		st, err := str.ServiceType().Get(ctx, "three-tier-app-demo")
		Expect(err).ToNot(HaveOccurred())
		// three-tier-app-demo is a composite demo type with no declared output fields.
		Expect(st.OutputSpec).To(Or(BeNil(), BeEmpty()))
	})

	// TC-OUT-0111: drift-guard (test-time only).
	// Loads each service type's spec.yaml via getkin/kin-openapi (resolves external $refs,
	// preserves ReadOnly), collects the readOnly field names+types from the type-specific
	// allOf entry, and asserts they match the curated OutputSpec in seed.go.
	// Runs only in the test binary — no runtime YAML loading added to the control plane.
	Describe("[TC-OUT-0111] drift-guard: curated output_spec matches spec.yaml readOnly fields", func() {
		var specsDir string

		BeforeEach(func() {
			wd, err := os.Getwd()
			Expect(err).ToNot(HaveOccurred())
			specsDir = filepath.Join(wd, "../../../api/catalog/v1alpha1/servicetypes")
		})

		DescribeTable("curated output_spec matches type-specific readOnly fields in spec.yaml",
			func(serviceTypeID, schemaName string, extraCommonFields []string) {
				// Load curated OutputSpec from DB (seeded).
				st, err := str.ServiceType().Get(ctx, serviceTypeID)
				Expect(err).ToNot(HaveOccurred())

				// Load type-specific readOnly fields from the spec.yaml.
				specPath := filepath.Join(specsDir, serviceTypeID, "spec.yaml")
				specFields, err := loadTypeSpecificReadOnlyFields(specPath, schemaName)
				Expect(err).ToNot(HaveOccurred())
				Expect(specFields).ToNot(BeEmpty(), "spec should have at least one type-specific readOnly field")

				// Partition the curated OutputSpec into spec-derived and common-field sets.
				curatedSpecFields := make(map[string]any)
				curatedCommonFields := make(map[string]any)
				for k, v := range st.OutputSpec {
					isCommon := false
					for _, cf := range extraCommonFields {
						if k == cf {
							isCommon = true
							break
						}
					}
					if isCommon {
						curatedCommonFields[k] = v
					} else {
						curatedSpecFields[k] = v
					}
				}

				// Direction 1: every type-specific spec readOnly field must be in the curated set
				// with a matching top-level type.
				for fieldName, sf := range specFields {
					Expect(curatedSpecFields).To(HaveKey(fieldName),
						"spec readOnly field %q is missing from curated output_spec", fieldName)
					def, ok := curatedSpecFields[fieldName].(map[string]any)
					Expect(ok).To(BeTrue(), "curated %q should be a map[string]any", fieldName)
					Expect(def["type"]).To(Equal(sf.typeName),
						"curated %q.type should match spec readOnly type", fieldName)

					if sf.typeName == "array" {
						assertItemPropsMatch(def, sf.itemProps, fieldName)
					}
				}

				// Direction 2: every curated spec-derived field must have a readOnly counterpart in the spec.
				for fieldName := range curatedSpecFields {
					Expect(specFields).To(HaveKey(fieldName),
						"curated field %q has no matching readOnly field in spec.yaml", fieldName)
				}

				// Common fields (e.g. namespace from ServiceMetadata) verified separately.
				for _, cf := range extraCommonFields {
					Expect(curatedCommonFields).To(HaveKey(cf),
						"expected common field %q in curated output_spec", cf)
				}
			},
			Entry("storage", "storage", "StorageSpec", nil),
			Entry("database", "database", "DatabaseSpec", nil),
			Entry("cluster", "cluster", "ClusterSpec", nil),
			Entry("container", "container", "ContainerSpec", []string{"namespace"}),
			Entry("vm", "vm", "VMSpec", nil),
			Entry("network", "network", "NetworkSpec", nil),
		)

		It("verifies metadata.namespace readOnly in common.yaml for container namespace field", func() {
			commonPath := filepath.Join(specsDir, "common.yaml")
			loader := openapi3.NewLoader()
			loader.IsExternalRefsAllowed = true
			doc, err := loader.LoadFromFile(commonPath)
			Expect(err).ToNot(HaveOccurred())

			serviceMetadata, ok := doc.Components.Schemas["ServiceMetadata"]
			Expect(ok).To(BeTrue(), "common.yaml should define ServiceMetadata schema")
			nsProp, ok := serviceMetadata.Value.Properties["namespace"]
			Expect(ok).To(BeTrue(), "ServiceMetadata should have a namespace property")
			Expect(nsProp.Value.ReadOnly).To(BeTrue(),
				"ServiceMetadata.namespace must be readOnly to be a valid output field")
			Expect(openAPITypeName(nsProp.Value)).To(Equal("string"))
		})
	})
})

// specFieldDef describes one readOnly field collected from a spec.yaml.
type specFieldDef struct {
	typeName  string            // "string", "integer", "array", "object"
	itemProps map[string]string // for arrays: item property name → type
}

// loadTypeSpecificReadOnlyFields loads specPath and returns readOnly fields from the
// inline (non-$ref) allOf entry of schemaName. CommonFields contributed via $ref are
// intentionally excluded; only type-specific output fields are returned.
func loadTypeSpecificReadOnlyFields(specPath, schemaName string) (map[string]specFieldDef, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	doc, err := loader.LoadFromFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("load spec %s: %w", specPath, err)
	}

	schemaRef, ok := doc.Components.Schemas[schemaName]
	if !ok {
		return nil, fmt.Errorf("schema %q not found in %s", schemaName, specPath)
	}

	result := make(map[string]specFieldDef)
	for _, allOfEntry := range schemaRef.Value.AllOf {
		if allOfEntry.Ref != "" {
			// Skip $ref entries (these are CommonFields — not type-specific outputs).
			continue
		}
		for name, propRef := range allOfEntry.Value.Properties {
			if !propRef.Value.ReadOnly {
				continue
			}
			sf := specFieldDef{typeName: openAPITypeName(propRef.Value)}
			if sf.typeName == "array" && propRef.Value.Items != nil {
				sf.itemProps = collectOpenAPIItemProps(propRef.Value.Items.Value)
			}
			result[name] = sf
		}
	}
	return result, nil
}

// collectOpenAPIItemProps returns property name → type for the properties of an
// array's items schema (after $ref resolution by kin-openapi).
func collectOpenAPIItemProps(s *openapi3.Schema) map[string]string {
	result := make(map[string]string)
	if s == nil {
		return result
	}
	for name, propRef := range s.Properties {
		result[name] = openAPITypeName(propRef.Value)
	}
	return result
}

// openAPITypeName returns the first type string from an OpenAPI 3.0 schema.
// In kin-openapi v0.139 Schema.Type is *openapi3.Types ([]string).
func openAPITypeName(s *openapi3.Schema) string {
	if s == nil || s.Type == nil {
		return ""
	}
	sl := s.Type.Slice()
	if len(sl) == 0 {
		return ""
	}
	return sl[0]
}

// assertItemPropsMatch verifies that the curated array definition's item properties
// match the types collected from the spec schema.
func assertItemPropsMatch(curatedDef map[string]any, specItemProps map[string]string, fieldName string) {
	items, ok := curatedDef["items"].(map[string]any)
	Expect(ok).To(BeTrue(), "curated %q should have an items map", fieldName)
	props, ok := items["properties"].(map[string]any)
	Expect(ok).To(BeTrue(), "curated %q.items should have a properties map", fieldName)

	for propName, specType := range specItemProps {
		Expect(props).To(HaveKey(propName),
			"curated %q.items.properties is missing spec property %q", fieldName, propName)
		propDef, ok := props[propName].(map[string]any)
		Expect(ok).To(BeTrue(), "curated %q.items.properties[%q] should be a map", fieldName, propName)
		Expect(propDef["type"]).To(Equal(specType),
			"curated %q.items.properties[%q].type should match spec", fieldName, propName)
	}

	// Also check the reverse: curated properties must exist in the spec.
	for propName := range props {
		Expect(specItemProps).To(HaveKey(propName),
			"curated %q has item property %q not found in spec", fieldName, propName)
	}
}
