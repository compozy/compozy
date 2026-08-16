package update

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/procutil"
	"github.com/gofrs/flock"
)

const (
	operationFileMode = 0o600
	operationDirMode  = 0o700
	maxOperationBytes = int64(1 << 20)
)

var (
	ErrOperationBlocked       = errors.New("update: operation blocked")
	ErrOperationNotFound      = errors.New("update: operation not found")
	ErrOperationConflict      = errors.New("update: operation revision conflict")
	ErrExecutorFenced         = errors.New("update: executor fenced")
	ErrOperationNotCancelable = errors.New("update: operation is not cancelable")
)

// BlockedError reports the live operation that prevented acquisition.
type BlockedError struct {
	Operation Operation
}

func (e *BlockedError) Error() string {
	if e.Operation.Holder == nil {
		return fmt.Sprintf("%s: operation %s is dormant", ErrOperationBlocked, e.Operation.ID)
	}
	return fmt.Sprintf(
		"%s: operation %s is held by %s pid %d",
		ErrOperationBlocked,
		e.Operation.ID,
		e.Operation.Holder.Surface,
		e.Operation.Holder.PID,
	)
}

func (e *BlockedError) Unwrap() error { return ErrOperationBlocked }

// OperationStore owns all reads and writes of the per-home update journal.
type OperationStore struct {
	operationPath string
	lockPath      string
	historyPath   string
	now           func() time.Time
	holderLive    func(*Holder, time.Time) bool
	events        OperationEventEmitter
}

// NewOperationStore binds the journal to one CompozyOS home.
func NewOperationStore(paths compozyconfig.HomePaths, events OperationEventEmitter) (*OperationStore, error) {
	operationPath := firstPath(paths.UpdateOperationFile, filepath.Join(paths.HomeDir, compozyconfig.UpdateOperationFileName))
	lockPath := firstPath(paths.UpdateOperationLock, filepath.Join(paths.HomeDir, compozyconfig.UpdateOperationLockName))
	historyPath := firstPath(
		paths.UpdateHistoryFile,
		filepath.Join(paths.HomeDir, compozyconfig.LogsDirName, compozyconfig.UpdateHistoryFileName),
	)
	if strings.TrimSpace(paths.HomeDir) == "" || operationPath == "" || lockPath == "" || historyPath == "" {
		return nil, errors.New("update: complete home paths are required for the operation store")
	}
	if events == nil {
		events = discardOperationEvents{}
	}
	return &OperationStore{
		operationPath: operationPath,
		lockPath:      lockPath,
		historyPath:   historyPath,
		now:           func() time.Time { return time.Now().UTC() },
		holderLive:    holderProcessIsLive,
		events:        events,
	}, nil
}

func holderProcessIsLive(holder *Holder, now time.Time) bool {
	return holder != nil && holder.LeaseExpiresAt.After(now) && procutil.Alive(holder.PID) &&
		procutil.MatchesStartTime(holder.PID, holder.PIDStartTime)
}

// HolderLive reports whether the lease still belongs to the recorded process identity.
func (s *OperationStore) HolderLive(holder *Holder) bool {
	return s != nil && s.holderLive != nil && s.holderLive(holder, s.now())
}

func firstPath(primary string, fallback string) string {
	if value := strings.TrimSpace(primary); value != "" {
		return value
	}
	return strings.TrimSpace(fallback)
}

// Read returns the validated live operation, or nil when no operation exists.
func (s *OperationStore) Read(ctx context.Context) (*Operation, error) {
	var operation *Operation
	err := s.withLock(ctx, func() error {
		var err error
		operation, err = s.readUnlocked()
		return err
	})
	return operation, err
}

