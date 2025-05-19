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
	triggerInput := map[string]any{
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
		normalizer := NewStateNormalizer(tplengine.FormatYAML)
		require.NotNil(t, normalizer)

		// Create a test state
		baseState := &BaseState{
			Input:   make(common.Input),
			Output:  make(common.Output),
			Env:     envMap,
			Trigger: triggerInput,
		}

		// Normalize the state
		normalized := normalizer.NormalizeState(baseState)

		// Verify normalized structure
		triggerMap := normalized["trigger"].(map[string]any)
		assert.NotNil(t, triggerMap["input"])
		assert.Same(t, baseState.GetInput(), normalized["input"])
		assert.Same(t, baseState.GetOutput(), normalized["output"])
		assert.Same(t, baseState.GetEnv(), normalized["env"])
	})

	t.Run("ParseTemplateValue", func(t *testing.T) {
		normalizer := NewStateNormalizer(tplengine.FormatYAML)
		require.NotNil(t, normalizer)

		// Create template context
		templateContext := map[string]any{
			"trigger": map[string]any{
				"input": triggerInput,
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
		normalizer := NewStateNormalizer(tplengine.FormatYAML)
		require.NotNil(t, normalizer)

		// Create a test state with templates
		baseState := &BaseState{
			Input: common.Input{
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
			Output: make(common.Output),
			Env: common.EnvMap{
				"VERSION":    "1.0.0",
				"USER_NAME":  "{{ .trigger.input.name }}",
				"USER_EMAIL": "{{ .trigger.input.email }}",
			},
			Trigger: triggerInput,
		}

		// Parse templates
		err := normalizer.ParseTemplates(baseState)
		require.NoError(t, err)

		// Verify input templates are parsed
		assert.Equal(t, "Hello John!", baseState.Input["greeting"])

		nested, ok := baseState.Input["nested"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "john@example.com", nested["email"])

		meta, ok := nested["meta"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "v1.0.0", meta["version"])

		items, ok := baseState.Input["items"].([]any)
		require.True(t, ok)
		assert.Equal(t, "Item: John", items[0])

		itemMap, ok := items[1].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "Age: 30", itemMap["age"])

		// Verify env templates are parsed
		assert.Equal(t, "1.0.0", baseState.Env["VERSION"])
		assert.Equal(t, "John", baseState.Env["USER_NAME"])
		assert.Equal(t, "john@example.com", baseState.Env["USER_EMAIL"])

		// Verify trigger templates are parsed
		assert.Equal(t, *baseState.GetTrigger(), baseState.Trigger)
		assert.Equal(t, "Welcome, John!", baseState.Trigger["message"])

		data, ok := baseState.Trigger["data"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "john@example.com", data["email"])
	})
}

func TestStateNormalizer_ErrorHandling(t *testing.T) {
	t.Run("UninitializedTemplateEngine", func(t *testing.T) {
		normalizer := &StateNormalizer{
			TemplateEngine: nil,
		}

		state := &BaseState{
			Input:   make(common.Input),
			Output:  make(common.Output),
			Env:     make(common.EnvMap),
			Trigger: make(common.Input),
		}

		err := normalizer.ParseTemplates(state)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "template engine is not initialized")
	})

	t.Run("InvalidTemplateInInput", func(t *testing.T) {
		normalizer := NewStateNormalizer(tplengine.FormatYAML)
		state := &BaseState{
			Input: common.Input{
				"greeting": "Hello {{ .name !",
			},
			Output:  make(common.Output),
			Env:     make(common.EnvMap),
			Trigger: make(common.Input),
		}

		err := normalizer.ParseTemplates(state)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse template in input")
	})

	t.Run("InvalidTemplateInEnv", func(t *testing.T) {
		normalizer := NewStateNormalizer(tplengine.FormatYAML)
		state := &BaseState{
			Input:  make(common.Input),
			Output: make(common.Output),
			Env: common.EnvMap{
				"USER_NAME": "{{ .name !",
			},
			Trigger: make(common.Input),
		}

		err := normalizer.ParseTemplates(state)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse template in env")
	})

	t.Run("InvalidTemplateInContext", func(t *testing.T) {
		normalizer := NewStateNormalizer(tplengine.FormatYAML)
		state := NewEmptyState()

		// Add an invalid template to the trigger input
		baseState := state.(*BaseState)
		baseState.Trigger["message"] = "{{ .invalid.syntax !"

		err := normalizer.ParseTemplates(state)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse template")
	})
}
