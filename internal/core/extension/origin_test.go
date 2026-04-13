package extensions

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInstallOriginRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	want := InstallOrigin{
		Remote:         "github",
		Repository:     "compozy/compozy",
		Ref:            "v1.2.3",
		Subdir:         "extensions/cy-idea-factory",
		ResolvedSource: "https://codeload.github.com/compozy/compozy/tar.gz/v1.2.3",
		InstalledAt:    time.Date(2026, time.April, 13, 12, 34, 56, 0, time.UTC),
	}

	if err := WriteInstallOrigin(dir, want); err != nil {
		t.Fatalf("WriteInstallOrigin() error = %v", err)
	}

	got, err := LoadInstallOrigin(dir)
	if err != nil {
		t.Fatalf("LoadInstallOrigin() error = %v", err)
	}
	if got == nil {
		t.Fatal("LoadInstallOrigin() returned nil origin")
	}
	if *got != want {
		t.Fatalf("unexpected install origin\nwant: %#v\ngot:  %#v", want, *got)
	}
}

func TestLoadInstallOriginReturnsNilWhenMissing(t *testing.T) {
	t.Parallel()

	got, err := LoadInstallOrigin(t.TempDir())
	if err != nil {
		t.Fatalf("LoadInstallOrigin() error = %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil origin when file is missing, got %#v", got)
	}
}

func TestWriteInstallOriginRejectsEmptyDirectory(t *testing.T) {
	t.Parallel()

	if err := WriteInstallOrigin("", InstallOrigin{}); err == nil {
		t.Fatal("expected empty directory write to fail")
	}
}

func TestLoadInstallOriginRejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, InstallOriginFileName)
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}

	if _, err := LoadInstallOrigin(dir); err == nil {
		t.Fatal("expected invalid provenance JSON to fail")
	}
}
