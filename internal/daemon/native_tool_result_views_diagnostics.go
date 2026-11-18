package daemon

import "github.com/compozy/compozy/internal/api/contract"

type compactPlatformDiagnostic struct {
	ID               string `json:"id"`
	Code             string `json:"code"`
	Severity         string `json:"severity"`
	Category         string `json:"category"`
	Title            string `json:"title"`
	Message          string `json:"message"`
	SuggestedCommand string `json:"suggested_command,omitempty"`
}

func compactPlatformDiagnostics(p []contract.DiagnosticItem) []compactPlatformDiagnostic {
	if p == nil {
		return nil
	}
	result := make([]compactPlatformDiagnostic, len(p))
	for i, d := range p {
		result[i] = compactPlatformDiagnostic{
			d.ID,
			d.Code,
			d.Severity,
			d.Category,
			d.Title,
			d.Message,
			d.SuggestedCommand,
		}
	}
	return result
}

// Keep trust decisions and actionable warnings without embedding diagnostic evidence.
type compactPlatformTrust struct {
	Decision         string                      `json:"decision"`
	RegistryTier     string                      `json:"registry_tier"`
	ChecksumVerified bool                        `json:"checksum_verified"`
	AllowUnverified  bool                        `json:"allow_unverified"`
	Warnings         []compactPlatformDiagnostic `json:"warnings,omitempty"`
}

func compactTrust(p *contract.ExtensionTrustReportPayload) *compactPlatformTrust {
	if p == nil {
		return nil
	}
	return &compactPlatformTrust{
		p.Decision,
		p.RegistryTier,
		p.ChecksumVerified,
		p.AllowUnverified,
		compactPlatformDiagnostics(p.Warnings),
	}
}
