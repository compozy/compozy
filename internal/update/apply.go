package update

import (
	"errors"
	"fmt"
	"os"

	goselfupdate "github.com/creativeprojects/go-selfupdate/update"
)

type selfBinaryApplier struct{}

func (selfBinaryApplier) ApplyBinary(
	sourcePath string,
	targetPath string,
	backupPath string,
	mode os.FileMode,
) error {
	reader, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("update: open replacement binary %q: %w", sourcePath, err)
	}
	defer func() {
		_ = reader.Close()
	}()

	if mode == 0 {
		mode = 0o755
	}
	if err := goselfupdate.Apply(reader, goselfupdate.Options{
		TargetPath:  targetPath,
		OldSavePath: backupPath,
		TargetMode:  mode,
	}); err != nil {
		return wrapSelfUpdateError("apply binary", "apply binary rollback failed", err)
	}
	return nil
}

func (selfBinaryApplier) RestoreBinary(
	backupPath string,
	targetPath string,
	mode os.FileMode,
) error {
	reader, err := os.Open(backupPath)
	if err != nil {
		return fmt.Errorf("update: open backup binary %q: %w", backupPath, err)
	}
	defer func() {
		_ = reader.Close()
	}()

	if mode == 0 {
		mode = 0o755
	}
	if err := goselfupdate.Apply(reader, goselfupdate.Options{
		TargetPath: targetPath,
		TargetMode: mode,
	}); err != nil {
		return wrapSelfUpdateError("restore backup binary", "restore rollback failed", err)
	}
	return nil
}

func wrapSelfUpdateError(action string, rollbackAction string, err error) error {
	if rollbackErr := goselfupdate.RollbackError(err); rollbackErr != nil {
		return fmt.Errorf("update: %s: %w", rollbackAction, errors.Join(err, rollbackErr))
	}
	return fmt.Errorf("update: %s: %w", action, err)
}
