package update

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func (s *OperationStore) archiveAndRemoveUnlocked(operation *Operation) error {
	if err := os.MkdirAll(filepath.Dir(s.historyPath), operationDirMode); err != nil {
		return fmt.Errorf("update: create history directory: %w", err)
	}
	alreadyArchived, err := historyContainsOperation(s.historyPath, operation.ID)
	if err != nil {
		return err
	}
	if !alreadyArchived {
		if err := appendHistoryRecord(s.historyPath, operation); err != nil {
			return err
		}
	}
	if err := os.Remove(s.operationPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("update: remove terminal operation journal: %w", err)
	}
	return syncDirectory(filepath.Dir(s.operationPath))
}

func historyContainsOperation(path string, operationID string) (contains bool, returnErr error) {
	operation, err := readArchivedOperation(path, operationID)
	return operation != nil, err
}

// ReadArchived returns one terminal operation from the durable audit trail.
func (s *OperationStore) ReadArchived(ctx context.Context, operationID string) (*Operation, error) {
	var operation *Operation
	err := s.withLock(ctx, func() error {
		var err error
		operation, err = readArchivedOperation(s.historyPath, operationID)
		return err
	})
	return operation, err
}

func readArchivedOperation(path string, operationID string) (operationResult *Operation, returnErr error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("update: open update history: %w", err)
	}
	defer func() {
		returnErr = errors.Join(returnErr, file.Close())
	}()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), int(maxOperationBytes))
	for scanner.Scan() {
		var record Operation
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return nil, fmt.Errorf("update: decode update history: %w", err)
		}
		if record.ID == operationID {
			return cloneOperation(&record), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("update: scan update history: %w", err)
	}
	return nil, nil
}

func appendHistoryRecord(path string, operation *Operation) (returnErr error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, operationFileMode)
	if err != nil {
		return fmt.Errorf("update: open update history for append: %w", err)
	}
	defer func() {
		returnErr = errors.Join(returnErr, file.Close())
	}()
	if err := json.NewEncoder(file).Encode(operation); err != nil {
		return fmt.Errorf("update: append update history: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("update: sync update history: %w", err)
	}
	return nil
}
