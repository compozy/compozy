package state

import (
	"testing"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/pkg/tplengine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateNormalizer(t *testing.T) {
	// Test data
	tgInput := common.Input{
		"name":    "John",
		"email":   "john@example.com",
		"age":     30,
		"message": "Welcome, John!",
		"data": map[string]any{
			"email": "john@example.com",
		},
	}
	envMap := common.EnvMap{
		"VERSION": "1.0.0",
		"ENV":     "test",
	}

	t.Run("NormalizeState", func(t *testing.T) {
		normalizer := NewNormalizer(tplengine.FormatYAML)
		require.NotNil(t, normalizer)

		// Create a test state
		bsState := NewEmptyState(
			WithEnv(&envMap),
			WithTrigger(&tgInput),
		)

		// Normalize the state
		normalized := normalizer.NormalizeState(bsState)

		// Verify normalized structure
		triggerMap := normalized["trigger"].(map[string]any)
		assert.NotNil(t, triggerMap["input"])
		assert.Same(t, bsState.GetInput(), normalized["input"])
		assert.Same(t, bsState.GetOutput(), normalized["output"])
		assert.Same(t, bsState.GetEnv(), normalized["env"])
	})

	t.Run("ParseTemplateValue", func(t *testing.T) {
		normalizer := NewNormalizer(tplengine.FormatYAML)
		require.NotNil(t, normalizer)

		// Create template context
		templateContext := map[string]any{
			"trigger": map[string]any{
				"input": tgInput,
			},
			"env": &envMap,
		}

		t.Run("String", func(t *testing.T) {
			// Test string template
			templateStr := "Hello {{ .trigger.input.name }}!"
			result, err := normalizer.ParseTemplateValue(templateStr, templateContext)
			require.NoError(t, err)
			assert.Equal(t, "Hello John!", result)
		})

		t.Run("Map", func(t *testing.T) {
			// Test map with templates
			templateMap := map[string]any{
				"greeting": "Hello {{ .trigger.input.name }}!",
				"info": map[string]any{
					"email":   "Email: {{ .trigger.input.email }}",
					"version": "v{{ .env.VERSION }}",
				},
			}
			result, err := normalizer.ParseTemplateValue(templateMap, templateContext)
			require.NoError(t, err)

			resultMap, ok := result.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "Hello John!", resultMap["greeting"])

			infoMap, ok := resultMap["info"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "Email: john@example.com", infoMap["email"])
			assert.Equal(t, "v1.0.0", infoMap["version"])
		})

		t.Run("Array", func(t *testing.T) {
			// Test array with templates
			templateArray := []any{
				"Item 1: {{ .trigger.input.name }}",
				map[string]any{
					"label": "Item 2",
					"email": "{{ .trigger.input.email }}",
				},
				[]any{
					"Subitem: {{ .env.VERSION }}",
					"Name: {{ .trigger.input.name }}",
				},
			}
			result, err := normalizer.ParseTemplateValue(templateArray, templateContext)
			require.NoError(t, err)

			resultArray, ok := result.([]any)
			require.True(t, ok)
			assert.Equal(t, "Item 1: John", resultArray[0])

			item2, ok := resultArray[1].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "Item 2", item2["label"])
			assert.Equal(t, "john@example.com", item2["email"])

			subitems, ok := resultArray[2].([]any)
			require.True(t, ok)
			assert.Equal(t, "Subitem: 1.0.0", subitems[0])
			assert.Equal(t, "Name: John", subitems[1])
		})

		t.Run("NonTemplateValues", func(t *testing.T) {
			// Test non-string values
			intVal, err := normalizer.ParseTemplateValue(42, templateContext)
			require.NoError(t, err)
			assert.Equal(t, 42, intVal)

			boolVal, err := normalizer.ParseTemplateValue(true, templateContext)
			require.NoError(t, err)
			assert.Equal(t, true, boolVal)

			nilVal, err := normalizer.ParseTemplateValue(nil, templateContext)
			require.NoError(t, err)
			assert.Nil(t, nilVal)
		})
	})

	t.Run("ParseTemplates", func(t *testing.T) {
		normalizer := NewNormalizer(tplengine.FormatYAML)
		require.NotNil(t, normalizer)

		// Create a test state with templates
		bsState := &BaseState{
			Input: &common.Input{
				"greeting": "Hello {{ .trigger.input.name }}!",
				"nested": map[string]any{
					"email": "{{ .trigger.input.email }}",
					"meta": map[string]any{
						"version": "v{{ .env.VERSION }}",
					},
				},
				"items": []any{
					"Item: {{ .trigger.input.name }}",
					map[string]any{"age": "Age: {{ .trigger.input.age }}"},
				},
			},
			Output: &common.Output{},
			Env: &common.EnvMap{
				"VERSION":    "1.0.0",
				"USER_NAME":  "{{ .trigger.input.name }}",
				"USER_EMAIL": "{{ .trigger.input.email }}",
			},
			Trigger: &tgInput,
		}

		// Parse templates
		err := normalizer.ParseTemplates(bsState)
		require.NoError(t, err)

		// Verify input templates are parsed
		assert.Equal(t, "Hello John!", (*bsState.Input)["greeting"])

		nested, ok := (*bsState.Input)["nested"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "john@example.com", nested["email"])

		meta, ok := nested["meta"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "v1.0.0", meta["version"])

		items, ok := (*bsState.Input)["items"].([]any)
		require.True(t, ok)
		assert.Equal(t, "Item: John", items[0])

		itemMap, ok := items[1].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "Age: 30", itemMap["age"])

		// Verify env templates are parsed
		assert.Equal(t, "1.0.0", (*bsState.Env)["VERSION"])
		assert.Equal(t, "John", (*bsState.Env)["USER_NAME"])
		assert.Equal(t, "john@example.com", (*bsState.Env)["USER_EMAIL"])

		// Verify trigger templates are parsed
		assert.Equal(t, bsState.GetTrigger(), bsState.Trigger)
		assert.Equal(t, "Welcome, John!", (*bsState.Trigger)["message"])

		data, ok := (*bsState.Trigger)["data"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "john@example.com", data["email"])
	})
}

func TestStateNormalizer_ErrorHandling(t *testing.T) {
	t.Run("UninitializedTemplateEngine", func(t *testing.T) {
		normalizer := &Normalizer{
			TemplateEngine: nil,
		}

		state := NewEmptyState()
		err := normalizer.ParseTemplates(state)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "template engine is not initialized")
	})

	t.Run("InvalidTemplateInInput", func(t *testing.T) {
		normalizer := NewNormalizer(tplengine.FormatYAML)
		state := NewEmptyState(
			WithInput(&common.Input{
				"greeting": "Hello {{ .name !",
			}),
		)

		err := normalizer.ParseTemplates(state)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse template in input")
	})

	t.Run("InvalidTemplateInEnv", func(t *testing.T) {
		normalizer := NewNormalizer(tplengine.FormatYAML)
		state := NewEmptyState(
			WithEnv(&common.EnvMap{
				"USER_NAME": "{{ .name !",
			}),
		)

		err := normalizer.ParseTemplates(state)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse template in env")
	})

	t.Run("InvalidTemplateInContext", func(t *testing.T) {
		normalizer := NewNormalizer(tplengine.FormatYAML)
		state := NewEmptyState()

		// Add an invalid template to the trigger input
		bsState := state.(*BaseState)
		(*bsState.Trigger)["message"] = "{{ .invalid.syntax !"

		err := normalizer.ParseTemplates(state)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse template")
	})
}
