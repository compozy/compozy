package model

import (
	"errors"
	"fmt"
	"testing"
)

func TestAutomationListPage(t *testing.T) {
	t.Parallel()

	t.Run("Should search and order jobs before applying the page limit", func(t *testing.T) {
		t.Parallel()

		jobs := []Job{
			listTestJob("job-dynamic-z", "Zulu needle", JobSourceDynamic, "workspace-a"),
			listTestJob("job-config-b", "Bravo needle", JobSourceConfig, "workspace-a"),
			listTestJob("job-package-a", "Alpha needle", JobSourcePackage, "workspace-a"),
			listTestJob("job-config-a2", "Alpha needle", JobSourceConfig, "workspace-a"),
			listTestJob("job-config-a1", "Alpha needle", JobSourceConfig, "workspace-a"),
			listTestJob("job-unmatched", "Unmatched", JobSourceConfig, "workspace-a"),
			listTestJob("job-other-workspace", "Needle elsewhere", JobSourceConfig, "workspace-b"),
		}

		page, err := BuildJobListPage(jobs, JobListQuery{
			Scope:       AutomationScopeWorkspace,
			WorkspaceID: "workspace-a",
			Search:      "  NEEDLE  ",
			Limit:       3,
		})
		if err != nil {
			t.Fatalf("BuildJobListPage() error = %v", err)
		}

		if got, want := page.Total, 5; got != want {
			t.Fatalf("BuildJobListPage().Total = %d, want %d", got, want)
		}
		if got, want := page.Limit, 3; got != want {
			t.Fatalf("BuildJobListPage().Limit = %d, want %d", got, want)
		}
		if !page.HasMore || page.NextCursor == "" {
			t.Fatalf("BuildJobListPage() pagination = %#v, want another page", page)
		}
		assertListJobIDs(t, page.Jobs, []string{"job-config-a1", "job-config-a2", "job-config-b"})
	})

	t.Run("Should walk job pages without gaps or duplicates", func(t *testing.T) {
		t.Parallel()

		jobs := make([]Job, 0, 63)
		for index := range 63 {
			source := JobSourceDynamic
			if index < 21 {
				source = JobSourceConfig
			} else if index < 42 {
				source = JobSourcePackage
			}
			job := listTestJob(
				fmt.Sprintf("job-%03d", index),
				fmt.Sprintf("Needle %03d", 62-index),
				source,
				"workspace-a",
			)
			job.TargetKind = TargetKindLoop
			job.LoopTarget = &LoopTarget{WorkspaceID: "workspace-a", LoopName: "triage"}
			jobs = append(jobs, job)
		}

		query := JobListQuery{
			Scope:       AutomationScopeWorkspace,
			WorkspaceID: "workspace-a",
			LoopName:    "triage",
			Search:      "needle",
			Limit:       7,
		}
		seen := make(map[string]struct{}, len(jobs))
		for {
			page, err := BuildJobListPage(jobs, query)
			if err != nil {
				t.Fatalf("BuildJobListPage(cursor=%q) error = %v", query.Cursor, err)
			}
			if got, want := page.Total, len(jobs); got != want {
				t.Fatalf("BuildJobListPage().Total = %d, want %d", got, want)
			}
			for _, job := range page.Jobs {
				if _, duplicate := seen[job.ID]; duplicate {
					t.Fatalf("BuildJobListPage() duplicated job %q", job.ID)
				}
				seen[job.ID] = struct{}{}
			}
			if !page.HasMore {
				if page.NextCursor != "" {
					t.Fatalf("final BuildJobListPage().NextCursor = %q, want empty", page.NextCursor)
				}
				break
			}
			if page.NextCursor == "" {
				t.Fatal("BuildJobListPage().NextCursor = empty while HasMore is true")
			}
			query.Cursor = page.NextCursor
		}

		if got, want := len(seen), len(jobs); got != want {
			t.Fatalf("walked job count = %d, want %d", got, want)
		}
	})

	t.Run("Should reject a cursor when the job query changes", func(t *testing.T) {
		t.Parallel()

		jobs := []Job{
			listTestJob("job-a", "Alpha needle", JobSourceConfig, "workspace-a"),
			listTestJob("job-b", "Bravo needle", JobSourceDynamic, "workspace-a"),
		}
		page, err := BuildJobListPage(jobs, JobListQuery{Search: "needle", Limit: 1})
		if err != nil {
			t.Fatalf("BuildJobListPage() error = %v", err)
		}

		_, err = BuildJobListPage(jobs, JobListQuery{
			Search: "different",
			Limit:  1,
			Cursor: page.NextCursor,
		})
		if !errors.Is(err, ErrListCursorInvalid) {
			t.Fatalf("BuildJobListPage(changed query) error = %v, want ErrListCursorInvalid", err)
		}
	})

	t.Run("Should search trigger filter keys and values before paging", func(t *testing.T) {
		t.Parallel()

		triggers := []Trigger{
			listTestTrigger("trigger-dynamic", "Dynamic", JobSourceDynamic, map[string]string{"branch": "main"}),
			listTestTrigger("trigger-package", "Package", JobSourcePackage, map[string]string{"repository": "api"}),
			listTestTrigger("trigger-config", "Config", JobSourceConfig, map[string]string{"team": "repository"}),
		}

		page, err := BuildTriggerListPage(triggers, TriggerListQuery{Search: "repository", Limit: 1})
		if err != nil {
			t.Fatalf("BuildTriggerListPage() error = %v", err)
		}
		if got, want := page.Total, 2; got != want {
			t.Fatalf("BuildTriggerListPage().Total = %d, want %d", got, want)
		}
		if !page.HasMore {
			t.Fatal("BuildTriggerListPage().HasMore = false, want true")
		}
		if got, want := page.Triggers[0].ID, "trigger-config"; got != want {
			t.Fatalf("BuildTriggerListPage().Triggers[0].ID = %q, want %q", got, want)
		}
	})
}

func listTestJob(id string, name string, source JobSource, workspaceID string) Job {
	return Job{
		ID:          id,
		Scope:       AutomationScopeWorkspace,
		Name:        name,
		AgentName:   "reviewer",
		WorkspaceID: workspaceID,
		Prompt:      "Review the current state",
		Source:      source,
		Schedule: &ScheduleSpec{
			Mode:     ScheduleModeEvery,
			Interval: "1h",
		},
	}
}

func listTestTrigger(id string, name string, source JobSource, filter map[string]string) Trigger {
	return Trigger{
		ID:        id,
		Scope:     AutomationScopeGlobal,
		Name:      name,
		AgentName: "reviewer",
		Prompt:    "Review the current state",
		Event:     "session.stopped",
		Source:    source,
		Filter:    filter,
	}
}

func assertListJobIDs(t *testing.T, jobs []Job, want []string) {
	t.Helper()
	if len(jobs) != len(want) {
		t.Fatalf("job count = %d, want %d", len(jobs), len(want))
	}
	for index := range want {
		if jobs[index].ID != want[index] {
			t.Fatalf("jobs[%d].ID = %q, want %q", index, jobs[index].ID, want[index])
		}
	}
}
