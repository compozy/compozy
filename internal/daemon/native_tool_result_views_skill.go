package daemon

import (
	"github.com/compozy/compozy/internal/api/contract"
)

type compactSkillCatalogEntry struct {
	Name        string                            `json:"name"`
	Version     string                            `json:"version,omitempty"`
	Source      string                            `json:"source"`
	Origin      string                            `json:"origin"`
	OwnerScope  string                            `json:"owner_scope"`
	OwnerID     string                            `json:"owner_id,omitempty"`
	Enabled     bool                              `json:"enabled"`
	Activation  contract.SkillActivationPayload   `json:"activation"`
	Diagnostics []contract.SkillDiagnosticPayload `json:"diagnostics,omitempty"`
	Exposures   *[]contract.SkillExposurePayload  `json:"exposures,omitzero"`
}

func projectCompactSkillCatalogEntry(p contract.SkillPayload) compactSkillCatalogEntry {
	return compactSkillCatalogEntry{
		Name:        p.Name,
		Version:     p.Version,
		Source:      p.Source,
		Origin:      p.Origin,
		OwnerScope:  p.OwnerScope,
		OwnerID:     p.OwnerID,
		Enabled:     p.Enabled,
		Activation:  p.Activation,
		Diagnostics: compactSkillDiagnostics(p.Diagnostics),
		Exposures:   p.Exposures,
	}
}

func compactSkillDiagnostics(diagnostics []contract.SkillDiagnosticPayload) []contract.SkillDiagnosticPayload {
	for _, diagnostic := range diagnostics {
		if diagnostic.State != contract.SkillDiagnosticStateValid ||
			diagnostic.VerificationStatus != contract.SkillVerificationStatusPassed ||
			len(diagnostic.Warnings) > 0 || diagnostic.Failure != nil || len(diagnostic.ActivationReasons) > 0 {
			return diagnostics
		}
	}
	return nil
}
