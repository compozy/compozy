package toolruntime

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
)

// MemoryStore is an in-memory Store implementation used by tests and fakes.
type MemoryStore struct {
	mu      sync.RWMutex
	records map[string]ProcessRecord
}

var _ Store = (*MemoryStore)(nil)

// NewMemoryStore constructs an empty in-memory process store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{records: make(map[string]ProcessRecord)}
}

// UpsertProcessRecord inserts or replaces a record.
func (s *MemoryStore) UpsertProcessRecord(_ context.Context, record ProcessRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.records == nil {
		s.records = make(map[string]ProcessRecord)
	}
	s.records[record.ID] = cloneRecord(record)
	return nil
}

// UpdateProcessRecordState updates lifecycle fields for an existing record.
func (s *MemoryStore) UpdateProcessRecordState(_ context.Context, update ProcessStateUpdate) error {
	id := strings.TrimSpace(update.ID)
	if id == "" {
		return errors.New("toolruntime: process id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.records == nil {
		return fmt.Errorf("%w: process record %q", ErrProcessNotFound, id)
	}
	record, ok := s.records[id]
	if !ok {
		return fmt.Errorf("%w: process record %q", ErrProcessNotFound, id)
	}
	record.ID = id
	record.State = update.State
	record.ExitCode = update.ExitCode
	record.Error = update.Error
	record.UpdatedAt = update.UpdatedAt
	record.CompletedAt = update.CompletedAt
	s.records[id] = cloneRecord(record)
	return nil
}

// ListProcessRecords returns records matching the query.
func (s *MemoryStore) ListProcessRecords(_ context.Context, query ProcessQuery) ([]ProcessRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	records := make([]ProcessRecord, 0, len(s.records))
	for _, record := range s.records {
		if queryMatches(record, query) {
			records = append(records, cloneRecord(record))
		}
	}
	slices.SortFunc(records, func(left, right ProcessRecord) int {
		if left.UpdatedAt.Equal(right.UpdatedAt) {
			return stringsCompare(left.ID, right.ID)
		}
		if left.UpdatedAt.Before(right.UpdatedAt) {
			return -1
		}
		return 1
	})
	if query.Limit > 0 && len(records) > query.Limit {
		records = records[:query.Limit]
	}
	return records, nil
}

func queryMatches(record ProcessRecord, query ProcessQuery) bool {
	if len(query.IDs) > 0 && !slices.Contains(query.IDs, record.ID) {
		return false
	}
	if len(query.States) > 0 && !slices.Contains(query.States, record.State) {
		return false
	}
	scope := query.Scope.Normalize()
	if !scope.IsZero() && !matchesScope(record, scope) {
		return false
	}
	return true
}

func cloneRecord(record ProcessRecord) ProcessRecord {
	cloned := record
	cloned.Args = append([]string(nil), record.Args...)
	if record.ExitCode != nil {
		value := *record.ExitCode
		cloned.ExitCode = &value
	}
	if record.CompletedAt != nil {
		value := *record.CompletedAt
		cloned.CompletedAt = &value
	}
	return cloned
}

func stringsCompare(left string, right string) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}
