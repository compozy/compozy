package contract

import apicontract "github.com/compozy/compozy/internal/api/contract"

// SkillSummary is the lightweight host-visible skill listing shape.
type SkillSummary struct {
	Name        string                             `json:"name"`
	Description string                             `json:"description,omitempty"`
	Source      string                             `json:"source"`
	Origin      string                             `json:"origin"`
	Enabled     bool                               `json:"enabled"`
	Activation  apicontract.SkillActivationPayload `json:"activation"`
}
