package run

import (
	"io"

	execpkg "github.com/compozy/compozy/internal/core/run/exec"
)

type PersistedExecRun = execpkg.PersistedExecRun

func LoadPersistedExecRun(workspaceRoot, runID string) (PersistedExecRun, error) {
	return execpkg.LoadPersistedExecRun(workspaceRoot, runID)
}

func WriteExecJSONFailure(dst io.Writer, runID string, err error) error {
	return execpkg.WriteExecJSONFailure(dst, runID, err)
}

func IsExecErrorReported(err error) bool {
	return execpkg.IsExecErrorReported(err)
}
