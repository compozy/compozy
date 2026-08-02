package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
)

const (
	// DotEnvStatusMissing reports that no .env file exists at the requested path.
	DotEnvStatusMissing = "missing"
	// DotEnvStatusValid reports that the .env file is structured and needs no repair.
	DotEnvStatusValid = "valid"
	// DotEnvStatusRepairable reports that the .env file can be repaired explicitly.
	DotEnvStatusRepairable = "repairable"
	// DotEnvStatusRepaired reports that the .env file was safely rewritten.
	DotEnvStatusRepaired = "repaired"
	// DotEnvStatusUnsupported reports that Compozy found content it will not rewrite.
	DotEnvStatusUnsupported = "unsupported"
)

const (
	dotEnvDiagnosticMultiKeyLine        = "multi_key_line"
	dotEnvDiagnosticSanitizedSecret     = "sanitized_secret_value"
	dotEnvDiagnosticUnsupportedLine     = "unsupported_line"
	dotEnvDiagnosticInvalidKey          = "invalid_key"
	dotEnvDiagnosticUnsupportedSymlink  = "unsupported_symlink"
	dotEnvDiagnosticUnsupportedDir      = "unsupported_directory"
	dotEnvDiagnosticUnsupportedFragment = "unsupported_fragment"
)

var (
	// ErrDotEnvUnsupported reports that .env content could not be safely parsed
	// or repaired without risking user-owned intent.
	ErrDotEnvUnsupported = errors.New("config: unsupported .env content")
)

