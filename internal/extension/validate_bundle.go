package extensionpkg

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	apicontract "github.com/compozy/compozy/internal/api/contract"
	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
)

// IssueSeverity classifies one bundle validation issue.
type IssueSeverity = extensioncontract.IssueSeverity

const (
	// IssueSeverityError blocks bundle use.
	IssueSeverityError = extensioncontract.IssueSeverityError
	// IssueSeverityWarning reports non-blocking authoring guidance.
	IssueSeverityWarning = extensioncontract.IssueSeverityWarning
)

// ValidationIssue is one positioned bundle validation diagnostic.
type ValidationIssue = extensioncontract.ValidationIssue

// ValidationReport is the structured daemon-free result shared by CLI and native authoring surfaces.
type ValidationReport = apicontract.ExtensionValidatePayload

// ValidateBundle validates a built or resource-only bundle without executing extension code.
func ValidateBundle(dir string) (*Manifest, []ValidationIssue, error) {
	manifest, err := LoadManifest(dir)
	if err == nil {
		return manifest, commandAmbiguityWarnings(manifest, bundleManifestPath(dir)), nil
	}
	issue, handled, issueErr := validationIssueForError(dir, err)
	if issueErr != nil {
		return nil, nil, issueErr
	}
	if handled {
		return nil, []ValidationIssue{issue}, nil
	}
	return nil, nil, err
}

// ValidateBundleReport adds the permission-derived consent summary to ValidateBundle.
func ValidateBundleReport(dir string) (*ValidationReport, error) {
	manifest, issues, err := ValidateBundle(dir)
	if err != nil {
		return nil, err
	}
	report := &ValidationReport{
		Issues: issues,
	}
	if manifest == nil {
		report.ConsentAreas = []ConsentArea{}
		return report, nil
	}
	report.Manifest = &apicontract.ExtensionManifestSummary{
		Name:              manifest.Name,
		Version:           manifest.Version,
		Description:       manifest.Description,
		MinCompozyVersion: manifest.MinCompozyVersion,
		Provides:          append([]string{}, manifest.Capabilities.Provides...),
		Permissions:       append([]string{}, manifest.Permissions.Requires...),
	}
	consent, err := DeriveConsentAreas(manifest.Permissions.Requires)
	if err != nil {
		return nil, fmt.Errorf("extension: derive validation consent areas: %w", err)
	}
	report.ConsentAreas = consent
	if report.ConsentAreas == nil {
		report.ConsentAreas = []ConsentArea{}
	}
	return report, nil
}

func validationIssueForError(dir string, err error) (ValidationIssue, bool, error) {
	path := bundleManifestPath(dir)
	issue := ValidationIssue{
		Path:     path,
		Message:  err.Error(),
		Severity: IssueSeverityError,
	}

	var parseErr toml.ParseError
	if errors.As(err, &parseErr) {
		issue.Line = parseErr.Position.Line
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return ValidationIssue{}, false, fmt.Errorf("extension: read invalid manifest %q: %w", path, readErr)
		}
		issue.Column = sourceColumn(data, parseErr.Position.Start)
		return issue, true, nil
	}
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return ValidationIssue{}, false, fmt.Errorf("extension: read invalid manifest %q: %w", path, readErr)
		}
		issue.Line, issue.Column = sourcePosition(data, int(syntaxErr.Offset)-1)
		return issue, true, nil
	}
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		issue.Field = typeErr.Field
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return ValidationIssue{}, false, fmt.Errorf("extension: read invalid manifest %q: %w", path, readErr)
		}
		issue.Line, issue.Column = sourcePosition(data, int(typeErr.Offset)-1)
		return issue, true, nil
	}
	var validationErr *ManifestValidationError
	if errors.As(err, &validationErr) {
		issue.Field = validationErr.Field
		return issue, true, nil
	}
	var compatibilityErr *ManifestCompatibilityError
	if errors.As(err, &compatibilityErr) {
		issue.Field = manifestMinCompozyVersionKey
		return issue, true, nil
	}
	var notFoundErr *ManifestNotFoundError
	if errors.As(err, &notFoundErr) {
		issue.Path = strings.TrimSpace(notFoundErr.Dir)
		return issue, true, nil
	}
	return ValidationIssue{}, false, nil
}

func bundleManifestPath(dir string) string {
	root := strings.TrimSpace(dir)
	tomlPath := filepath.Join(root, manifestTOMLFileName)
	if info, err := os.Stat(tomlPath); err == nil && info.Mode().IsRegular() {
		return tomlPath
	}
	return filepath.Join(root, manifestJSONFileName)
}

func sourcePosition(data []byte, offset int) (int, int) {
	if offset < 0 {
		offset = 0
	}
	if offset > len(data) {
		offset = len(data)
	}
	line := bytes.Count(data[:offset], []byte{'\n'}) + 1
	return line, sourceColumn(data, offset)
}

func sourceColumn(data []byte, offset int) int {
	if offset < 0 {
		offset = 0
	}
	if offset > len(data) {
		offset = len(data)
	}
	lineStart := bytes.LastIndexByte(data[:offset], '\n') + 1
	return offset - lineStart + 1
}
