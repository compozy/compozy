package daemon

import (
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func nativeResultViewFixturesCommand(t *testing.T) []nativeResultViewFixture {
	t.Helper()
	p := contract.SessionCommandsResponse{
		Revision: "rev",
		Commands: []contract.SessionCommandPayload{
			{
				ID:                "skill:review",
				CanonicalToken:    "/review",
				DisplayName:       "Review",
				Description:       strings.Repeat("Review changes ", 30),
				Lane:              "skill",
				Source:            contract.SessionCommandSource{Kind: "skill", Scope: "workspace", ID: "source"},
				Placements:        []string{"composer"},
				InputHint:         "changes",
				Available:         false,
				UnavailableReason: "verification_failed",
			},
		},
	}
	return []nativeResultViewFixture{
		{
			ToolID: toolspkg.ToolIDCommandList,
			Name:   "unavailable command",
			Result: func(v toolspkg.ResultView) (toolspkg.ToolResult, error) { return commandListViews.result(v, p) },
			MustKeep: []string{
				"revision",
				"commands.0.id",
				"commands.0.canonical_token",
				"commands.0.input_hint",
				"commands.0.available",
				"commands.0.unavailable_reason",
				"commands.0.source.kind",
				"commands.0.source.scope",
			},
		},
		{
			ToolID: toolspkg.ToolIDCommandList,
			Name:   "empty list",
			Result: func(v toolspkg.ResultView) (toolspkg.ToolResult, error) {
				p.Commands = []contract.SessionCommandPayload{}
				return commandListViews.result(v, p)
			},
			MustKeep: []string{"revision", "commands"},
		},
	}
}
