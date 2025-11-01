package codegen

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratedFilesHashes(t *testing.T) {
	// This test locks generated outputs to make intentional template changes explicit.
	// When updates are expected, run `go test -run TestGeneratedFilesHashes -v`
	// and refresh the hashes from the failure output.
	files := map[string]string{
		"options_generated.go":   "c827fddefb3ca3a92e9148f83b8ddab434033e3a0b015877ae5686c5312a5a60",
		"engine_execution.go":    "4a398c36ef0d122a0fa10e4b2deaa4948d524ccaf809264b3ded3ca8ebaa32da",
		"engine_loading.go":      "4c67e1323d10f6ebf6d270e4838ae57058fd70488d5930f968e6e79be7bf4058",
		"engine_registration.go": "d5af56637e802639fb4d14b5462e799b131072a0437441d1dd83c1e4a3078161",
	}
	root := filepath.Clean("..")
	for name, expected := range files {
		name := name
		expected := expected
		t.Run("Should match generated hash for "+name, func(t *testing.T) {
			path := filepath.Join(root, name)
			data, err := os.ReadFile(path)
			require.NoError(t, err, "failed to read %s", name)
			sum := sha256.Sum256(data)
			hash := hex.EncodeToString(sum[:])
			assert.Equal(t, expected, hash, "hash mismatch for %s", name)
		})
	}
}
