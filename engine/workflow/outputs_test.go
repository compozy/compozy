package workflow

import (
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
)

func TestWorkflowConfig_Outputs(t *testing.T) {
	t.Run("Should get outputs when defined", func(t *testing.T) {
		outputs := &core.Input{
			"result": "{{ .tasks.final.output }}",
		}
		config := &Config{
			ID:      "test-workflow",
			Outputs: outputs,
		}

		assert.Equal(t, outputs, config.GetOutputs())
	})

	t.Run("Should return nil when outputs not defined", func(t *testing.T) {
		config := &Config{
			ID: "test-workflow",
		}

		assert.Nil(t, config.GetOutputs())
	})
}

func TestWorkflowConfig_ValidateOutputs(t *testing.T) {
	t.Run("Should pass validation when outputs is nil", func(t *testing.T) {
		config := &Config{
			ID: "test-workflow",
		}

		err := config.validateOutputs()
		assert.NoError(t, err)
	})

	t.Run("Should pass validation with valid outputs", func(t *testing.T) {
		config := &Config{
			ID: "test-workflow",
			Outputs: &core.Input{
				"result": "{{ .tasks.process.output }}",
				"status": "success",
			},
		}

		err := config.validateOutputs()
		assert.NoError(t, err)
	})

	t.Run("Should fail validation with empty outputs", func(t *testing.T) {
		config := &Config{
			ID:      "test-workflow",
			Outputs: &core.Input{},
		}

		err := config.validateOutputs()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "outputs cannot be empty when defined")
	})
}
