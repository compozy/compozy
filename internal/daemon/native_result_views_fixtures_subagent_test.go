package daemon

import (
	"fmt"
	"testing"

	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func nativeResultViewFixturesSubagent(t *testing.T) []nativeResultViewFixture {
	t.Helper()
	capabilities := session.SubagentCapabilities{
		ParentSessionID: "sess-parent",
		Inherited:       session.SubagentTarget{Agent: "general", Provider: "codex", Model: "current"},
		PermissionMode:  "approve-all",
		Depth:           1,
		Live:            2,
		Agents:          []session.SubagentAgentOption{{Name: "general", Provider: "codex", CanDelegate: true}},
		Providers: []session.SubagentProviderOption{
			{Provider: "codex", DisplayName: "Codex", CanDelegate: true, ModelsTotal: 50, ModelsTruncated: true},
			{
				Provider:        "unavailable",
				DisplayName:     "Unavailable",
				Constraints:     []string{"provider_unavailable"},
				ModelsTotal:     2,
				ModelsTruncated: true,
			},
		},
	}
	for i := range 40 {
		capabilities.Providers[0].Models = append(capabilities.Providers[0].Models, session.SubagentModelOption{
			ID: fmt.Sprintf("model-%d", i), Label: fmt.Sprintf("Model %d", i),
			ReasoningEfforts: []string{"low", "medium", "high"}, Speeds: []string{"normal", "fast"},
		})
	}
	empty := session.SubagentCapabilities{ParentSessionID: "sess-parent"}
	return []nativeResultViewFixture{
		{ToolID: toolspkg.ToolIDSubagentCapabilities, Name: "bounded model catalog and constraints",
			Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return subagentCapabilitiesViews.result(view, capabilities)
			},
			MustKeep: []string{
				"parent_session_id",
				"inherited",
				"permission_mode",
				"depth",
				"live_subagents",
				"agents.0.name",
				"agents.0.can_delegate",
				"agents.0.constraints",
				"providers.0.provider",
				"providers.0.models_total",
				"providers.0.models_truncated",
				"providers.1.can_delegate",
				"providers.1.constraints",
			}},
		{ToolID: toolspkg.ToolIDSubagentCapabilities, Name: "empty catalogs",
			Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return subagentCapabilitiesViews.result(view, empty)
			},
			MustKeep: []string{"parent_session_id", "agents", "providers", "depth", "live_subagents"}},
		{ToolID: toolspkg.ToolIDSubagentCapabilities, Name: "failure inside successful result",
			Result: func(toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return subagentFailure(session.ErrSubagentParentNotActive, "")
			},
			MustKeep: []string{"error.code", "error.message"}},
	}
}
