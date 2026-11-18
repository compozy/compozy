package daemon

import (
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/cmdpalette"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func nativeResultViewFixturesCmd(t *testing.T) []nativeResultViewFixture {
	t.Helper()
	p := contract.CmdPaletteCommand{
		ID:           "window.close",
		Title:        "Close",
		Source:       "daemon",
		Available:    false,
		Reason:       "requires attached shell",
		Destructive:  true,
		Section:      "Windows",
		Icon:         "close",
		Bindings:     []string{"mod+w"},
		Confirmation: &cmdpalette.Confirmation{Title: "Close?", Body: "Close window", Confirm: "Close"},
		Arguments:    []cmdpalette.Argument{{Name: "target", Type: cmdpalette.ArgumentTypeText, Required: true}},
	}
	return []nativeResultViewFixture{
		{
			ToolID: toolspkg.ToolIDCmdPaletteList,
			Name:   "unavailable command",
			Result: func(v toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return paletteListViews.result(v, []contract.CmdPaletteCommand{p})
			},
			MustKeep: []string{
				"commands.0.id",
				"commands.0.title",
				"commands.0.source",
				"commands.0.available",
				"commands.0.reason",
				"commands.0.destructive",
				"commands.0.arguments.0.name",
				"commands.0.arguments.0.required",
				"commands.0.confirmation.title",
				"commands.0.confirmation.confirm",
			},
		},
		{
			ToolID: toolspkg.ToolIDCmdPaletteList,
			Name:   "empty list",
			Result: func(v toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return paletteListViews.result(v, []contract.CmdPaletteCommand{})
			},
			MustKeep: []string{"commands"},
		},
	}
}
