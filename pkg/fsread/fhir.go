package fsread

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/innomon/agentic/pkg/registry"
)

//go:embed fhir.schema.json
var fhirSchemaData []byte

var (
	fullSchema      map[string]any
	fullDefinitions map[string]any
)

func init() {
	if err := json.Unmarshal(fhirSchemaData, &fullSchema); err != nil {
		panic(fmt.Sprintf("failed to parse embedded FHIR schema: %v", err))
	}
	var ok bool
	fullDefinitions, ok = fullSchema["definitions"].(map[string]any)
	if !ok {
		panic("embedded FHIR schema missing 'definitions' map")
	}
	registry.RegisterToolHandler("fhir_get_schema", fhirGetSchemaHandler)
}

// fhirGetSchemaHandler implements the 'fhir_get_schema' tool.
func fhirGetSchemaHandler(_ context.Context, args map[string]any) (any, error) {
	resourceType, ok := args["resource_type"].(string)
	if !ok || resourceType == "" {
		return nil, fmt.Errorf("missing required parameter 'resource_type'")
	}

	if _, ok := fullDefinitions[resourceType]; !ok {
		return nil, fmt.Errorf("resource type %q not found in schema definitions", resourceType)
	}

	subsetDefinitions := make(map[string]any)
	toVisit := []string{resourceType}
	visited := make(map[string]bool)

	for len(toVisit) > 0 {
		current := toVisit[0]
		toVisit = toVisit[1:]

		if visited[current] {
			continue
		}
		visited[current] = true

		def, ok := fullDefinitions[current]
		if !ok {
			continue
		}

		subsetDefinitions[current] = def
		refs := findRefs(def)
		for _, ref := range refs {
			if strings.HasPrefix(ref, "#/definitions/") {
				refType := strings.TrimPrefix(ref, "#/definitions/")
				if !visited[refType] {
					toVisit = append(toVisit, refType)
				}
			}
		}
	}

	subset := map[string]any{
		"$schema":     fullSchema["$schema"],
		"definitions": subsetDefinitions,
		"$ref":        "#/definitions/" + resourceType,
	}

	resJSON, err := json.MarshalIndent(subset, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema subset: %w", err)
	}

	return string(resJSON), nil
}

// findRefs recursively finds all "$ref" values in a JSON object/array.
func findRefs(v any) []string {
	var refs []string
	switch val := v.(type) {
	case map[string]any:
		for k, v := range val {
			if k == "$ref" {
				if s, ok := v.(string); ok {
					refs = append(refs, s)
				}
			} else {
				refs = append(refs, findRefs(v)...)
			}
		}
	case []any:
		for _, item := range val {
			refs = append(refs, findRefs(item)...)
		}
	}
	return refs
}