// DotEnvDiagnostic describes one .env parse or repair issue without exposing values.
type DotEnvDiagnostic struct {
	Line    int    `json:"line,omitempty"`
	Key     string `json:"key,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// DotEnvRepairReport summarizes .env inspection or repair without including values.
type DotEnvRepairReport struct {
	Path        string             `json:"path"`
	Status      string             `json:"status"`
	Repaired    bool               `json:"repaired"`
	Diagnostics []DotEnvDiagnostic `json:"diagnostics,omitempty"`
}

// DotEnvRepairError carries structured diagnostics for unsupported .env content.
type DotEnvRepairError struct {
	Path        string
	Diagnostics []DotEnvDiagnostic
}

type dotEnvParseResult struct {
	values       map[string]string
	lines        []string
	diagnostics  []DotEnvDiagnostic
	needsRepair  bool
	unsupported  bool
	finalNewline bool
}

// WorkspaceDotEnvFile returns the .env file path for a resolved workspace root.
func WorkspaceDotEnvFile(workspaceRoot string) string {
	return filepath.Join(strings.TrimSpace(workspaceRoot), ".env")
}

// InspectDotEnvFile parses one .env file and reports whether explicit repair is possible.
func InspectDotEnvFile(path string) (DotEnvRepairReport, error) {
	normalizedPath, data, exists, err := readDotEnvFile(path)
	if err != nil {
		if repairErr, ok := errors.AsType[*DotEnvRepairError](err); ok {
			return DotEnvRepairReport{
				Path:        normalizedPath,
				Status:      DotEnvStatusUnsupported,
				Diagnostics: append([]DotEnvDiagnostic(nil), repairErr.Diagnostics...),
			}, err
		}
		return DotEnvRepairReport{Path: normalizedPath, Status: DotEnvStatusMissing}, err
	}
	if !exists {
		return DotEnvRepairReport{Path: normalizedPath, Status: DotEnvStatusMissing}, nil
	}

	parsed := parseDotEnvDocument(string(data))
	return dotEnvReport(normalizedPath, parsed, false), nil
}

// RepairDotEnvFile safely rewrites one .env file when every change is bounded and structured.
func RepairDotEnvFile(path string) (report DotEnvRepairReport, err error) {
	normalizedPath := strings.TrimSpace(path)
	if normalizedPath == "" {
		return DotEnvRepairReport{Path: normalizedPath, Status: DotEnvStatusMissing},
			errors.New("config: .env path is required")
	}

	directory, name, err := fileutil.OpenParentDirectory(normalizedPath)
	if err != nil {
		return dotEnvRepairReadFailure(normalizedPath, err)
	}
	defer func() {
		if closeErr := directory.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close .env parent directory %q: %w", normalizedPath, closeErr))
		}
	}()
	return repairDotEnvFileInDirectory(directory, name, normalizedPath)
}

func repairDotEnvFileInDirectory(
	directory *fileutil.Directory,
	name string,
	normalizedPath string,
) (DotEnvRepairReport, error) {
	data, info, err := directory.ReadRegularFile(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DotEnvRepairReport{Path: normalizedPath, Status: DotEnvStatusMissing}, nil
		}
		return dotEnvRepairReadFailure(normalizedPath, err)
	}
	mode := info.Mode().Perm()

	parsed := parseDotEnvDocument(string(data))
	report := dotEnvReport(normalizedPath, parsed, false)
	if parsed.unsupported {
		return report, dotEnvUnsupportedError(normalizedPath, parsed.diagnostics)
	}
	if !parsed.needsRepair {
		return report, nil
	}

	repaired := strings.Join(parsed.lines, "\n")
	if parsed.finalNewline {
		repaired += "\n"
	}
	if err := replaceDotEnvFile(directory, name, normalizedPath, []byte(repaired), mode); err != nil {
		return report, err
	}

	report = dotEnvReport(normalizedPath, parsed, true)
	return report, nil
}

func dotEnvRepairReadFailure(normalizedPath string, err error) (DotEnvRepairReport, error) {
	if errors.Is(err, os.ErrNotExist) {
		return DotEnvRepairReport{Path: normalizedPath, Status: DotEnvStatusMissing}, nil
	}
	if errors.Is(err, fileutil.ErrSymlink) {
		report := DotEnvRepairReport{
			Path:   normalizedPath,
			Status: DotEnvStatusUnsupported,
			Diagnostics: []DotEnvDiagnostic{{
				Code:    dotEnvDiagnosticUnsupportedSymlink,
				Message: ".env repair refuses to rewrite symlinks",
			}},
		}
		return report, dotEnvUnsupportedError(normalizedPath, report.Diagnostics)
	}
	if errors.Is(err, fileutil.ErrDirectory) {
		report := DotEnvRepairReport{
			Path:   normalizedPath,
			Status: DotEnvStatusUnsupported,
			Diagnostics: []DotEnvDiagnostic{{
				Code:    dotEnvDiagnosticUnsupportedDir,
				Message: ".env path is a directory",
			}},
		}
		return report, dotEnvUnsupportedError(normalizedPath, report.Diagnostics)
	}
	if errors.Is(err, fileutil.ErrNotRegular) {
		return DotEnvRepairReport{Path: normalizedPath, Status: DotEnvStatusMissing}, fmt.Errorf(
			".env file %q must be a regular file",
			normalizedPath,
		)
	}
	return DotEnvRepairReport{Path: normalizedPath, Status: DotEnvStatusMissing}, fmt.Errorf(
		"read .env file %q: %w",
		normalizedPath,
		err,
	)
}

func readDotEnvFile(path string) (string, []byte, bool, error) {
	normalizedPath, data, _, exists, err := readDotEnvRegularFile(path)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return normalizedPath, data, exists, nil
	}
	if errors.Is(err, fileutil.ErrSymlink) {
		return normalizedPath, nil, false, dotEnvUnsupportedError(normalizedPath, []DotEnvDiagnostic{{
			Code:    dotEnvDiagnosticUnsupportedSymlink,
			Message: ".env load refuses to read symlinks",
		}})
	}
	if errors.Is(err, fileutil.ErrDirectory) {
		return normalizedPath, nil, false, dotEnvUnsupportedError(normalizedPath, []DotEnvDiagnostic{{
			Code:    dotEnvDiagnosticUnsupportedDir,
			Message: ".env path is a directory",
		}})
	}
	if errors.Is(err, fileutil.ErrNotRegular) {
		return normalizedPath, nil, false, fmt.Errorf(".env file %q must be a regular file", normalizedPath)
	}
	return normalizedPath, nil, false, fmt.Errorf("read .env file %q: %w", normalizedPath, err)
}

func readDotEnvRegularFile(path string) (string, []byte, os.FileMode, bool, error) {
	normalizedPath := strings.TrimSpace(path)
	if normalizedPath == "" {
		return "", nil, 0, false, errors.New("config: .env path is required")
	}

	data, info, err := fileutil.ReadRegularFile(normalizedPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return normalizedPath, nil, 0, false, nil
		}
		return normalizedPath, nil, 0, false, err
	}
	return normalizedPath, data, info.Mode().Perm(), true, nil
}
