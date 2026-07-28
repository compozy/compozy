package e2e

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"strings"
)

func captureFileTargets(targetDir string, sourcePaths []string) ([]captureFileTarget, error) {
	usedNames := make(map[string]struct{}, len(sourcePaths))
	targets := make([]captureFileTarget, 0, len(sourcePaths))
	for index, sourcePath := range sourcePaths {
		name, err := uniqueCaptureFileName(targetDir, sourcePath, index, usedNames)
		if err != nil {
			return nil, err
		}
		targets = append(targets, captureFileTarget{
			sourcePath: sourcePath,
			targetPath: filepath.Join(targetDir, name),
		})
	}
	return targets, nil
}

func uniqueCaptureFileName(
	targetDir string,
	sourcePath string,
	sourceIndex int,
	usedNames map[string]struct{},
) (string, error) {
	baseName := filepath.Base(sourcePath)
	if baseName == "." || baseName == string(filepath.Separator) {
		return "", fmt.Errorf("source %q does not have a file name", sourcePath)
	}
	for attempt := 0; ; attempt++ {
		candidate := baseName
		if attempt == 1 {
			candidate = fmt.Sprintf("%03d-%s", sourceIndex+1, baseName)
		}
		if attempt > 1 {
			candidate = fmt.Sprintf("%03d-%d-%s", sourceIndex+1, attempt, baseName)
		}
		key := strings.ToLower(candidate)
		if _, ok := usedNames[key]; ok {
			continue
		}
		exists, err := fileExists(filepath.Join(targetDir, candidate))
		if err != nil {
			return "", err
		}
		if exists {
			continue
		}
		usedNames[key] = struct{}{}
		return candidate, nil
	}
}

func fileExists(path string) (bool, error) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func copyFile(sourcePath string, targetPath string) (err error) {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := source.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}

	target, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := target.Close()
		if err != nil {
			if removeErr := os.Remove(targetPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				err = errors.Join(err, fmt.Errorf("remove partial artifact %q: %w", targetPath, removeErr))
			}
		}
		if closeErr != nil {
			if err == nil {
				err = closeErr
			} else {
				err = errors.Join(err, closeErr)
			}
		}
	}()

	if _, err := io.Copy(target, source); err != nil {
		return err
	}
	return nil
}

func sanitizePathComponent(value string) string {
	clean := strings.TrimSpace(strings.ToLower(value))
	if clean == "" {
		return defaultArtifactSlug
	}

	var builder strings.Builder
	builder.Grow(len(clean))
	lastDash := false
	for _, r := range clean {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		default:
			if lastDash || builder.Len() == 0 {
				continue
			}
			builder.WriteByte('-')
			lastDash = true
		}
	}

	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return defaultArtifactSlug
	}
	return result
}

func findToolHostOperation(
	operations []ToolHostOperationDiagnostic,
	operation string,
	outcome ToolHostOperationOutcome,
	requireError bool,
) (ToolHostOperationDiagnostic, bool) {
	wantOperation := strings.TrimSpace(operation)
	for _, item := range operations {
		if strings.TrimSpace(item.Operation) != wantOperation {
			continue
		}
		if item.Outcome != outcome {
			continue
		}
		if requireError && strings.TrimSpace(item.Error) == "" {
			continue
		}
		if !requireError && strings.TrimSpace(item.Error) != "" {
			continue
		}
		return item, true
	}
	return ToolHostOperationDiagnostic{}, false
}
