package update

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func (s *OperationStore) publishNewUnlocked(operation *Operation) (returnErr error) {
	tempPath, err := writeOperationTemp(s.operationPath, operation)
	if err != nil {
		return err
	}
	defer func() {
		if removeErr := os.Remove(tempPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			returnErr = errors.Join(returnErr, fmt.Errorf("update: remove operation temp file: %w", removeErr))
		}
	}()
	if err := os.Link(tempPath, s.operationPath); err != nil {
		if errors.Is(err, os.ErrExist) {
			return ErrOperationBlocked
		}
		return fmt.Errorf("update: publish new operation journal: %w", err)
	}
	return syncDirectory(filepath.Dir(s.operationPath))
}

func (s *OperationStore) replaceUnlocked(operation *Operation) error {
	if err := validateOperation(operation); err != nil {
		return err
	}
	tempPath, err := writeOperationTemp(s.operationPath, operation)
	if err != nil {
		return err
	}
	if err := os.Rename(tempPath, s.operationPath); err != nil {
		removeErr := os.Remove(tempPath)
		return errors.Join(fmt.Errorf("update: replace operation journal: %w", err), removeErr)
	}
	return syncDirectory(filepath.Dir(s.operationPath))
}

func writeOperationTemp(targetPath string, operation *Operation) (path string, returnErr error) {
	if err := os.MkdirAll(filepath.Dir(targetPath), operationDirMode); err != nil {
		return "", fmt.Errorf("update: create operation directory: %w", err)
	}
	file, err := os.CreateTemp(filepath.Dir(targetPath), ".update-operation-*.tmp")
	if err != nil {
		return "", fmt.Errorf("update: create operation temp file: %w", err)
	}
	path = file.Name()
	defer func() {
		closeErr := file.Close()
		if returnErr != nil {
			removeErr := os.Remove(path)
			returnErr = errors.Join(returnErr, closeErr, removeErr)
			return
		}
		returnErr = errors.Join(returnErr, closeErr)
	}()
	if err := file.Chmod(operationFileMode); err != nil {
		return path, fmt.Errorf("update: secure operation temp file: %w", err)
	}
	encoder := json.NewEncoder(file)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(operation); err != nil {
		return path, fmt.Errorf("update: encode operation journal: %w", err)
	}
	if err := file.Sync(); err != nil {
		return path, fmt.Errorf("update: sync operation journal: %w", err)
	}
	return path, nil
}

func syncDirectory(path string) (returnErr error) {
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("update: open journal directory for sync: %w", err)
	}
	defer func() {
		returnErr = errors.Join(returnErr, directory.Close())
	}()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("update: sync journal directory: %w", err)
	}
	return nil
}
