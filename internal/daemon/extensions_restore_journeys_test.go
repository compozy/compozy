package daemon

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

// Invariant: a restored built-in is usable without a restart (IT-002) and comes back enabled for the default
// profile (IT-007): removal deletes the row's enablement exceptions, and restore enables the row it enrolls.
// Both run on the real registry and the embedded bundled package.

// TestBundledEnrollmentTableOrder guards boot order: boot and restore both read this table, so its
// contents and order must match what boot enrolled before the table existed.
func TestBundledEnrollmentTableOrder(t *testing.T) {
	t.Parallel()

	names := make([]string, 0, len(bundledEnrollments))
	for _, entry := range bundledEnrollments {
		if entry.Enroll == nil {
			t.Fatalf("bundled enrollment %q has no enroll function", entry.Name)
		}
		names = append(names, entry.Name)
	}
	want := []string{"open-design", "spec-cycle", "tailscale", "forge-github"}
	if !slices.Equal(names, want) {
		t.Fatalf("bundled enrollment order = %v, want %v", names, want)
	}
}

func TestBuiltinRestoreMakesBuiltinUsableWithoutRestart(t *testing.T) {
	t.Parallel()

	service, registry := restoreTestService(t)
	restoreTestRemovePermanently(t, service, registry, "spec-cycle")
	names, err := registry.EnabledBundledNames()
	if err != nil {
		t.Fatalf("EnabledBundledNames(removed) error = %v", err)
	}
	if slices.Contains(names, "spec-cycle") {
		t.Fatalf("EnabledBundledNames(removed) = %v, want spec-cycle absent", names)
	}

	item, err := service.RestoreBuiltin(t.Context(), "spec-cycle", restoreTestOperator(t))
	if err != nil {
		t.Fatalf("RestoreBuiltin() error = %v", err)
	}

	// A new session reads the enabled bundled names to grant tool trust, so the restored name must appear there.
	names, err = registry.EnabledBundledNames()
	if err != nil {
		t.Fatalf("EnabledBundledNames(restored) error = %v", err)
	}
	if !slices.Contains(names, "spec-cycle") {
		t.Fatalf("EnabledBundledNames(restored) = %v, want spec-cycle present", names)
	}
	resolver := &nativeToolPolicyResolver{extensionRegistry: registry}
	var inputs toolspkg.PolicyInputs
	if err := resolver.applyBundledExtensionTrust(&inputs); err != nil {
		t.Fatalf("applyBundledExtensionTrust() error = %v", err)
	}
	want := toolspkg.SourceGrant{Kind: toolspkg.SourceExtension, Owner: "spec-cycle"}
	if !slices.Contains(inputs.TrustedSources, want) {
		t.Fatalf("TrustedSources = %#v, want the bundled spec-cycle grant", inputs.TrustedSources)
	}

	// The bundled skills come from the embedded package materialized into the managed install path.
	skillFile := filepath.Join(item.Path, "skills", "cy-create-spec", "SKILL.md")
	if _, err := os.Stat(skillFile); err != nil {
		t.Fatalf("restored bundled skill %q: %v", skillFile, err)
	}
}

func TestBuiltinRestoreEnablesBuiltinDisabledBeforeRemoval(t *testing.T) {
	t.Parallel()

	service, registry := restoreTestService(t)
	restoreTestSeedRow(t, registry, "spec-cycle", extensionpkg.SourceBundled)
	if err := registry.Disable("spec-cycle"); err != nil {
		t.Fatalf("Disable(spec-cycle) error = %v", err)
	}
	if _, err := service.RemovePermanently(t.Context(), "spec-cycle", restoreTestOperator(t)); err != nil {
		t.Fatalf("RemovePermanently(disabled bundled) error = %v", err)
	}
	if _, err := registry.Get("spec-cycle"); !errors.Is(err, extensionpkg.ErrExtensionNotFound) {
		t.Fatalf("Get(after permanent removal) error = %v, want ErrExtensionNotFound", err)
	}

	item, err := service.RestoreBuiltin(t.Context(), "spec-cycle", restoreTestOperator(t))
	if err != nil {
		t.Fatalf("RestoreBuiltin() error = %v", err)
	}
	if !item.Enabled {
		t.Fatalf("RestoreBuiltin payload = %#v, want enabled because removal deleted the enablement row", item)
	}
	installed, err := registry.Get("spec-cycle")
	if err != nil || !installed.Enabled {
		t.Fatalf("Get(restored) = %#v, %v; want an enabled row", installed, err)
	}
}