// Acquire creates the sole operation or resumes an eligible dormant operation.
func (s *OperationStore) Acquire(ctx context.Context, request OperationRequest) (*Operation, error) {
	if err := validateOperationRequest(request); err != nil {
		return nil, err
	}
	var acquired *Operation
	err := s.withLock(ctx, func() error {
		existing, err := s.readUnlocked()
		if err != nil {
			return err
		}
		if existing != nil {
			if canResumeOperation(existing, request.Targets, s.now(), s.holderLive) {
				return s.resumeUnlocked(ctx, existing, request, &acquired)
			}
			blocked := *existing
			s.events.EmitUpdateEvent(ctx, eventFromOperation(
				EventOperationBlocked, &blocked, blocked.ActiveTarget, request.RequestedBy, "blocked", s.now(),
			))
			return &BlockedError{Operation: blocked}
		}

		now := s.now()
		operationID, err := randomOperationID()
		if err != nil {
			return err
		}
		operation := &Operation{
			SchemaVersion: OperationSchemaVersion,
			ID:            operationID,
			RequestedBy:   request.RequestedBy,
			Revision:      1,
			Targets:       append([]Target(nil), request.Targets...),
			ActiveTarget:  request.Targets[0],
			Percent:       -1,
			Runtime:       cloneRuntimeState(request.Runtime),
			App:           cloneAppState(request.App),
			Holder:        cloneHolder(&request.Holder),
			Deadline:      request.Deadline.UTC(),
			StartedAt:     now,
			UpdatedAt:     now,
		}
		if err := validateOperation(operation); err != nil {
			return err
		}
		if err := s.publishNewUnlocked(operation); err != nil {
			return err
		}
		s.events.EmitUpdateEvent(ctx, eventFromOperation(
			EventOperationAcquired, operation, operation.ActiveTarget, request.RequestedBy, "accepted", now,
		))
		acquired = cloneOperation(operation)
		return nil
	})
	return acquired, err
}

func validateOperationRequest(request OperationRequest) error {
	if !request.RequestedBy.valid() {
		return fmt.Errorf("update: invalid requester %q", request.RequestedBy)
	}
	if err := validateTargets(request.Targets); err != nil {
		return err
	}
	if request.Holder.PID <= 0 || request.Holder.PIDStartTime.IsZero() || !request.Holder.Surface.valid() ||
		strings.TrimSpace(request.Holder.ExecutorGeneration) == "" || request.Holder.LeaseExpiresAt.IsZero() {
		return errors.New("update: valid initial holder is required")
	}
	for _, target := range request.Targets {
		switch target {
		case TargetRuntime:
			if request.Runtime == nil || request.Runtime.Phase != PhasePending {
				return errors.New("update: runtime target requires a pending runtime identity")
			}
		case TargetApp:
			if request.App == nil || request.App.Phase != PhasePending {
				return errors.New("update: app target requires a pending app identity")
			}
		}
	}
	return nil
}

func (s *OperationStore) withLock(ctx context.Context, fn func() error) (returnErr error) {
	if err := os.MkdirAll(filepath.Dir(s.lockPath), operationDirMode); err != nil {
		return fmt.Errorf("update: create operation lock directory: %w", err)
	}
	fileLock := flock.New(s.lockPath, flock.SetPermissions(operationFileMode))
	locked, err := fileLock.TryLockContext(ctx, 25*time.Millisecond)
	if err != nil {
		closeErr := fileLock.Close()
		return errors.Join(fmt.Errorf("update: acquire operation lock: %w", err), closeErr)
	}
	if !locked {
		closeErr := fileLock.Close()
		return errors.Join(ErrOperationBlocked, closeErr)
	}
	defer func() {
		unlockErr := fileLock.Unlock()
		closeErr := fileLock.Close()
		returnErr = errors.Join(returnErr, unlockErr, closeErr)
	}()
	return fn()
}

func (s *OperationStore) readUnlocked() (operationResult *Operation, returnErr error) {
	file, err := os.Open(s.operationPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("update: open operation journal: %w", err)
	}
	defer func() {
		returnErr = errors.Join(returnErr, file.Close())
	}()
	decoder := json.NewDecoder(io.LimitReader(file, maxOperationBytes+1))
	decoder.DisallowUnknownFields()
	var operation Operation
	if err := decoder.Decode(&operation); err != nil {
		return nil, fmt.Errorf("update: decode operation journal: %w", err)
	}
	if err := validateOperation(&operation); err != nil {
		return nil, err
	}
	return &operation, nil
}

func randomOperationID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("update: generate operation id: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// NewExecutorGeneration returns a random lease fence for one executor process.
func NewExecutorGeneration() (string, error) { return randomOperationID() }

func cloneOperation(operation *Operation) *Operation {
	if operation == nil {
		return nil
	}
	cloned := *operation
	cloned.Targets = append([]Target(nil), operation.Targets...)
	cloned.Runtime = cloneRuntimeState(operation.Runtime)
	cloned.App = cloneAppState(operation.App)
	cloned.Holder = cloneHolder(operation.Holder)
	return &cloned
}

func cloneRuntimeState(state *RuntimeOperationState) *RuntimeOperationState {
	if state == nil {
		return nil
	}
	cloned := *state
	return &cloned
}

func cloneAppState(state *AppOperationState) *AppOperationState {
	if state == nil {
		return nil
	}
	cloned := *state
	return &cloned
}

func cloneHolder(holder *Holder) *Holder {
	if holder == nil {
		return nil
	}
	cloned := *holder
	return &cloned
}
