package extensionpkg

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
	apicontract "github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/extension/agentplugin"
	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
)

// IssueSeverity classifies one extension validation issue.
type IssueSeverity = extensioncontract.IssueSeverity

const (
	// IssueSeverityError blocks extension use.
	IssueSeverityError = extensioncontract.IssueSeverityError
	// IssueSeverityWarning reports non-blocking authoring guidance.
	IssueSeverityWarning = extensioncontract.IssueSeverityWarning
	// IssueSeverityWarn reports one non-blocking portable component skip.
	IssueSeverityWarn = extensioncontract.IssueSeverityWarn
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
	format, dualManifest, err := extensionValidationFormat(dir)
	if err != nil {
		if errors.Is(err, ErrAgentPluginManifestInvalid) {
			return invalidAgentPluginValidationReport(err), nil
		}
		return nil, err
	}
	if format == FormatAgentPlugin {
		return validateAgentPluginBundleReport(dir)
	}

	manifest, issues, err := ValidateBundle(dir)
	if err != nil {
		return nil, err
	}
	report := &ValidationReport{
		Status: configValidationStatus(issues), Format: string(FormatCompozy),
		Issues: issues, DualManifest: dualManifest,
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

func extensionValidationFormat(dir string) (ExtensionFormat, bool, error) {
	root := strings.TrimSpace(dir)
	pluginPath := filepath.Join(root, agentPluginManifestFileName)
	pluginInfo, pluginErr := os.Lstat(pluginPath)
	pluginExists := pluginErr == nil
	if pluginErr != nil && !errors.Is(pluginErr, os.ErrNotExist) {
		return "", false, fmt.Errorf("extension: inspect Agent Plugins manifest %q: %w", pluginPath, pluginErr)
	}
	for _, name := range []string{manifestTOMLFileName, manifestJSONFileName} {
		if exists, err := fileExists(filepath.Join(root, name)); err != nil {
			return "", false, err
		} else if exists {
			return FormatCompozy, pluginExists, nil
		}
	}
	if pluginExists && !pluginInfo.Mode().IsRegular() {
		_, loadErr := LoadManifest(root)
		return "", false, loadErr
	}
	status, _, err := agentplugin.ClassifyManifest(root)
	if err != nil {
		return "", false, err
	}
	if status == agentplugin.SchemaSupported {
		return FormatAgentPlugin, false, nil
	}
	if pluginExists || status == agentplugin.SchemaUnsupportedVersion || detectAgentPluginClientLayout(root) != "" {
		_, loadErr := LoadManifest(root)
		return "", false, loadErr
	}
	return FormatCompozy, false, nil
}

func validateAgentPluginBundleReport(dir string) (*ValidationReport, error) {
	root := strings.TrimSpace(dir)
	dataDir, err := defaultAgentPluginDataDir(root, filepath.Base(root))
	if err != nil {
		return nil, err
	}
	pkg, err := agentplugin.Load(root, agentplugin.LoadOptions{DataDir: dataDir})
	if err != nil {
		if manifestErr, ok := errors.AsType[*agentplugin.ManifestError](err); ok {
			return invalidAgentPluginValidationReport(
				newAgentPluginManifestValidationError(filepath.Join(root, agentPluginManifestFileName), manifestErr),
			), nil
		}
		return nil, fmt.Errorf("extension: validate Agent Plugins package %q: %w", root, err)
	}
	manifest, err := SynthesizeAgentPluginManifest(pkg, root)
	if err != nil {
		return nil, err
	}
	if err := validateStaticKitResources(context.Background(), root, manifest); err != nil {
		issues, validationErr := validationIssuesForError(root, err)
		if validationErr != nil {
			return nil, validationErr
		}
		return &ValidationReport{
			Status: agentPluginInvalidStatus, Format: string(FormatAgentPlugin), Name: pkg.Name, Version: pkg.Version,
			Issues: issues,
		}, nil
	}
	return &ValidationReport{
		Status:      "valid",
		Format:      string(FormatAgentPlugin),
		Name:        pkg.Name,
		Version:     pkg.Version,
		WouldIngest: agentPluginValidationComponents(root, pkg),
		Issues:      agentPluginValidationWarnings(pkg.Diagnostics),
	}, nil
}

func invalidAgentPluginValidationReport(err error) *ValidationReport {
	issues := []ValidationIssue{{
		Path: filepath.Base(agentPluginManifestFileName), Scope: agentPluginManifestFileName,
		Message: err.Error(), Severity: IssueSeverityError,
	}}
	if manifestErr, ok := errors.AsType[*AgentPluginManifestValidationError](err); ok && manifestErr != nil {
		issues = make([]ValidationIssue, 0, len(manifestErr.Issues))
		for _, issue := range manifestErr.Issues {
			issues = append(issues, ValidationIssue{
				Path: agentPluginManifestFileName, Scope: agentPluginManifestFileName,
				Field: strings.TrimSpace(issue.Path), Message: strings.TrimSpace(issue.Message),
				Severity: IssueSeverityError,
			})
		}
	}
	return &ValidationReport{Status: agentPluginInvalidStatus, Format: string(FormatAgentPlugin), Issues: issues}
}

func agentPluginValidationComponents(root string, pkg *agentplugin.Package) []apicontract.ExtensionValidationComponent {
	components := make([]apicontract.ExtensionValidationComponent, 0, len(pkg.Skills)+len(pkg.Servers))
	for _, skill := range pkg.Skills {
		detail, err := filepath.Rel(root, skill.Dir)
		if err != nil {
			detail = filepath.Join("skills", skill.Name)
		}
		components = append(components, apicontract.ExtensionValidationComponent{
			Kind: agentPluginSkillKind, Name: skill.Name, Detail: filepath.ToSlash(detail),
		})
	}
	for _, server := range pkg.Servers {
		components = append(components, apicontract.ExtensionValidationComponent{
			Kind: agentPluginMCPServerKind, Name: server.Name, Transport: server.Transport, Detail: server.Transport,
		})
	}
	slices.SortStableFunc(components, func(left, right apicontract.ExtensionValidationComponent) int {
		if left.Kind != right.Kind {
			if left.Kind == agentPluginSkillKind {
				return -1
			}
			return 1
		}
		return strings.Compare(left.Name, right.Name)
	})
	return components
}

func agentPluginValidationWarnings(values []agentplugin.Diagnostic) []ValidationIssue {
	issues := make([]ValidationIssue, 0, len(values))
	for _, item := range values {
		issues = append(issues, ValidationIssue{
			Scope: strings.TrimSpace(item.Scope), Message: strings.TrimSpace(item.Message), Severity: IssueSeverityWarn,
		})
	}
	return issues
}

func configValidationStatus(issues []ValidationIssue) string {
	for _, issue := range issues {
		if issue.Severity == IssueSeverityError {
			return agentPluginInvalidStatus
		}
	}
	return "valid"
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
