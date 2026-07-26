//go:build !windows

package toolruntime

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/compozy/agh/internal/procutil"
)

func TestDefaultInterrupterProcessGroups(t *testing.T) {
	t.Parallel()

	t.Run("Should wait for recovered process group descendants before returning", func(t *testing.T) {
		command := `trap 'exit 0' TERM; sh -c 'trap "" TERM; sleep 30' & wait`
		cmd := exec.CommandContext(context.Background(), "sh", "-c", command)
		procutil.ConfigureCommandProcessGroup(cmd)
		if err := cmd.Start(); err != nil {
			t.Fatalf("cmd.Start() error = %v", err)
		}
		if err := procutil.RegisterCommandProcessGroup(cmd); err != nil {
			t.Fatalf("RegisterCommandProcessGroup() error = %v", err)
		}
		pgid := cmd.Process.Pid
		waitDone := make(chan error, 1)
		go func() {
			waitDone <- cmd.Wait()
		}()
		t.Cleanup(func() {
			if err := procutil.KillProcessGroupIDAndWait(pgid, time.Second); err != nil {
				t.Logf("cleanup KillProcessGroupIDAndWait(%d) error: %v", pgid, err)
			}
			select {
			case err := <-waitDone:
				if err != nil {
					t.Logf("cleanup cmd.Wait() error: %v", err)
				}
			case <-time.After(time.Second):
				t.Fatalf("timed out waiting for process group leader cleanup")
			}
		})

		startedAt, err := procutil.StartedAt(cmd.Process.Pid)
		if err != nil {
			t.Fatalf("StartedAt(%d) error = %v", cmd.Process.Pid, err)
		}
		record := ProcessRecord{
			ID:             "proc-group",
			PID:            cmd.Process.Pid,
			ProcessGroupID: pgid,
			StartedAt:      startedAt,
		}

		if err := (defaultInterrupter{}).InterruptProcess(context.Background(), record); err != nil {
			t.Fatalf("InterruptProcess() error = %v", err)
		}
		if err := procutil.WaitForProcessGroupIDExit(pgid, 100*time.Millisecond); err != nil {
			t.Fatalf("process group still alive after InterruptProcess(): %v", err)
		}
	})
}
