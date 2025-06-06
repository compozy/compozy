package schema

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ParamsValidator_Validate_NilParamsWithSchema(t *testing.T) {
	t.Run("Should return error when params are nil but schema is not", func(t *testing.T) {
		s := &Schema{
			"type": "object",
		}

		v := NewParamsValidator[any](nil, s, "testID")
		err := v.Validate(context.Background())
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "parameters are nil but a schema is defined")
			assert.Contains(t, err.Error(), "testID")
		}
	})
}
