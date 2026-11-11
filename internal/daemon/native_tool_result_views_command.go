package daemon

import (
	"github.com/compozy/compozy/internal/api/contract"
)

type compactComposerCommand struct {
	ID                string                        `json:"id"`
	CanonicalToken    string                        `json:"canonical_token"`
	DisplayName       string                        `json:"display_name"`
	Lane              string                        `json:"lane"`
	Source            contract.SessionCommandSource `json:"source"`
	InputHint         string                        `json:"input_hint,omitempty"`
	Available         bool                          `json:"available"`
	UnavailableReason string                        `json:"unavailable_reason,omitempty"`
}

func projectCompactComposerCommand(p contract.SessionCommandPayload) compactComposerCommand {
	return compactComposerCommand{
		ID:                p.ID,
		CanonicalToken:    p.CanonicalToken,
		DisplayName:       p.DisplayName,
		Lane:              p.Lane,
		Source:            p.Source,
		InputHint:         p.InputHint,
		Available:         p.Available,
		UnavailableReason: p.UnavailableReason,
	}
}
