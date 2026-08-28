package update

import (
	"errors"
	"fmt"
	"io"
	"os"

	goselfupdate "github.com/creativeprojects/go-selfupdate/update"
)

type selfBinaryApplier struct{}

// ApplyBinaryReplacement atomically replaces a runtime binary and keeps a rollback copy.
func ApplyBinaryReplacement(
	sourcePath string,
	targetPath string,
	backupPath string,
	mode os.FileMode,
) error {
	return (selfBinaryApplier{}).ApplyBinary(sourcePath, targetPath, backupPath, mode)
}

// RestoreBinaryReplacement restores a runtime binary from a rollback copy.
func RestoreBinaryReplacement(backupPath string, targetPath string, mode os.FileMode) error {
	return (selfBinaryApplier{}).RestoreBinary(backupPath, targetPath, mode)
}

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

	if mode == 0 {
		mode = 0o755
	}
	return applySelfUpdate(
		reader,
		sourcePath,
		"replacement binary",
		"apply binary",
		"apply binary rollback failed",
		goselfupdate.Options{
			TargetPath:  targetPath,
			OldSavePath: backupPath,
			TargetMode:  mode,
		},
	)
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

	if mode == 0 {
		mode = 0o755
	}
	return applySelfUpdate(
		reader,
		backupPath,
		"backup binary",
		"restore backup binary",
		"restore rollback failed",
		goselfupdate.Options{
			TargetPath: targetPath,
			TargetMode: mode,
		},
	)
}

func applySelfUpdate(
	source io.ReadCloser,
	sourcePath string,
	sourceKind string,
	action string,
	rollbackAction string,
	options goselfupdate.Options,
) (err error) {
	defer func() {
		if closeErr := source.Close(); closeErr != nil {
			err = errors.Join(
				err,
				fmt.Errorf("update: close %s %q: %w", sourceKind, sourcePath, closeErr),
			)
		}
	}()

	if applyErr := goselfupdate.Apply(source, options); applyErr != nil {
		return wrapSelfUpdateError(action, rollbackAction, applyErr)
	}
	return nil
}

func wrapSelfUpdateError(action string, rollbackAction string, err error) error {
	if rollbackErr := goselfupdate.RollbackError(err); rollbackErr != nil {
		return fmt.Errorf("update: %s: %w", rollbackAction, errors.Join(err, rollbackErr))
	}
	return fmt.Errorf("update: %s: %w", action, err)
}
