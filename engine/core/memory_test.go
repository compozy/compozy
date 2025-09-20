package core

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestMemoryReference_Unmarshal(t *testing.T) {
	t.Run("Should unmarshal YAML scalar as ID-only", func(t *testing.T) {
		var m MemoryReference
		err := yaml.Unmarshal([]byte("conversation"), &m)
		require.NoError(t, err, "unmarshal scalar")
		assert.Equal(t, "conversation", m.ID)
		assert.Empty(t, m.Key)
		assert.Empty(t, m.Mode)
	})

	t.Run("Should unmarshal YAML object with fields", func(t *testing.T) {
		data := []byte("id: conv\nkey: user-1\nmode: read-write\n")
		var m MemoryReference
		err := yaml.Unmarshal(data, &m)
		require.NoError(t, err, "unmarshal object")
		assert.Equal(t, "conv", m.ID)
		assert.Equal(t, "user-1", m.Key)
		assert.Equal(t, "read-write", m.Mode)
	})

	t.Run("Should unmarshal JSON scalar as ID-only", func(t *testing.T) {
		var m MemoryReference
		err := json.Unmarshal([]byte(`"conversation"`), &m)
		require.NoError(t, err, "json scalar")
		assert.Equal(t, "conversation", m.ID)
		assert.Empty(t, m.Key)
		assert.Empty(t, m.Mode)
	})
}
