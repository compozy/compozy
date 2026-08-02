package contract

import apicontract "github.com/compozy/compozy/internal/api/contract"

// IssueSeverity classifies one extension-package validation issue.
type IssueSeverity = apicontract.IssueSeverity

const (
	// IssueSeverityError blocks extension-package use.
	IssueSeverityError = apicontract.IssueSeverityError
	// IssueSeverityWarning reports non-blocking authoring guidance.
	IssueSeverityWarning = apicontract.IssueSeverityWarning
)

// IssueSeverityValues returns the closed validation-severity set.
func IssueSeverityValues() []string {
	return apicontract.IssueSeverityValues()
}

// ValidationIssue is one positioned extension-package validation diagnostic.
type ValidationIssue = apicontract.ValidationIssue

// ConsentArea is one operator-facing permission summary derived from Host API methods.
type ConsentArea = apicontract.ConsentArea

// ExtensionManifestSummary is the authoring-facing subset of a validated manifest.
type ExtensionManifestSummary = apicontract.ExtensionManifestSummary

// ExtensionValidatePayload is the shared result for extension-package validation surfaces.
type ExtensionValidatePayload = apicontract.ExtensionValidatePayload
