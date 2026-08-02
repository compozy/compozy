package support

import (
	"errors"
	"os"
	"strings"
	"sync"
	"time"
)

type operationStore struct {
	mu         sync.RWMutex
	now        func() time.Time
	removeFile func(string) error
	ops        map[string]Operation
}

type expiredOperationArtifact struct {
	operationID string
	filePath    string
	completedAt time.Time
}

type expiredArtifactCleanupFailure struct {
	operationID string
	cause       error
}

func (f expiredArtifactCleanupFailure) Error() string {
	return "support: remove expired bundle artifact"
}

func (f expiredArtifactCleanupFailure) Unwrap() error {
	return f.cause
}

func newOperationStore(now func() time.Time) *operationStore {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &operationStore{
		now:        now,
		removeFile: os.Remove,
		ops:        make(map[string]Operation),
	}
}

func (s *operationStore) create(operationID string) Operation {
	now := s.now().UTC()
	op := Operation{OperationID: operationID, Status: OperationPending, CreatedAt: now, UpdatedAt: now}
	s.mu.Lock()
	s.ops[operationID] = op
	s.mu.Unlock()
	return op
}

func (s *operationStore) get(operationID string) (Operation, error) {
	s.mu.RLock()
	op, ok := s.ops[strings.TrimSpace(operationID)]
	s.mu.RUnlock()
	if !ok {
		return Operation{}, ErrOperationNotFound
	}
	return cloneOperation(op), nil
}

func (s *operationStore) markRunning(operationID string) {
	s.update(operationID, func(op *Operation, now time.Time) { op.Status = OperationRunning; op.UpdatedAt = now })
}

func (s *operationStore) markCompleted(operationID string, result Operation) {
	s.update(operationID, func(op *Operation, _ time.Time) { *op = cloneOperation(result) })
}

func (s *operationStore) markFailed(operationID string, reason string) {
	s.update(operationID, func(op *Operation, now time.Time) {
		op.Status = OperationFailed
		op.FailureReason = strings.TrimSpace(reason)
		op.UpdatedAt = now
		op.CompletedAt = &now
	})
}

func (s *operationStore) update(operationID string, fn func(*Operation, time.Time)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	op, ok := s.ops[strings.TrimSpace(operationID)]
	if !ok {
		return
	}
	now := s.now().UTC()
	fn(&op, now)
	s.ops[operationID] = op
}

func (s *operationStore) cleanup(retention time.Duration) []expiredArtifactCleanupFailure {
	if retention <= 0 {
		return nil
	}
	cutoff := s.now().UTC().Add(-retention)
	candidates := s.expiredArtifacts(cutoff)
	failures := make([]expiredArtifactCleanupFailure, 0)
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.filePath) != "" {
			if err := s.removeFile(candidate.filePath); err != nil && !errors.Is(err, os.ErrNotExist) {
				failures = append(failures, expiredArtifactCleanupFailure{
					operationID: candidate.operationID,
					cause:       err,
				})
				continue
			}
		}
		s.deleteIfUnchanged(candidate, cutoff)
	}
	return failures
}

func (s *operationStore) expiredArtifacts(cutoff time.Time) []expiredOperationArtifact {
	s.mu.RLock()
	defer s.mu.RUnlock()

	artifacts := make([]expiredOperationArtifact, 0)
	for id, op := range s.ops {
		if op.CompletedAt == nil || op.CompletedAt.After(cutoff) {
			continue
		}
		artifacts = append(artifacts, expiredOperationArtifact{
			operationID: id,
			filePath:    op.FilePath,
			completedAt: *op.CompletedAt,
		})
	}
	return artifacts
}

func (s *operationStore) deleteIfUnchanged(candidate expiredOperationArtifact, cutoff time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[candidate.operationID]
	if !ok || op.CompletedAt == nil || op.CompletedAt.After(cutoff) {
		return
	}
	if op.FilePath != candidate.filePath || !op.CompletedAt.Equal(candidate.completedAt) {
		return
	}
	delete(s.ops, candidate.operationID)
}

func cloneOperation(op Operation) Operation {
	if op.Manifest != nil {
		manifest := *op.Manifest
		manifest.Artifacts = append([]ManifestArtifact(nil), op.Manifest.Artifacts...)
		op.Manifest = &manifest
	}
	if op.CompletedAt != nil {
		completed := *op.CompletedAt
		op.CompletedAt = &completed
	}
	return op
}
