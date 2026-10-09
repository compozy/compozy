package daemon

import (
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func subagentRuntimePayload(target session.SubagentTarget) map[string]any {
	return map[string]any{
		daemonAgentField:              target.Agent,
		watchEventsPayloadProviderKey: target.Provider,
		"model":                       target.Model,
		"reasoning_effort":            target.ReasoningEffort,
		"speed":                       target.Speed,
	}
}

func subagentCapabilitiesPayload(c session.SubagentCapabilities) map[string]any {
	agents := make([]map[string]any, 0, len(c.Agents))
	for _, a := range c.Agents {
		agents = append(
			agents,
			map[string]any{
				"name":                        a.Name,
				watchEventsPayloadProviderKey: a.Provider,
				"can_delegate":                a.CanDelegate,
				"constraints":                 nonNilSubagentStrings(a.Constraints),
			},
		)
	}
	providers := make([]map[string]any, 0, len(c.Providers))
	for _, p := range c.Providers {
		models := make([]map[string]any, 0, len(p.Models))
		for _, m := range p.Models {
			models = append(
				models,
				map[string]any{
					"id":                m.ID,
					"label":             m.Label,
					"reasoning_efforts": nonNilSubagentStrings(m.ReasoningEfforts),
					"speeds":            nonNilSubagentStrings(m.Speeds),
				},
			)
		}
		providers = append(
			providers,
			map[string]any{
				watchEventsPayloadProviderKey: p.Provider,
				"display_name":                p.DisplayName,
				"can_delegate":                p.CanDelegate,
				"constraints":                 nonNilSubagentStrings(p.Constraints),
				"models":                      models,
				"models_total":                p.ModelsTotal,
				"models_truncated":            p.ModelsTruncated,
			},
		)
	}
	return map[string]any{
		"parent_session_id": c.ParentSessionID,
		"inherited":         subagentRuntimePayload(c.Inherited),
		"permission_mode":   c.PermissionMode,
		"depth":             c.Depth,
		"live_subagents":    c.Live,
		"agents":            agents,
		"providers":         providers,
	}
}

func nonNilSubagentStrings(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

func subagentPayload(s *session.Subagent) map[string]any {
	p := map[string]any{
		"subagent_id":       s.ID,
		"parent_session_id": s.ParentSessionID,
		"parent_turn_id":    s.ParentTurnID,
		"child_session_id":  s.ChildSessionID,
		"origin":            s.Origin,
		"title":             s.Title,
		"role":              s.Role,
		"status":            s.Status,
		"work_state":        s.WorkState,
		"progress":          s.Progress,
		"runtime": subagentRuntimePayload(
			session.SubagentTarget{
				Agent:           s.RuntimeAgent,
				Provider:        s.RuntimeProvider,
				Model:           s.RuntimeModel,
				ReasoningEffort: s.RuntimeEffort,
				Speed:           s.RuntimeSpeed,
			},
		),
		"depth":            s.Depth,
		"result":           s.Result,
		"result_truncated": s.ResultTruncated,
		"error":            s.Error,
		"wait_timed_out":   s.WaitTimedOut,
		"delivery":         s.Delivery,
		"started_at":       s.StartedAt,
		"settled_at":       s.SettledAt,
	}
	if s.ResultTruncated {
		p["hint"] = "Read the full answer with compozy__session_history on child_session_id."
	}
	return p
}

func subagentInvalidResult(message string) (toolspkg.ToolResult, error) {
	return subagentErrorResult("invalid_request", message)
}
func subagentInputFailure(err error) (toolspkg.ToolResult, error) {
	return subagentInvalidResult(err.Error())
}
func subagentErrorResult(code, message string) (toolspkg.ToolResult, error) {
	return structuredResult(map[string]any{"error": map[string]string{"code": code, "message": message}}, message)
}

func subagentFailure(err error, id string) (toolspkg.ToolResult, error) {
	if detail, ok := errors.AsType[*session.SubagentError](err); ok {
		return subagentErrorResult(detail.Code, detail.Message)
	}
	switch {
	case errors.Is(err, session.ErrSubagentParentNotActive):
		return subagentErrorResult("parent_not_active", "Subagents require an active turn in the calling session.")
	case errors.Is(err, session.ErrSubagentNotFound), errors.Is(err, store.ErrSubagentNotFound):
		return subagentErrorResult("subagent_not_found", fmt.Sprintf("Subagent %s not found.", id))
	case errors.Is(err, session.ErrSubagentNotCancelable):
		return subagentErrorResult(
			"subagent_not_cancelable",
			"Provider-native subagents cannot be canceled; stop the parent turn instead.",
		)
	case errors.Is(err, session.ErrSubagentArchiveFollows):
		return subagentErrorResult(
			"subagent_archive_follows_parent",
			"Subagent sessions are archived with their parent session.",
		)
	case errors.Is(err, session.ErrSubagentInvalidRequest), errors.Is(err, store.ErrSubagentIdempotencyConflict):
		return subagentInvalidResult(subagentFailureMessage(err, session.ErrSubagentInvalidRequest))
	case errors.Is(err, session.ErrSubagentCapabilityDenied):
		return subagentErrorResult(
			"capability_denied",
			subagentFailureMessage(err, session.ErrSubagentCapabilityDenied),
		)
	case errors.Is(err, session.ErrSpawnPermissionDenied):
		return subagentErrorResult(
			"permission_escalation_denied",
			subagentFailureMessage(err, session.ErrSpawnPermissionDenied),
		)
	case errors.Is(err, session.ErrSubagentTargetUnavailable):
		return subagentErrorResult(
			"provider_unavailable",
			subagentFailureMessage(err, session.ErrSubagentTargetUnavailable),
		)
	default:
		return toolspkg.ToolResult{}, err
	}
}

func subagentFailureMessage(err, sentinel error) string {
	for inner := errors.Unwrap(err); inner != nil && inner != sentinel; inner = errors.Unwrap(err) {
		err = inner
	}
	return strings.TrimPrefix(err.Error(), sentinel.Error()+": ")
}
