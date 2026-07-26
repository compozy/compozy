package support

import (
	"errors"

	"os"

	"strings"
	"sync"
	"time"
)

type operationStore struct {
	mu  sync.RWMutex
	now func() time.Time
	ops map[string]Operation
}

func newOperationStore(now func() time.Time) *operationStore {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &operationStore{now: now, ops: make(map[string]Operation)}
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

func (s *operationStore) cleanup(retention time.Duration) {
	if retention <= 0 {
		return
	}
	cutoff := s.now().UTC().Add(-retention)
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, op := range s.ops {
		if op.CompletedAt == nil || op.CompletedAt.After(cutoff) {
			continue
		}
		if strings.TrimSpace(op.FilePath) != "" {
			if removeErr := os.Remove(op.FilePath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				continue
			}
		}
		delete(s.ops, id)
	}
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
