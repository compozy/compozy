package main

// Suite: ACP mock diagnostics writer.
// Invariant: process ownership overrides caller-provided diagnostic ownership.
// Boundary IN: writer stamping and JSONL persistence.
// Boundary OUT: subprocess environment propagation, owned by internal/testutil/acpmock/driver_test.go.

import (
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/internal/testutil/acpmock"
)

func TestWriteDiagnosticsStampsProcessOwner(t *testing.T) {
	// not parallel: t.Setenv owns the process environment until this subtest completes.
	t.Run("Should replace a caller-supplied owner with the trimmed process owner", func(t *testing.T) {
		t.Setenv("COMPOZY_SESSION_ID", "  compozy-session-process  ")

		diagnosticsPath := filepath.Join(t.TempDir(), "owned-diagnostics.jsonl")
		agent := &mockAgent{diagnosticsPath: diagnosticsPath}
		if err := agent.writeDiagnostics(acpmock.DiagnosticsRecord{
			CompozySessionID: "compozy-session-forged",
			AgentName:        "owner-test",
		}); err != nil {
			t.Fatalf("writeDiagnostics() error = %v", err)
		}

		records, err := acpmock.ReadDiagnostics(diagnosticsPath)
		if err != nil {
			t.Fatalf("ReadDiagnostics() error = %v", err)
		}
		if got, want := len(records), 1; got != want {
			t.Fatalf("ReadDiagnostics() = %#v, want one record", records)
		}
		if got, want := records[0].CompozySessionID, "compozy-session-process"; got != want {
			t.Fatalf("CompozySessionID = %q, want %q", got, want)
		}
	})
}
