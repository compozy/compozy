package codegen

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestGeneratedFilesHashes(t *testing.T) {
	// This test locks generated outputs to make intentional template changes explicit.
	// When updates are expected, run `go test -run TestGeneratedFilesHashes -v`
	// and refresh the hashes from the failure output.
	files := map[string]string{
		"options_generated.go":   "c827fddefb3ca3a92e9148f83b8ddab434033e3a0b015877ae5686c5312a5a60",
		"engine_execution.go":    "4a398c36ef0d122a0fa10e4b2deaa4948d524ccaf809264b3ded3ca8ebaa32da",
		"engine_loading.go":      "25d3c9fa658465605325ed48c3123064b427b496900adde646e11fa86a97ceff",
		"engine_registration.go": "d89b5092948ea52c758ab5753706661e2731e4f83480947c308ec6acb7a3c810",
	}
	root := filepath.Clean("..")
	for name, expected := range files {
		path := filepath.Join(root, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		sum := sha256.Sum256(data)
		hash := hex.EncodeToString(sum[:])
		if hash != expected {
			t.Fatalf("unexpected hash for %s: got %s want %s", name, hash, expected)
		}
	}
}
