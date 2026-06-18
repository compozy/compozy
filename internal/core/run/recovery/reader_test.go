package recovery

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/core/model"
)

type resultFixture struct {
	SchemaVersion int          `json:"schema_version"`
	RunID         string       `json:"run_id"`
	Status        RunStatus    `json:"status"`
	ArtifactsDir  string       `json:"artifacts_dir"`
	ResultPath    string       `json:"result_path"`
	Jobs          []JobOutcome `json:"jobs"`
}

func TestReadRunOutcome(t *testing.T) {
	t.Parallel()

	t.Run("Should return failed job safe names", func(t *testing.T) {
		t.Parallel()

		artifacts := writeResultFixture(t, func(result *resultFixture) {
			result.Status = StatusFailed
			result.Jobs = []JobOutcome{
				{SafeName: "task_01", Status: StatusSucceeded, ExitCode: 0},
				{SafeName: "task_02", Status: StatusFailed, ExitCode: 1},
			}
		})

		outcome, err := ReadRunOutcome(artifacts)
		if err != nil {
			t.Fatalf("ReadRunOutcome() error = %v", err)
		}
		if outcome == nil {
			t.Fatal("expected outcome")
		}
		if got := outcome.FailedJobIDs(); !reflect.DeepEqual(got, []string{"task_02"}) {
			t.Fatalf("FailedJobIDs() = %#v", got)
		}
	})

	t.Run("Should report canceled run or job", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name   string
			mutate func(*resultFixture)
		}{
			{
				name: "run status",
				mutate: func(result *resultFixture) {
					result.Status = StatusCanceled
					result.Jobs = []JobOutcome{{SafeName: "task_01", Status: StatusSucceeded, ExitCode: 0}}
				},
			},
			{
				name: "job status",
				mutate: func(result *resultFixture) {
					result.Status = StatusFailed
					result.Jobs = []JobOutcome{{SafeName: "task_01", Status: StatusCanceled, ExitCode: -1}}
				},
			},
		}

		for _, tc := range tests {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				outcome, err := ReadRunOutcome(writeResultFixture(t, tc.mutate))
				if err != nil {
					t.Fatalf("ReadRunOutcome() error = %v", err)
				}
				if outcome == nil || !outcome.Canceled() {
					t.Fatalf("expected canceled outcome, got %#v", outcome)
				}
			})
		}
	})

	t.Run("Should report timeout jobs", func(t *testing.T) {
		t.Parallel()

		artifacts := writeResultFixture(t, func(result *resultFixture) {
			result.Status = StatusFailed
			result.Jobs = []JobOutcome{{SafeName: "task_timeout", Status: StatusFailed, ExitCode: TimeoutExitCode}}
		})

		outcome, err := ReadRunOutcome(artifacts)
		if err != nil {
			t.Fatalf("ReadRunOutcome() error = %v", err)
		}
		if outcome == nil || !outcome.TimedOut() || !outcome.Jobs[0].TimedOut() {
			t.Fatalf("expected timeout outcome, got %#v", outcome)
		}
		if got := outcome.TimeoutJobIDs(); !reflect.DeepEqual(got, []string{"task_timeout"}) {
			t.Fatalf("TimeoutJobIDs() = %#v", got)
		}
	})

	t.Run("Should return no outcome for missing result", func(t *testing.T) {
		t.Parallel()

		artifacts := model.NewRunArtifacts(t.TempDir(), "missing-result")
		outcome, err := ReadRunOutcome(artifacts)
		if err == nil {
			t.Fatal("expected missing result error")
		}
		if outcome != nil {
			t.Fatalf("expected no outcome, got %#v", outcome)
		}
		if !strings.Contains(err.Error(), "read run result") {
			t.Fatalf("expected descriptive read error, got %v", err)
		}
	})

	t.Run("Should return no outcome for corrupt result", func(t *testing.T) {
		t.Parallel()

		artifacts := model.NewRunArtifacts(t.TempDir(), "corrupt-result")
		if err := os.MkdirAll(filepath.Dir(artifacts.ResultPath), 0o755); err != nil {
			t.Fatalf("mkdir result dir: %v", err)
		}
		if err := os.WriteFile(artifacts.ResultPath, []byte(`{"schema_version":`), 0o600); err != nil {
			t.Fatalf("write corrupt result: %v", err)
		}

		outcome, err := ReadRunOutcome(artifacts)
		if err == nil {
			t.Fatal("expected corrupt result error")
		}
		if outcome != nil {
			t.Fatalf("expected no outcome, got %#v", outcome)
		}
		if !strings.Contains(err.Error(), "decode run result") {
			t.Fatalf("expected descriptive decode error, got %v", err)
		}
	})

	t.Run("Should reject schema version mismatch", func(t *testing.T) {
		t.Parallel()

		artifacts := writeResultFixture(t, func(result *resultFixture) {
			result.SchemaVersion = 99
		})

		outcome, err := ReadRunOutcome(artifacts)
		if err == nil {
			t.Fatal("expected schema mismatch error")
		}
		if outcome != nil {
			t.Fatalf("expected no outcome, got %#v", outcome)
		}
		if !strings.Contains(err.Error(), "unsupported run result schema_version 99") {
			t.Fatalf("expected schema version error, got %v", err)
		}
	})
}

func writeResultFixture(t *testing.T, mutate func(*resultFixture)) model.RunArtifacts {
	t.Helper()

	artifacts := model.NewRunArtifacts(t.TempDir(), "reader-test-run")
	result := resultFixture{
		SchemaVersion: ResultSchemaVersion,
		RunID:         artifacts.RunID,
		Status:        StatusSucceeded,
		ArtifactsDir:  artifacts.RunDir,
		ResultPath:    artifacts.ResultPath,
		Jobs:          []JobOutcome{{SafeName: "task_01", Status: StatusSucceeded, ExitCode: 0}},
	}
	if mutate != nil {
		mutate(&result)
	}
	if err := os.MkdirAll(filepath.Dir(artifacts.ResultPath), 0o755); err != nil {
		t.Fatalf("mkdir result dir: %v", err)
	}
	payload, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("marshal result fixture: %v", err)
	}
	if err := os.WriteFile(artifacts.ResultPath, payload, 0o600); err != nil {
		t.Fatalf("write result fixture: %v", err)
	}
	return artifacts
}
