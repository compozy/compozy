package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/fileutil"
	aghlogger "github.com/compozy/agh/internal/logger"
	"github.com/compozy/agh/internal/procutil"
)

type restartRequestRuntime struct {
	now           func() time.Time
	startedAt     time.Time
	socketPath    string
	activeSession int
	pid           int
	executable    func() (string, error)
	startDetached detachedStartFunc
	signalProcess func(int, syscall.Signal) error
}

func defaultDetachedStart(ctx context.Context, req detachedStartRequest) (restartProcess, error) {
	return procutil.SpawnDetachedLoggedProcess(ctx, procutil.DetachedLaunchRequest{
		Binary:  req.binary,
		Args:    append([]string(nil), req.args...),
		Sandbox: aghlogger.WithMirrorToStderrEnv(append([]string(nil), req.sandbox...), false),
		LogPath: req.logPath,
	})
}

func newRestartStore(homePaths aghconfig.HomePaths, now func() time.Time) *restartStore {
	if now == nil {
		now = func() time.Time {
			return time.Now().UTC()
		}
	}
	return &restartStore{
		homePaths: homePaths,
		now:       now,
	}
}

func (s *restartStore) Create(operation RestartOperation) (RestartOperation, error) {
	now := s.now().UTC()
	if operation.Status == "" {
		operation.Status = RestartStatusPending
	}
	operation.StartedAt = now
	operation.UpdatedAt = now
	operation.CompletedAt = nil
	operation.NewPID = 0
	operation.FailureReason = ""
	if err := operation.Validate(); err != nil {
		return RestartOperation{}, err
	}

	path, err := s.operationPath(operation.OperationID)
	if err != nil {
		return RestartOperation{}, err
	}
	if _, err := os.Stat(path); err == nil {
		return RestartOperation{}, fmt.Errorf("daemon: restart operation %q already exists", operation.OperationID)
	} else if !errors.Is(err, os.ErrNotExist) {
		return RestartOperation{}, fmt.Errorf("daemon: stat restart operation %q: %w", operation.OperationID, err)
	}

	if err := s.write(path, operation); err != nil {
		return RestartOperation{}, err
	}
	return operation, nil
}

func (s *restartStore) Get(operationID string) (RestartOperation, error) {
	path, err := s.operationPath(operationID)
	if err != nil {
		return RestartOperation{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return RestartOperation{}, fmt.Errorf("%w: %s", ErrRestartOperationNotFound, operationID)
		}
		return RestartOperation{}, fmt.Errorf("daemon: read restart operation %q: %w", operationID, err)
	}

	var operation RestartOperation
	if err := json.Unmarshal(data, &operation); err != nil {
		return RestartOperation{}, fmt.Errorf("daemon: decode restart operation %q: %w", operationID, err)
	}
	if err := operation.Validate(); err != nil {
		return RestartOperation{}, err
	}
	return operation, nil
}

func (s *restartStore) Transition(operationID string, transition restartTransition) (RestartOperation, error) {
	current, err := s.Get(operationID)
	if err != nil {
		return RestartOperation{}, err
	}

	next, err := advanceRestartOperation(current, transition, s.now().UTC())
	if err != nil {
		return RestartOperation{}, err
	}

	path, err := s.operationPath(operationID)
	if err != nil {
		return RestartOperation{}, err
	}
	if err := s.write(path, next); err != nil {
		return RestartOperation{}, err
	}
	return next, nil
}

func (s *restartStore) operationPath(operationID string) (string, error) {
	cleanID := strings.TrimSpace(operationID)
	if cleanID == "" {
		return "", errors.New("daemon: restart operation id is required")
	}
	if strings.ContainsAny(cleanID, `/\`) {
		return "", fmt.Errorf("daemon: restart operation id %q contains path separators", cleanID)
	}
	restartsDir := strings.TrimSpace(s.homePaths.RestartsDir)
	if restartsDir == "" {
		return "", errors.New("daemon: restart operations directory is required")
	}
	return filepath.Join(restartsDir, cleanID+".json"), nil
}

func (s *restartStore) write(path string, operation RestartOperation) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("daemon: create restart operation directory for %q: %w", path, err)
	}
	payload, err := json.MarshalIndent(operation, "", "  ")
	if err != nil {
		return fmt.Errorf("daemon: encode restart operation %q: %w", operation.OperationID, err)
	}
	payload = append(payload, '\n')
	if err := fileutil.AtomicWriteFile(path, payload, 0o600); err != nil {
		return fmt.Errorf("daemon: persist restart operation %q: %w", operation.OperationID, err)
	}
	return nil
}
