package storeschema

import (
	"bytes"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModernizeSQLCGenerated(t *testing.T) {
	t.Parallel()

	t.Run("Should replace legacy empty interfaces and preserve gofmt output", func(t *testing.T) {
		t.Parallel()

		outputPath := t.TempDir()
		source := []byte(
			"package generated\n\n" +
				"type Row struct {\n" +
				" Value interface{} `json:\"value\"`\n" +
				" Label string `json:\"label\"`\n" +
				"}\n",
		)
		filePath := filepath.Join(outputPath, "query.sql.go")
		if err := os.WriteFile(filePath, source, 0o600); err != nil {
			t.Fatalf("os.WriteFile() error = %v", err)
		}

		if err := modernizeSQLCGenerated(outputPath); err != nil {
			t.Fatalf("modernizeSQLCGenerated() error = %v", err)
		}
		got, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("os.ReadFile() error = %v", err)
		}
		if strings.Contains(string(got), "interface{}") {
			t.Fatalf("generated source = %q, want any instead of interface{}", got)
		}
		want, err := format.Source(got)
		if err != nil {
			t.Fatalf("format.Source() error = %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("generated source is not gofmt-stable:\n%s", got)
		}
	})
}

func TestValidateSQLCPackageRegistry(t *testing.T) {
	t.Parallel()

	const (
		firstPackage        = "internal/example/first"
		secondPackage       = "internal/example/second"
		unregisteredPackage = "internal/example/unregistered"
	)

	t.Run("Should accept the exact discovered sqlc package set", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeSQLCConfig(t, root, firstPackage)
		writeSQLCConfig(t, root, secondPackage)

		err := validateSQLCPackageRegistry(root, []string{
			secondPackage,
			firstPackage,
		})
		if err != nil {
			t.Fatalf("validateSQLCPackageRegistry() error = %v", err)
		}
	})

	t.Run("Should reject an unregistered production sqlc package", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeSQLCConfig(t, root, firstPackage)
		writeSQLCConfig(t, root, unregisteredPackage)

		err := validateSQLCPackageRegistry(root, []string{firstPackage})
		if err == nil {
			t.Fatal("validateSQLCPackageRegistry() error = nil, want registry drift")
		}
		if !strings.Contains(err.Error(), unregisteredPackage) {
			t.Fatalf("validateSQLCPackageRegistry() error = %v, want discovered package", err)
		}
	})

	t.Run("Should reject a registered package without sqlc configuration", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeSQLCConfig(t, root, firstPackage)

		err := validateSQLCPackageRegistry(root, []string{
			firstPackage,
			secondPackage,
		})
		if err == nil {
			t.Fatal("validateSQLCPackageRegistry() error = nil, want registry drift")
		}
		if !strings.Contains(err.Error(), secondPackage) {
			t.Fatalf("validateSQLCPackageRegistry() error = %v, want missing registered package", err)
		}
	})
}

func writeSQLCConfig(t *testing.T, root string, packagePath string) {
	t.Helper()

	dir := filepath.Join(root, packagePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sqlc.yaml"), []byte("version: \"2\"\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(sqlc.yaml) error = %v", err)
	}
}
