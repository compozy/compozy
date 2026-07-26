package extensionpkg

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	toolspkg "github.com/compozy/agh/internal/tools"
)

type schemaDigestFixture struct {
	Name      string          `json:"name"`
	Schema    json.RawMessage `json:"schema"`
	Canonical string          `json:"canonical"`
	SHA256    string          `json:"sha256"`
}

func TestSchemaDigestFixturesMatchJCSCanonicalBytes(t *testing.T) {
	t.Parallel()

	fixtures := readSchemaDigestFixtures(t, filepath.Join("testdata", "digest", "cases.json"))
	for _, fixture := range fixtures {
		t.Run("Should Match Fixture "+fixture.Name, func(t *testing.T) {
			t.Parallel()

			canonical, err := toolspkg.CanonicalJSON(fixture.Schema)
			if err != nil {
				t.Fatalf("CanonicalJSON(%s) error = %v", fixture.Name, err)
			}
			if got, want := string(canonical), fixture.Canonical; got != want {
				t.Fatalf("CanonicalJSON(%s) = %q, want %q", fixture.Name, got, want)
			}

			digest, err := toolspkg.SchemaDigest(fixture.Schema)
			if err != nil {
				t.Fatalf("SchemaDigest(%s) error = %v", fixture.Name, err)
			}
			if digest != fixture.SHA256 {
				t.Fatalf("SchemaDigest(%s) = %q, want %q", fixture.Name, digest, fixture.SHA256)
			}
		})
	}
}

func TestSchemaDigestFixturesAreSharedWithSDKs(t *testing.T) {
	t.Parallel()

	daemonFixture := readFile(t, filepath.Join("testdata", "digest", "cases.json"))
	tests := []struct {
		name string
		path string
	}{
		{
			name: "Should Match TypeScript SDK Fixture",
			path: filepath.Join("..", "..", "sdk", "typescript", "test-fixtures", "digest", "cases.json"),
		},
		{
			name: "Should Match Go SDK Fixture",
			path: filepath.Join("..", "..", "sdk", "go", "test-fixtures", "digest", "cases.json"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if sdkFixture := readFile(t, tt.path); !bytes.Equal(sdkFixture, daemonFixture) {
				t.Fatalf("digest fixture %s differs from daemon fixture", tt.path)
			}
		})
	}
}

func readSchemaDigestFixtures(t *testing.T, path string) []schemaDigestFixture {
	t.Helper()

	var fixtures []schemaDigestFixture
	if err := json.Unmarshal(readFile(t, path), &fixtures); err != nil {
		t.Fatalf("json.Unmarshal(%s) error = %v", path, err)
	}
	if len(fixtures) == 0 {
		t.Fatalf("%s has no digest fixtures", path)
	}
	return fixtures
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%s) error = %v", path, err)
	}
	return data
}
