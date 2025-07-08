package config

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSensitiveString_String(t *testing.T) {
	t.Run("Should redact non-empty values", func(t *testing.T) {
		s := SensitiveString("secret-password-123")
		assert.Equal(t, "[REDACTED]", s.String())
		assert.Equal(t, "[REDACTED]", s.String())
	})

	t.Run("Should return empty string for empty values", func(t *testing.T) {
		s := SensitiveString("")
		assert.Equal(t, "", s.String())
	})
}

func TestSensitiveString_Value(t *testing.T) {
	t.Run("Should return actual value", func(t *testing.T) {
		secret := "my-secret-api-key"
		s := SensitiveString(secret)
		assert.Equal(t, secret, s.Value())
	})
}

func TestSensitiveString_MarshalJSON(t *testing.T) {
	t.Run("Should marshal as redacted string", func(t *testing.T) {
		type TestStruct struct {
			APIKey SensitiveString `json:"api_key"`
			Name   string          `json:"name"`
		}

		test := TestStruct{
			APIKey: SensitiveString("secret-key-123"),
			Name:   "test-service",
		}

		data, err := json.Marshal(test)
		require.NoError(t, err)

		var result map[string]string
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)

		assert.Equal(t, "[REDACTED]", result["api_key"])
		assert.Equal(t, "test-service", result["name"])
	})

	t.Run("Should marshal empty value as empty string", func(t *testing.T) {
		s := SensitiveString("")
		data, err := json.Marshal(s)
		require.NoError(t, err)
		assert.Equal(t, `""`, string(data))
	})
}

func TestSensitiveString_UnmarshalJSON(t *testing.T) {
	t.Run("Should unmarshal string values", func(t *testing.T) {
		var s SensitiveString
		err := json.Unmarshal([]byte(`"secret-value"`), &s)
		require.NoError(t, err)
		assert.Equal(t, "secret-value", s.Value())
	})

	t.Run("Should handle empty strings", func(t *testing.T) {
		var s SensitiveString
		err := json.Unmarshal([]byte(`""`), &s)
		require.NoError(t, err)
		assert.Equal(t, "", s.Value())
	})
}
