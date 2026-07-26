package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepairDotEnvFilePermissionsContract(t *testing.T) {
	t.Parallel()

	t.Run("Should tighten repaired dotenv files to owner read write", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), ".env")
		contents := "OPENAI_API_KEY=sk-live\u200b ANTHROPIC_API_KEY=anthropic\u2011key\n"
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatalf("WriteFile(.env) error = %v", err)
		}

		report, err := RepairDotEnvFile(path)
		if err != nil {
			t.Fatalf("RepairDotEnvFile() error = %v", err)
		}
		if report.Status != DotEnvStatusRepaired || !report.Repaired {
			t.Fatalf("RepairDotEnvFile() = %#v, want repaired status", report)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat(.env) error = %v", err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf(".env mode = %#o, want %#o", got, os.FileMode(0o600))
		}
	})
}
