package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Version_And_StoreDir(t *testing.T) {
	t.Run("Should read version from env or fallback", func(t *testing.T) {
		t.Setenv("COMPOZY_VERSION", "v1.2.3")
		assert.Equal(t, "v1.2.3", GetVersion())
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
		assert.Equal(t, StatusPending, ToStatus(AgentStatusUnspecified.String()))
		assert.Equal(t, StatusRunning, ToStatus(TaskStatusRunning.String()))
		assert.Equal(t, StatusSuccess, ToStatus(WorkflowStatusSuccess.String()))
		assert.Equal(t, StatusFailed, ToStatus(ToolStatusFailed.String()))
		assert.Equal(t, StatusWaiting, ToStatus(TaskStatusWaiting.String()))
		assert.Equal(t, StatusPaused, ToStatus(WorkflowStatusPaused.String()))
		assert.Equal(t, StatusCanceled, ToStatus(TaskStatusCanceled.String()))
		assert.Equal(t, StatusTimedOut, ToStatus(WorkflowStatusTimedOut.String()))
		assert.Equal(t, StatusPending, ToStatus("UNKNOWN"))
	})
}
