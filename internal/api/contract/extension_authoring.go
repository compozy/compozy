package contract

// IssueSeverity classifies one extension-package validation issue.
type IssueSeverity string

const (
	// IssueSeverityError blocks extension-package use.
	IssueSeverityError IssueSeverity = "error"
	// IssueSeverityWarning reports non-blocking authoring guidance.
	IssueSeverityWarning IssueSeverity = "warning"
)

// IssueSeverityValues returns the closed validation-severity set.
func IssueSeverityValues() []string {
	return []string{string(IssueSeverityError), string(IssueSeverityWarning)}
}

// ValidationIssue is one positioned extension-package validation diagnostic.
type ValidationIssue struct {
	Path     string        `json:"path"`
	Line     int           `json:"line,omitempty"`
	Column   int           `json:"column,omitempty"`
	Field    string        `json:"field,omitempty"`
	Message  string        `json:"message"`
	Severity IssueSeverity `json:"severity"`
}

// ConsentArea is one operator-facing permission summary derived from Host API methods.
type ConsentArea struct {
	Area   string `json:"area"`
	Access string `json:"access"`
}

// ExtensionManifestSummary is the stable authoring-facing subset of a validated manifest.
type ExtensionManifestSummary struct {
	Name              string   `json:"name"`
	Version           string   `json:"version"`
	Description       string   `json:"description,omitempty"`
	MinCompozyVersion string   `json:"min_compozy_version"`
	Provides          []string `json:"provides"`
	Permissions       []string `json:"permissions"`
}

// ExtensionValidatePayload is the shared result for extension-package validation surfaces.
type ExtensionValidatePayload struct {
	Manifest     *ExtensionManifestSummary `json:"manifest,omitempty"`
	Issues       []ValidationIssue         `json:"issues"`
	ConsentAreas []ConsentArea             `json:"consent_areas"`
}
