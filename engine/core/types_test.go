package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Version_And_StoreDir(t *testing.T) {
	t.Run("Should read version from env", func(t *testing.T) {
		t.Setenv("COMPOZY_VERSION", "v1.2.3")
		assert.Equal(t, "v1.2.3", GetVersion())
	})
	t.Run("Should fallback when env is unset", func(t *testing.T) {
		os.Unsetenv("COMPOZY_VERSION")
		assert.Equal(t, "v0", GetVersion())
	})
	t.Run("Should resolve store dir", func(t *testing.T) {
		assert.Equal(t, ".compozy", GetStoreDir(""))
		base := t.TempDir()
		assert.Equal(t, filepath.Join(base, ".compozy"), GetStoreDir(base))
	})
}

func Test_Stringers_And_Status(t *testing.T) {
	t.Run("Should stringify types", func(t *testing.T) {
		assert.Equal(t, "trigger", CmdType("trigger").String())
		assert.Equal(t, "dispatched", EvtType("dispatched").String())
		assert.Equal(t, "worker.Worker", SourceType("worker.Worker").String())
	})
	t.Run("Should validate and convert statuses", func(t *testing.T) {
		assert.True(t, StatusPending.IsValid())
		assert.False(t, StatusType("X").IsValid())
		cases := []struct {
			name string
			in   string
			want StatusType
		}{
			{"unspecified->pending", AgentStatusUnspecified.String(), StatusPending},
			{"running->running", TaskStatusRunning.String(), StatusRunning},
			{"success->success", WorkflowStatusSuccess.String(), StatusSuccess},
			{"failed->failed", ToolStatusFailed.String(), StatusFailed},
			{"waiting->waiting", TaskStatusWaiting.String(), StatusWaiting},
			{"paused->paused", WorkflowStatusPaused.String(), StatusPaused},
			{"canceled->canceled", TaskStatusCanceled.String(), StatusCanceled},
			{"timed_out->timed_out", WorkflowStatusTimedOut.String(), StatusTimedOut},
			{"unknown->pending", "UNKNOWN", StatusPending},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) { assert.Equal(t, tc.want, ToStatus(tc.in)) })
		}
	})
}
