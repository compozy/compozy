package orchestrate

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/compozy/compozy/engine/schema"
	"github.com/invopop/jsonschema"
)

var (
	planSchemaOnce sync.Once
	planSchemaInst *schema.Schema
	planSchemaErr  error
)

func PlanSchema() (*schema.Schema, error) {
	planSchemaOnce.Do(func() {
		planSchemaInst, planSchemaErr = generatePlanSchema()
	})
	if planSchemaErr != nil {
		return nil, planSchemaErr
	}
	return planSchemaInst.Clone()
}

func generatePlanSchema() (*schema.Schema, error) {
	reflector := &jsonschema.Reflector{
		RequiredFromJSONSchemaTags: true,
		AllowAdditionalProperties:  false,
		ExpandedStruct:             true,
		DoNotReference:             true,
		FieldNameTag:               "json",
	}
	js := reflector.Reflect(&Plan{})
	js.Version = jsonschema.Version
	js.ID = "https://compozy.dev/schema/tool/orchestrate/plan.json"
	js.AdditionalProperties = jsonschema.TrueSchema

	raw, err := json.Marshal(js)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal plan schema: %w", err)
	}
	var result schema.Schema
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("failed to decode plan schema: %w", err)
	}
	return &result, nil
}
