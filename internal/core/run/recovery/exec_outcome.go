package recovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
	execpkg "github.com/compozy/compozy/internal/core/run/exec"
)

const execRecoveryJobID = "exec"

type persistedExecTurnOutcome struct {
	Status        RunStatus `json:"status"`
	ResultPath    string    `json:"result_path,omitempty"`
	StdoutLogPath string    `json:"stdout_log_path,omitempty"`
	StderrLogPath string    `json:"stderr_log_path,omitempty"`
	Error         string    `json:"error,omitempty"`
}

// ReadExecRunOutcome reads the persisted exec run/turn artifacts as a recovery
// outcome. Missing or corrupt artifacts remain fail-safe: callers get the
// original execution error when one exists.
func ReadExecRunOutcome(
	ctx context.Context,
	cfg *model.RuntimeConfig,
	artifacts model.RunArtifacts,
	executeErr error,
) (RunOutcome, error) {
	record, err := execpkg.LoadPersistedExecRun(workspaceRootForOutcome(cfg), artifacts.RunID)
	if err != nil {
		if executeErr != nil {
			return RunOutcome{}, executeErr
		}
		return RunOutcome{}, err
	}
	status := execOutcomeStatus(ctx, RunStatus(record.Status), executeErr)
	turn := readLastExecTurnOutcome(artifacts, record.TurnCount)
	jobErr := strings.TrimSpace(turn.Error)
	if jobErr == "" {
		jobErr = strings.TrimSpace(record.LastError)
	}
	if jobErr == "" && executeErr != nil {
		jobErr = executeErr.Error()
	}
	return RunOutcome{
		RunID:        artifacts.RunID,
		Status:       status,
		ArtifactsDir: artifacts.RunDir,
		ResultPath:   turn.ResultPath,
		Jobs: []JobOutcome{{
			SafeName: execRecoveryJobID,
			Status:   status,
			ExitCode: execRecoveryExitCode(status),
			OutLog:   turn.StdoutLogPath,
			ErrLog:   turn.StderrLogPath,
			Error:    jobErr,
		}},
	}, executeErr
}

func workspaceRootForOutcome(cfg *model.RuntimeConfig) string {
	if cfg == nil {
		return ""
	}
	return cfg.WorkspaceRoot
}

func execOutcomeStatus(ctx context.Context, status RunStatus, executeErr error) RunStatus {
	if executionCanceled(ctx, executeErr) {
		return StatusCanceled
	}
	switch status {
	case StatusSucceeded, StatusFailed, StatusCanceled:
		return status
	default:
		if executeErr != nil {
			return StatusFailed
		}
		return StatusUnknown
	}
}

func executionCanceled(ctx context.Context, err error) bool {
	if errors.Is(err, context.Canceled) {
		return true
	}
	if ctx == nil {
		return false
	}
	return errors.Is(ctx.Err(), context.Canceled) || errors.Is(context.Cause(ctx), context.Canceled)
}

func execRecoveryExitCode(status RunStatus) int {
	switch status {
	case StatusSucceeded:
		return 0
	case StatusCanceled:
		return -1
	default:
		return 1
	}
}

func readLastExecTurnOutcome(artifacts model.RunArtifacts, turnCount int) persistedExecTurnOutcome {
	if turnCount <= 0 || strings.TrimSpace(artifacts.TurnsDir) == "" {
		return persistedExecTurnOutcome{}
	}
	path := filepath.Join(artifacts.TurnsDir, fmt.Sprintf("%04d", turnCount), "result.json")
	payload, err := os.ReadFile(path)
	if err != nil {
		return persistedExecTurnOutcome{}
	}
	var turn persistedExecTurnOutcome
	if err := json.Unmarshal(payload, &turn); err != nil {
		return persistedExecTurnOutcome{}
	}
	if strings.TrimSpace(turn.ResultPath) == "" {
		turn.ResultPath = path
	}
	return turn
}
