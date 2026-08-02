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
	if err == nil {
		if err := validateStaticKitResources(dir, manifest); err != nil {
			issue, handled, issueErr := validationIssueForError(dir, err)
			if issueErr != nil {
				return nil, nil, issueErr
			}
			if handled {
				return nil, []ValidationIssue{issue}, nil
			}
			return nil, nil, err
		}
		return manifest, commandAmbiguityWarnings(manifest, extensionManifestPath(dir)), nil
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
	path := extensionManifestPath(dir)
	issue := ValidationIssue{
		Path:     path,
		Message:  err.Error(),
		Severity: IssueSeverityError,
	}
	var resourceErr *resourceValidationError
	if errors.As(err, &resourceErr) && resourceErr != nil {
		issue.Path = strings.TrimSpace(resourceErr.Path)
		err = resourceErr.Err
	}

	var parseErr toml.ParseError
	if errors.As(err, &parseErr) {
		issue.Line = parseErr.Position.Line
		data, readErr := os.ReadFile(issue.Path)
		if readErr != nil {
			return ValidationIssue{}, false, fmt.Errorf("extension: read invalid source %q: %w", issue.Path, readErr)
		}
		issue.Column = sourceColumn(data, parseErr.Position.Start)
		return issue, true, nil
	}
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		data, readErr := os.ReadFile(issue.Path)
		if readErr != nil {
			return ValidationIssue{}, false, fmt.Errorf("extension: read invalid source %q: %w", issue.Path, readErr)
		}
		issue.Line, issue.Column = sourcePosition(data, int(syntaxErr.Offset)-1)
		return issue, true, nil
	}
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		issue.Field = typeErr.Field
		data, readErr := os.ReadFile(issue.Path)
		if readErr != nil {
			return ValidationIssue{}, false, fmt.Errorf("extension: read invalid source %q: %w", issue.Path, readErr)
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

func extensionManifestPath(dir string) string {
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
