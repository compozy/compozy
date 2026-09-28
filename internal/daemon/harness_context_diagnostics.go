package daemon

import "strings"

func buildHarnessDiagnosticLabel(
	sessionCtx HarnessSessionContext,
	policy ResolvedHarnessPolicy,
) string {
	parts := []string{string(sessionCtx.SessionClass)}
	parts = append(parts, string(policy.TurnOrigin))
	if policy.ReentryMode == ReentryModeSynthetic {
		parts = append(parts, "reentry")
	}
	return strings.Join(parts, ".")
}

func buildHarnessObservabilityTags(
	surface ResolutionSurface,
	sessionCtx HarnessSessionContext,
	turnCtx HarnessTurnContext,
	policy ResolvedHarnessPolicy,
) map[string]string {
	tags := map[string]string{
		harnessContextHarnessSurfacePath:         string(surface),
		harnessContextHarnessSessionTypePath:     string(sessionCtx.Type),
		harnessContextHarnessSessionClassPath:    string(policy.SessionClass),
		harnessContextHarnessTurnOriginPath:      string(policy.TurnOrigin),
		harnessContextHarnessDiagnosticLabelPath: policy.DiagnosticLabel,
	}
	if sessionID := strings.TrimSpace(sessionCtx.SessionID); sessionID != "" {
		tags["session_id"] = sessionID
	}
	if provider := strings.TrimSpace(sessionCtx.Provider); provider != "" {
		tags["provider"] = provider
	}
	if turnCtx.Synthetic != nil {
		tags["harness.synthetic_reason"] = turnCtx.Synthetic.Reason
		tags["harness.synthetic_trigger"] = turnCtx.Synthetic.Trigger
	}
	if turnCtx.Detached != nil {
		if turnCtx.Detached.TaskID != "" {
			tags["harness.task_id"] = turnCtx.Detached.TaskID
		}
		if turnCtx.Detached.TaskRunID != "" {
			tags["harness.task_run_id"] = turnCtx.Detached.TaskRunID
		}
	}
	return tags
}
