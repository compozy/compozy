package extensionpkg

import (
	"bytes"
	"context"
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

// IssueSeverity classifies one extension validation issue.
type IssueSeverity = extensioncontract.IssueSeverity

const (
	// IssueSeverityError blocks extension use.
	IssueSeverityError = extensioncontract.IssueSeverityError
	// IssueSeverityWarning reports non-blocking authoring guidance.
	IssueSeverityWarning = extensioncontract.IssueSeverityWarning
)

// ValidationIssue is one positioned extension validation diagnostic.
type ValidationIssue = extensioncontract.ValidationIssue

// ValidationReport is the structured daemon-free result shared by CLI and native authoring surfaces.
type ValidationReport = apicontract.ExtensionValidatePayload

// ValidateBundle validates a built or resource-only extension archive without executing extension code.
func ValidateBundle(dir string) (*Manifest, []ValidationIssue, error) {
	manifest, err := LoadManifest(dir)
	if err != nil {
		issues, validationErr := validationIssuesForError(dir, err)
		return nil, issues, validationErr
	}
	if err := validateStaticKitResources(context.Background(), dir, manifest); err != nil {
		issues, validationErr := validationIssuesForError(dir, err)
		return nil, issues, validationErr
	}
	return manifest, commandAmbiguityWarnings(manifest, extensionManifestPath(dir)), nil
}

func validationIssuesForError(dir string, err error) ([]ValidationIssue, error) {
	issue, handled, issueErr := validationIssueForError(dir, err)
	if issueErr != nil {
		return nil, issueErr
	}
	if handled {
		return []ValidationIssue{issue}, nil
	}
	return nil, err
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
	path := extensionManifestPath(dir)
	issue := ValidationIssue{
		Path:     path,
		Message:  err.Error(),
		Severity: IssueSeverityError,
	}
	if resourceErr, ok := errors.AsType[*resourceValidationError](err); ok && resourceErr != nil {
		issue.Path = strings.TrimSpace(resourceErr.Path)
		err = resourceErr.Err
	}

	if parseErr, ok := errors.AsType[toml.ParseError](err); ok {
		issue.Line = parseErr.Position.Line
		data, readErr := os.ReadFile(issue.Path)
		if readErr != nil {
			return ValidationIssue{}, false, fmt.Errorf("extension: read invalid source %q: %w", issue.Path, readErr)
		}
		issue.Column = sourceColumn(data, parseErr.Position.Start)
		return issue, true, nil
	}
	if syntaxErr, ok := errors.AsType[*json.SyntaxError](err); ok {
		data, readErr := os.ReadFile(issue.Path)
		if readErr != nil {
			return ValidationIssue{}, false, fmt.Errorf("extension: read invalid source %q: %w", issue.Path, readErr)
		}
		issue.Line, issue.Column = sourcePositionFromJSONOffset(data, syntaxErr.Offset)
		return issue, true, nil
	}
	if typeErr, ok := errors.AsType[*json.UnmarshalTypeError](err); ok {
		issue.Field = typeErr.Field
		data, readErr := os.ReadFile(issue.Path)
		if readErr != nil {
			return ValidationIssue{}, false, fmt.Errorf("extension: read invalid source %q: %w", issue.Path, readErr)
		}
		issue.Line, issue.Column = sourcePositionFromJSONOffset(data, typeErr.Offset)
		return issue, true, nil
	}
	if validationErr, ok := errors.AsType[*ManifestValidationError](err); ok {
		issue.Field = validationErr.Field
		return issue, true, nil
	}
	var compatibilityErr *ManifestCompatibilityError
	if errors.As(err, &compatibilityErr) {
		issue.Field = manifestMinCompozyVersionKey
		return issue, true, nil
	}
	if notFoundErr, ok := errors.AsType[*ManifestNotFoundError](err); ok {
		issue.Path = strings.TrimSpace(notFoundErr.Dir)
		return issue, true, nil
	}
	return ValidationIssue{}, false, nil
}

func extensionManifestPath(dir string) string {
	root := strings.TrimSpace(dir)
	tomlPath := filepath.Join(root, manifestTOMLFileName)
	if info, err := os.Stat(tomlPath); err == nil && info.Mode().IsRegular() {
		return tomlPath
	}
	jsonPath := filepath.Join(root, manifestJSONFileName)
	if info, err := os.Stat(jsonPath); err == nil && info.Mode().IsRegular() {
		return jsonPath
	}
	return filepath.Join(root, agentPluginManifestFileName)
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

func sourcePositionFromJSONOffset(data []byte, offset int64) (int, int) {
	if offset <= 1 {
		return sourcePosition(data, 0)
	}
	byteOffset := min(offset-1, int64(len(data)))
	return sourcePosition(data, int(byteOffset))
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
