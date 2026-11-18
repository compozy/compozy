package daemon

import (
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/cmdpalette"
)

type compactPaletteCommand struct {
	ID           cmdpalette.CommandID     `json:"id"`
	Title        string                   `json:"title"`
	Source       string                   `json:"source"`
	Available    bool                     `json:"available"`
	Reason       string                   `json:"reason,omitempty"`
	Destructive  bool                     `json:"destructive"`
	Confirmation *cmdpalette.Confirmation `json:"confirmation,omitzero"`
	Arguments    []cmdpalette.Argument    `json:"arguments"`
}

func projectCompactPaletteCommand(p contract.CmdPaletteCommand) compactPaletteCommand {
	return compactPaletteCommand{
		ID:           p.ID,
		Title:        p.Title,
		Source:       p.Source,
		Available:    p.Available,
		Reason:       p.Reason,
		Destructive:  p.Destructive,
		Confirmation: p.Confirmation,
		Arguments:    p.Arguments,
	}
}
