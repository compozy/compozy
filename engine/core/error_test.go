package core

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Error_Type(t *testing.T) {
	t.Run("Should build from error with code and details", func(t *testing.T) {
		e := NewError(errors.New("boom"), "E1", map[string]any{"k": "v"})
		assert.Equal(t, "boom", e.Error())
		m := e.AsMap()
		assert.Equal(t, "boom", m["message"])
		assert.Equal(t, "E1", m["code"])
		assert.Equal(t, map[string]any{"k": "v"}, m["details"])
	})
	t.Run("Should build from nil error and handle empty/nil cases", func(t *testing.T) {
		e := NewError(nil, "", nil)
		assert.Equal(t, "unknown error", e.Error())
		var enil *Error
		assert.Equal(t, "", enil.Error())
		assert.Nil(t, enil.AsMap())
		assert.Nil(t, (&Error{}).AsMap())
	})
}
