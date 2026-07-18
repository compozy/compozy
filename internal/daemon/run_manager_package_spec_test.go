package daemon

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/core/model"
)

// TestEnsureCurrentPackageSpecifications guards the lifecycle-resolution contract
// for missing canonical specs.
// CONTRACT: nested-workflows/reviews-007/issue_005. A package whose initiative is
// missing a canonical spec must surface a typed, client-actionable 422 problem
// (not a generic error that leaks the absolute SpecDir), while a genuine read
// fault stays an untyped internal error so the transport still maps it to 500.
func TestEnsureCurrentPackageSpecifications(t *testing.T) {
	t.Parallel()

	writeSpec := func(t *testing.T, dir, name string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte("# spec\n"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	t.Run("Should return nil when both canonical specs exist", func(t *testing.T) {
		t.Parallel()
		specDir := t.TempDir()
		writeSpec(t, specDir, "_prd.md")
		writeSpec(t, specDir, "_techspec.md")

		scope := model.ExecutionScope{SpecDir: specDir, WorkflowRef: "customer-management/WP-001"}
		if err := ensureCurrentPackageSpecifications(scope); err != nil {
			t.Fatalf("ensureCurrentPackageSpecifications(complete) error = %v, want nil", err)
		}
	})

	t.Run("Should return typed 422 problem when _techspec.md is missing", func(t *testing.T) {
		t.Parallel()
		specDir := t.TempDir()
		writeSpec(t, specDir, "_prd.md") // PRD present, techspec absent: the reported scenario.

		ref := "customer-management/WP-001"
		scope := model.ExecutionScope{SpecDir: specDir, WorkflowRef: ref}
		err := ensureCurrentPackageSpecifications(scope)

		var problem *apicore.Problem
		if !errors.As(err, &problem) {
			t.Fatalf("ensureCurrentPackageSpecifications(missing techspec) error = %v, want *apicore.Problem", err)
		}
		if problem.Status != http.StatusUnprocessableEntity {
			t.Fatalf("problem.Status = %d, want %d", problem.Status, http.StatusUnprocessableEntity)
		}
		if problem.Code != "package_specification_missing" {
			t.Fatalf("problem.Code = %q, want package_specification_missing", problem.Code)
		}
		if got := problem.Details["specification"]; got != "_techspec.md" {
			t.Fatalf("problem.Details[specification] = %#v, want _techspec.md", got)
		}
		if got := problem.Details["workflow"]; got != ref {
			t.Fatalf("problem.Details[workflow] = %#v, want %q", got, ref)
		}
		// The message must stay actionable and must not leak the absolute SpecDir.
		if strings.Contains(problem.Message, specDir) {
			t.Fatalf("problem.Message leaks SpecDir %q: %q", specDir, problem.Message)
		}
		// The underlying os.ErrNotExist stays wrapped for server-side diagnostics.
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("problem does not wrap os.ErrNotExist: %v", err)
		}
	})

	t.Run("Should return an untyped error when the spec read fails for a non-missing reason", func(t *testing.T) {
		t.Parallel()
		// A SpecDir path that points at a regular file makes filepath.Join(...)/_prd.md
		// a non-existent child, but reading through a non-directory yields ENOTDIR
		// rather than a plain "not exist", exercising the non-ENOENT branch.
		base := t.TempDir()
		notADir := filepath.Join(base, "specfile")
		if err := os.WriteFile(notADir, []byte("x"), 0o600); err != nil {
			t.Fatalf("write specfile: %v", err)
		}

		scope := model.ExecutionScope{SpecDir: notADir, WorkflowRef: "customer-management/WP-001"}
		err := ensureCurrentPackageSpecifications(scope)
		if err == nil {
			t.Fatal("ensureCurrentPackageSpecifications(non-dir SpecDir) error = nil, want error")
		}
		var problem *apicore.Problem
		if errors.As(err, &problem) {
			t.Fatalf("non-missing read fault mapped to typed problem: %v", err)
		}
	})
}
