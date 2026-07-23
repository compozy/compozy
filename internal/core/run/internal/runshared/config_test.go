package runshared

import (
	"testing"

	"github.com/compozy/compozy/internal/core/model"
)

func TestCountJobsIssues(t *testing.T) {
	t.Parallel()
	// One job packs two issues under a single primary file group; the run-level
	// total must count every issue across jobs, not just the job count, so a
	// "1 job / 2 issues" run never looks as if an issue were dropped.
	jobs := []Job{
		{Groups: map[string][]model.IssueEntry{
			"pkg/contract/patientCare.ts": {{Name: "issue_001"}, {Name: "issue_002"}},
		}},
		{Groups: map[string][]model.IssueEntry{
			"pkg/api/routes.ts": {{Name: "issue_003"}},
		}},
	}
	if got := CountJobsIssues(jobs); got != 3 {
		t.Fatalf("CountJobsIssues() = %d, want 3", got)
	}
	if got := CountJobsIssues(nil); got != 0 {
		t.Fatalf("CountJobsIssues(nil) = %d, want 0", got)
	}
}

func TestJobCodeFileLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		codeFiles []string
		want      string
	}{
		{
			name:      "empty",
			codeFiles: nil,
			want:      "",
		},
		{
			name:      "single file",
			codeFiles: []string{"task_01"},
			want:      "task_01",
		},
		{
			name:      "multiple files",
			codeFiles: []string{"task_01", "task_02", "task_03"},
			want:      "task_01, task_02, task_03",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			j := Job{CodeFiles: append([]string(nil), tt.codeFiles...)}
			if got := j.CodeFileLabel(); got != tt.want {
				t.Fatalf("codeFileLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}
