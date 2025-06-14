package project

import (
	"testing"

	"github.com/compozy/compozy/engine/autoload"
	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
)

func TestConfig_AutoloadValidationCaching(t *testing.T) {
	cwd, _ := core.CWDFromPath(".")
	t.Run("Should cache autoload validation results", func(t *testing.T) {
		config := &Config{
			Name: "test-project",
			CWD:  cwd,
			AutoLoad: &autoload.Config{
				Enabled: true,
				Include: []string{"*.yaml"},
			},
		}
		err1 := config.Validate()
		assert.NoError(t, err1)
		assert.True(t, config.autoloadValidated)
		assert.NoError(t, config.autoloadValidError)
		err2 := config.Validate()
		assert.NoError(t, err2)
	})

	t.Run("Should cache autoload validation errors", func(t *testing.T) {
		config := &Config{
			Name: "test-project",
			CWD:  cwd,
			AutoLoad: &autoload.Config{
				Enabled: true,
				Include: []string{},
			},
		}
		err1 := config.Validate()
		assert.Error(t, err1)
		assert.True(t, config.autoloadValidated)
		assert.Error(t, config.autoloadValidError)
		err2 := config.Validate()
		assert.Error(t, err2)
		assert.Equal(t, err1.Error(), err2.Error())
	})

	t.Run("Should skip validation when autoload is nil", func(t *testing.T) {
		config := &Config{
			Name:     "test-project",
			CWD:      cwd,
			AutoLoad: nil,
		}
		err := config.Validate()
		assert.NoError(t, err)
		assert.False(t, config.autoloadValidated)
	})
}
