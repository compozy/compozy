package extensionpkg

import (
	"strings"
	"time"

	apicontract "github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/session"
	speedpkg "github.com/compozy/compozy/internal/speed"
)

const maxHostAPIRuntimeDiagnosticBytes = 2048

func hostAPISessionRuntimePayloadFromInfo(info *session.Info) apicontract.SessionRuntimePayload {
	if info == nil {
		return apicontract.SessionRuntimePayload{}
	}

	payload := apicontract.SessionRuntimePayload{
		Status:            info.RuntimeStatus,
		Transition:        info.RuntimeTransition,
		Failure:           diagnostics.RedactAndBound(info.RuntimeFailure, maxHostAPIRuntimeDiagnosticBytes),
		Selected:          apicontract.PromptRuntimeSelectionPayloadFromSelection(info.SelectedRuntime),
		SelectionRevision: info.RuntimeSelectionRevision,
		ACPSessionID:      strings.TrimSpace(info.ACPSessionID),
	}
	if hostAPIRuntimeHasEffectiveSelection(info) {
		payload.Effective = &apicontract.RuntimeSelectionPayload{
			Provider:        strings.TrimSpace(info.Provider),
			Model:           strings.TrimSpace(info.Model),
			ReasoningEffort: apicontract.ReasoningEffort(strings.TrimSpace(info.ReasoningEffort)),
			Speed:           info.Speed,
			SpeedResolution: speedpkg.CloneResolution(info.SpeedResolution),
		}
	}
	payload.ACPCaps = apicontract.ACPCapsPayloadFromACP(info.ACPCaps, info.ACPCapsKnown)
	return payload
}

func hostAPIRuntimeHasEffectiveSelection(info *session.Info) bool {
	if info == nil || strings.TrimSpace(info.Provider) == "" {
		return false
	}
	switch info.RuntimeStatus {
	case session.RuntimeStatusReady, session.RuntimeStatusReconfiguring, session.RuntimeStatusFailed:
		return true
	case session.RuntimeStatusUnbound:
		return info.RuntimeTransition != session.RuntimeTransitionNone
	default:
		return false
	}
}

func hostAPISessionStatusFromInfo(info *session.Info) hostAPISessionStatus {
	if info == nil {
		return hostAPISessionStatus{}
	}
	return hostAPISessionStatus{
		SessionID:   info.ID,
		Name:        info.Name,
		Agent:       info.AgentName,
		Runtime:     hostAPISessionRuntimePayloadFromInfo(info),
		WorkspaceID: info.WorkspaceID,
		Workspace:   info.Workspace,
		State:       info.State,
		ArchivedAt:  cloneHostAPITime(info.ArchivedAt),
		StopReason:  info.StopReason,
		StopDetail:  info.StopDetail,
		CreatedAt:   info.CreatedAt,
		UpdatedAt:   info.UpdatedAt,
	}
}

func cloneHostAPITime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	return new(value.UTC())
}
