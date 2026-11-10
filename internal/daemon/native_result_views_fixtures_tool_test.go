package daemon

import (
	"testing"
	"time"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func nativeResultViewFixturesTool(t *testing.T) []nativeResultViewFixture {
	t.Helper()
	grantKey := toolspkg.ApprovalGrantKey{
		ProfileID:   "profile",
		WorkspaceID: "ws",
		AgentName:   "reviewer",
		ToolID:      toolspkg.ToolIDToolList,
		InputDigest: "sha256:input",
		AccessScope: "read:workspace:ws@profile",
	}
	grant := toolspkg.ApprovalGrant{
		ID:               "grant",
		ApprovalGrantKey: grantKey,
		Decision:         toolspkg.ApprovalGrantAllow,
		ProfileName:      "Work",
		ProfileColor:     "blue",
		ProfileIcon:      "work",
		ProfileEmoji:     "icon",
		ProfileArchived:  true,
		CreatedAt:        time.Unix(1, 0).UTC(),
		LastUsedAt:       time.Unix(2, 0).UTC(),
	}
	availability := toolspkg.Availability{
		Registered:  true,
		Enabled:     true,
		ReasonCodes: []toolspkg.ReasonCode{toolspkg.ReasonDependencyMissing},
	}
	decision := toolspkg.EffectiveToolDecision{
		ApprovalRequired:     true,
		ReasonCodes:          []toolspkg.ReasonCode{toolspkg.ReasonPolicyDenied},
		SystemPermissionMode: "approve-reads",
		SessionPolicyResult:  "denied",
	}
	page := nativeToolListPage{
		Tools: []nativeToolListView{
			{
				Descriptor: nativeToolListDescriptor{
					ID:                 toolspkg.ToolIDToolList,
					Risk:               toolspkg.RiskRead,
					HasResultViews:     true,
					InputSchemaDigest:  "sha256:input",
					OutputSchemaDigest: "sha256:output",
				},
				Availability: availability,
				Decision:     decision,
			},
		},
		Count:      1,
		Total:      2,
		NextOffset: 1,
		HasMore:    true,
	}
	descriptor := toolspkg.Descriptor{
		ID:           toolspkg.ToolIDToolList,
		Description:  "List tools",
		Risk:         toolspkg.RiskRead,
		InputSchema:  []byte(`{"type":"object"}`),
		OutputSchema: []byte(`{"type":"object"}`),
	}
	descriptor.SetResultViews(
		&toolspkg.ResultViews{
			Compact: toolspkg.ResultViewContract{
				Schema: []byte(`{"type":"object","additionalProperties":false}`),
				Digest: "sha256:compact",
			},
			Full: toolspkg.ResultViewContract{Schema: []byte(`{"type":"object"}`), Digest: "sha256:full"},
		},
	)
	return []nativeResultViewFixture{
		{
			ToolID: toolspkg.ToolIDToolList,
			Name:   "unavailable paginated tool",
			Result: func(v toolspkg.ResultView) (toolspkg.ToolResult, error) { return toolListViews.result(v, page) },
			MustKeep: []string{
				"tools.0.descriptor.id",
				"tools.0.descriptor.risk",
				"tools.0.descriptor.has_result_views",
				"tools.0.availability.reason_codes.0",
				"tools.0.decision.callable",
				"tools.0.decision.approval_required",
				"tools.0.decision.reason_codes.0",
				"offset",
				"count",
				"total",
				"has_more",
				"next_offset",
			},
		},
		{
			ToolID: toolspkg.ToolIDToolList,
			Name:   "empty list",
			Result: func(v toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return toolListViews.result(v, nativeToolListPage{Tools: []nativeToolListView{}})
			},
			MustKeep: []string{"tools", "has_more", "total"},
		},
		{
			ToolID: toolspkg.ToolIDToolSearch,
			Name:   "unavailable tool",
			Result: func(v toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return toolSearchViews.result(
					v,
					[]toolspkg.ToolView{{Descriptor: descriptor, Availability: availability, Decision: decision}},
				)
			},
			MustKeep: []string{
				"tools.0.descriptor.id",
				"tools.0.descriptor.description",
				"tools.0.descriptor.output_views.compact.digest",
				"tools.0.descriptor.output_views.full.digest",
				"tools.0.descriptor.risk",
				"tools.0.availability.reason_codes.0",
				"tools.0.decision.reason_codes.0",
				"tools.0.decision.approval_required",
			},
		},
		{
			ToolID: toolspkg.ToolIDToolSearch,
			Name:   "empty list",
			Result: func(v toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return toolSearchViews.result(v, []toolspkg.ToolView{})
			},
			MustKeep: []string{"tools"},
		},
		{
			ToolID: toolspkg.ToolIDToolApprovalsSet,
			Name:   "access grant",
			Result: func(v toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return approvalSetViews.result(v, nativeToolApprovalSetResponse{Grant: grant})
			},
			MustKeep: []string{
				"grant.id",
				"grant.profile_id",
				"grant.workspace_id",
				"grant.agent_name",
				"grant.tool_id",
				"grant.input_digest",
				"grant.access_scope",
				"grant.decision",
				"grant.created_at",
				"grant.last_used_at",
			},
		},
		{
			ToolID: toolspkg.ToolIDToolApprovalsList,
			Name:   "access grant",
			Result: func(v toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return approvalListViews.result(
					v,
					nativeToolApprovalListResponse{Grants: []toolspkg.ApprovalGrant{grant}, Total: 1},
				)
			},
			MustKeep: []string{
				"total",
				"grants.0.id",
				"grants.0.profile_id",
				"grants.0.workspace_id",
				"grants.0.agent_name",
				"grants.0.tool_id",
				"grants.0.input_digest",
				"grants.0.access_scope",
				"grants.0.decision",
				"grants.0.created_at",
				"grants.0.last_used_at",
			},
		},
		{
			ToolID: toolspkg.ToolIDToolApprovalsList,
			Name:   "empty list",
			Result: func(v toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return approvalListViews.result(v, nativeToolApprovalListResponse{Grants: []toolspkg.ApprovalGrant{}})
			},
			MustKeep: []string{"grants", "total"},
		},
	}
}
