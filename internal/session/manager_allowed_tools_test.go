package session

import (
	"errors"
	"strings"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func TestAllowedToolsOverridePolicyHelpers(t *testing.T) {
	t.Parallel()
	catalog, err := toolspkg.NewToolsetCatalog(toolspkg.Toolset{
		ID:    toolspkg.ToolsetIDTasks,
		Tools: []string{"agh__task_*"},
	})
	if err != nil {
		t.Fatalf("NewToolsetCatalog() error = %v", err)
	}

	t.Run("Should accept unrestricted agent profiles", func(t *testing.T) {
		t.Parallel()

		err := validateAllowedToolsOverrideSubset(aghconfig.ResolvedAgent{}, []string{
			toolspkg.ToolIDTaskRead.String(),
		}, catalog)
		if err != nil {
			t.Fatalf("validateAllowedToolsOverrideSubset(unrestricted) error = %v", err)
		}
	})

	t.Run("Should accept wildcard agent profile matches", func(t *testing.T) {
		t.Parallel()

		err := validateAllowedToolsOverrideSubset(aghconfig.ResolvedAgent{
			Tools: []string{"agh__task_*"},
		}, []string{
			toolspkg.ToolIDTaskRead.String(),
		}, catalog)
		if err != nil {
			t.Fatalf("validateAllowedToolsOverrideSubset(wildcard) error = %v", err)
		}
	})

	t.Run("Should reject tools denied by the agent profile", func(t *testing.T) {
		t.Parallel()

		err := validateAllowedToolsOverrideSubset(aghconfig.ResolvedAgent{
			DenyTools: []string{"agh__task_*"},
		}, []string{
			toolspkg.ToolIDTaskRead.String(),
		}, catalog)
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("validateAllowedToolsOverrideSubset(denied) error = %v, want %v", err, ErrValidation)
		}
		if !strings.Contains(err.Error(), "denied by agent profile") {
			t.Fatalf("validateAllowedToolsOverrideSubset(denied) error = %v, want denied message", err)
		}
	})

	t.Run("Should accept members of toolset-only profiles", func(t *testing.T) {
		t.Parallel()

		err := validateAllowedToolsOverrideSubset(aghconfig.ResolvedAgent{
			Toolsets: []string{toolspkg.ToolsetIDTasks.String()},
		}, []string{
			toolspkg.ToolIDTaskRead.String(),
		}, catalog)
		if err != nil {
			t.Fatalf("validateAllowedToolsOverrideSubset(toolset member) error = %v", err)
		}
	})

	t.Run("Should reject nonmembers of toolset-only profiles", func(t *testing.T) {
		t.Parallel()

		err := validateAllowedToolsOverrideSubset(aghconfig.ResolvedAgent{
			Toolsets: []string{toolspkg.ToolsetIDTasks.String()},
		}, []string{
			toolspkg.ToolIDSessionList.String(),
		}, catalog)
		if !errors.Is(err, ErrValidation) || !strings.Contains(err.Error(), "widens agent profile") {
			t.Fatalf("validateAllowedToolsOverrideSubset(toolset nonmember) error = %v, want widening validation", err)
		}
	})

	t.Run("Should reject invalid agent toolsets", func(t *testing.T) {
		t.Parallel()

		err := validateAllowedToolsOverrideSubset(aghconfig.ResolvedAgent{
			Toolsets: []string{"Bad"},
		}, []string{
			toolspkg.ToolIDTaskRead.String(),
		}, catalog)
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("validateAllowedToolsOverrideSubset(invalid toolset) error = %v, want %v", err, ErrValidation)
		}
		if !strings.Contains(err.Error(), "agent toolsets[0]") {
			t.Fatalf("validateAllowedToolsOverrideSubset(invalid toolset) error = %v, want indexed message", err)
		}
	})

	t.Run("Should reject blank requested tools", func(t *testing.T) {
		t.Parallel()

		_, _, err := normalizeAllowedToolsOverride([]string{" "})
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("normalizeAllowedToolsOverride(blank) error = %v, want %v", err, ErrValidation)
		}
		if !strings.Contains(err.Error(), "allowed_tools[0]") {
			t.Fatalf("normalizeAllowedToolsOverride(blank) error = %v, want indexed message", err)
		}
	})
}
