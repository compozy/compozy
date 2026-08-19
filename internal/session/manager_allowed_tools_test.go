package session

import (
	"errors"
	"slices"
	"strings"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func TestAllowedToolsOverridePolicyHelpers(t *testing.T) {
	t.Parallel()
	catalog, err := toolspkg.NewToolsetCatalog(toolspkg.Toolset{
		ID:    toolspkg.ToolsetIDTasks,
		Tools: []string{"compozy__task_*"},
	})
	if err != nil {
		t.Fatalf("NewToolsetCatalog() error = %v", err)
	}

	t.Run("Should accept unrestricted agent profiles", func(t *testing.T) {
		t.Parallel()

		err := validateAllowedToolsOverrideSubset(compozyconfig.ResolvedAgent{}, []string{
			toolspkg.ToolIDTaskRead.String(),
		}, catalog)
		if err != nil {
			t.Fatalf("validateAllowedToolsOverrideSubset(unrestricted) error = %v", err)
		}
	})

	t.Run("Should accept wildcard agent profile matches", func(t *testing.T) {
		t.Parallel()

		err := validateAllowedToolsOverrideSubset(compozyconfig.ResolvedAgent{
			Tools: []string{"compozy__task_*"},
		}, []string{
			toolspkg.ToolIDTaskRead.String(),
		}, catalog)
		if err != nil {
			t.Fatalf("validateAllowedToolsOverrideSubset(wildcard) error = %v", err)
		}
	})

	t.Run("Should reject tools denied by the agent profile", func(t *testing.T) {
		t.Parallel()

		err := validateAllowedToolsOverrideSubset(compozyconfig.ResolvedAgent{
			DenyTools: []string{"compozy__task_*"},
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

		err := validateAllowedToolsOverrideSubset(compozyconfig.ResolvedAgent{
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

		err := validateAllowedToolsOverrideSubset(compozyconfig.ResolvedAgent{
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

		err := validateAllowedToolsOverrideSubset(compozyconfig.ResolvedAgent{
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

	t.Run("Should merge internal deny patterns without removing heartbeat", func(t *testing.T) {
		t.Parallel()

		spec := sessionStartSpec{deniedToolsOverride: []string{
			toolspkg.ToolIDTaskRunFail.String(),
			toolspkg.ToolIDTaskRunComplete.String(),
			toolspkg.ToolIDTaskRunFail.String(),
		}}
		resolved := compozyconfig.ResolvedAgent{DenyTools: []string{"compozy__config_*"}}
		if err := spec.applyDeniedToolsOverride(&resolved); err != nil {
			t.Fatalf("applyDeniedToolsOverride() error = %v", err)
		}
		want := []string{
			"compozy__config_*",
			toolspkg.ToolIDTaskRunComplete.String(),
			toolspkg.ToolIDTaskRunFail.String(),
		}
		if !slices.Equal(resolved.DenyTools, want) {
			t.Fatalf("resolved.DenyTools = %#v, want %#v", resolved.DenyTools, want)
		}
		if slices.Contains(resolved.DenyTools, toolspkg.ToolIDTaskRunHeartbeat.String()) {
			t.Fatalf("resolved.DenyTools = %#v, heartbeat must remain available", resolved.DenyTools)
		}
	})
}
