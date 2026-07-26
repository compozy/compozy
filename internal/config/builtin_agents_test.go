package config

import (
	"slices"
	"strings"
	"testing"
)

func TestBuiltinAgentDefReturnsRuntimeOwnedIdentities(t *testing.T) {
	t.Parallel()

	t.Run("Should return the coordinator identity with a real unpinned prompt", func(t *testing.T) {
		t.Parallel()

		got, ok := BuiltinAgentDef("coordinator")
		if !ok {
			t.Fatal("BuiltinAgentDef(coordinator) ok = false, want true")
		}
		if got.Name != "coordinator" || strings.TrimSpace(got.Prompt) == "" {
			t.Fatalf("BuiltinAgentDef(coordinator) = %#v, want named non-empty identity", got)
		}
		if got.Prompt == "AGH coordinator agent identity." {
			t.Fatal("BuiltinAgentDef(coordinator).Prompt uses the deleted placeholder")
		}
		if got.Provider != "" || got.Model != "" {
			t.Fatalf("BuiltinAgentDef(coordinator) route = %q/%q, want unpinned", got.Provider, got.Model)
		}
	})

	t.Run("Should return the dreaming curator identity", func(t *testing.T) {
		t.Parallel()

		got, ok := BuiltinAgentDef("dreaming-curator")
		if !ok || got.Name != "dreaming-curator" || strings.TrimSpace(got.Prompt) == "" {
			t.Fatalf("BuiltinAgentDef(dreaming-curator) = (%#v, %t), want named non-empty identity", got, ok)
		}
	})

	t.Run("Should reject catalog and unknown names", func(t *testing.T) {
		t.Parallel()

		for _, name := range []string{"general", "unknown"} {
			if got, ok := BuiltinAgentDef(name); ok {
				t.Errorf("BuiltinAgentDef(%q) = %#v, true; want false", name, got)
			}
		}
	})
}

func TestBuiltinAgentNamesOwnExactReservations(t *testing.T) {
	t.Parallel()

	t.Run("Should return the closed builtin roster", func(t *testing.T) {
		t.Parallel()

		if got, want := BuiltinAgentNames(), []string{"coordinator", "dreaming-curator"}; !slices.Equal(got, want) {
			t.Fatalf("BuiltinAgentNames() = %#v, want %#v", got, want)
		}
	})

	t.Run("Should normalize exact builtin names", func(t *testing.T) {
		t.Parallel()

		for _, name := range []string{"coordinator", " COORDINATOR ", "dreaming-curator", " DREAMING-CURATOR "} {
			if !IsReservedAgentName(name) {
				t.Errorf("IsReservedAgentName(%q) = false, want true", name)
			}
		}
	})

	t.Run("Should not reserve catalog or prefix names", func(t *testing.T) {
		t.Parallel()

		for _, name := range []string{"general", "coordinator-helper"} {
			if IsReservedAgentName(name) {
				t.Errorf("IsReservedAgentName(%q) = true, want false", name)
			}
		}
	})
}
