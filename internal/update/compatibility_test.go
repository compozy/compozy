package update

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeCompatibilityBackstop(t *testing.T) {
	t.Run("Should refuse a runtime requiring a newer installed app", func(t *testing.T) {
		t.Parallel()

		err := CheckRuntimeCompatibility(Compatibility{
			RuntimeVersion: "v1.2.0",
			MinAppVersion:  "v1.1.0",
		}, InstalledApp{Present: true, Version: "v1.0.0"})
		want := "update: runtime v1.2.0 requires CompozyOS app v1.1.0 or newer; installed app is v1.0.0"
		if err == nil || err.Error() != want {
			t.Fatalf("CheckRuntimeCompatibility() error = %v, want %q", err, want)
		}
	})

	t.Run("Should pass when no desktop app is installed", func(t *testing.T) {
		t.Parallel()

		err := CheckRuntimeCompatibility(Compatibility{
			RuntimeVersion: "v1.2.0",
			MinAppVersion:  "v9.0.0",
		}, InstalledApp{})
		if err != nil {
			t.Fatalf("CheckRuntimeCompatibility(no app) error = %v", err)
		}
	})

	t.Run("Should pass when the installed app meets the minimum", func(t *testing.T) {
		t.Parallel()

		err := CheckRuntimeCompatibility(Compatibility{
			RuntimeVersion: "v1.2.0",
			MinAppVersion:  "v1.1.0",
		}, InstalledApp{Present: true, Version: "v1.1.0"})
		if err != nil {
			t.Fatalf("CheckRuntimeCompatibility(equal app) error = %v", err)
		}
	})

	t.Run("Should refuse an installed app whose version is unavailable", func(t *testing.T) {
		t.Parallel()

		err := CheckRuntimeCompatibility(Compatibility{
			RuntimeVersion: "v1.2.0",
			MinAppVersion:  "v1.1.0",
		}, InstalledApp{Present: true})
		if err == nil || !strings.Contains(err.Error(), "version is unavailable") {
			t.Fatalf("CheckRuntimeCompatibility() error = %v, want unavailable-version refusal", err)
		}
	})
}

func TestCompatibilityAsset(t *testing.T) {
	t.Run("Should reject trailing JSON after the compatibility contract", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "compat.json")
		contents := []byte(`{"runtime_version":"v1.1.0","min_app_version":"v1.0.0"}{}`)
		if err := os.WriteFile(path, contents, 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		_, err := readCompatibility(path)
		if err == nil || !strings.Contains(err.Error(), "multiple JSON values") {
			t.Fatalf("readCompatibility() error = %v, want trailing-value refusal", err)
		}
	})

	t.Run("Should reject compatibility assets above the size limit", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "compat.json")
		if err := os.WriteFile(path, bytes.Repeat([]byte("x"), int(maxCompatibilityBytes+1)), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		_, err := readCompatibility(path)
		if err == nil || !strings.Contains(err.Error(), "limit is") {
			t.Fatalf("readCompatibility() error = %v, want size-limit refusal", err)
		}
	})
}
