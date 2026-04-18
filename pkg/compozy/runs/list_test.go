package runs

import (
	"testing"
	"time"
)

func TestListReturnsRunsSortedAndFiltered(t *testing.T) {
	base := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	reader := &stubDaemonRunReader{
		listSummaries: [][]RunSummary{{
			{
				RunID:         "run-early",
				Status:        "failed",
				Mode:          "prd-tasks",
				WorkspaceRoot: "/workspace",
				StartedAt:     base,
			},
			{
				RunID:         "run-late",
				Status:        "completed",
				Mode:          "exec",
				WorkspaceRoot: "/workspace",
				StartedAt:     base.Add(2 * time.Hour),
			},
			{
				RunID:         "run-middle",
				Status:        "running",
				Mode:          "prd-tasks",
				WorkspaceRoot: "/workspace",
				StartedAt:     base.Add(time.Hour),
			},
		}},
	}
	withStubDaemonRunReader(t, reader)

	got, err := List("/workspace", ListOptions{
		Status: []string{"running", "completed"},
		Mode:   []string{"exec", "prd-tasks"},
		Limit:  2,
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("List() returned %d runs, want 2", len(got))
	}
	if got[0].RunID != "run-late" || got[1].RunID != "run-middle" {
		t.Fatalf("List() order = [%s %s], want [run-late run-middle]", got[0].RunID, got[1].RunID)
	}
	if len(reader.listWorkspaces) != 1 || reader.listWorkspaces[0] != "/workspace" {
		t.Fatalf("listWorkspaces = %#v, want [/workspace]", reader.listWorkspaces)
	}
}

func TestListAppliesTimeBoundsClientSide(t *testing.T) {
	base := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	reader := &stubDaemonRunReader{
		listSummaries: [][]RunSummary{{
			{RunID: "run-1", StartedAt: base},
			{RunID: "run-2", StartedAt: base.Add(time.Hour)},
			{RunID: "run-3", StartedAt: base.Add(2 * time.Hour)},
		}},
	}
	withStubDaemonRunReader(t, reader)

	got, err := List("/workspace", ListOptions{
		Since: base.Add(30 * time.Minute),
		Until: base.Add(90 * time.Minute),
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 1 || got[0].RunID != "run-2" {
		t.Fatalf("List() = %#v, want only run-2", got)
	}
}
