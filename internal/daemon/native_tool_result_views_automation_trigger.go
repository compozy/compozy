package daemon

import (
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
	automationpkg "github.com/compozy/compozy/internal/automation"
)

type compactTriggerRecord struct {
	ID                   string                             `json:"id"`
	ProfileID            string                             `json:"profile_id"`
	ProfileName          string                             `json:"profile_name"`
	Scope                automationpkg.Scope                `json:"scope"`
	Name                 string                             `json:"name"`
	TargetKind           automationpkg.TargetKind           `json:"target_kind"`
	TargetInstanceScope  string                             `json:"target_instance_scope,omitempty"`
	AgentName            string                             `json:"agent_name"`
	WorkspaceID          string                             `json:"workspace_id,omitempty"`
	Event                string                             `json:"event"`
	Enabled              bool                               `json:"enabled"`
	Source               automationpkg.JobSource            `json:"source"`
	WebhookID            string                             `json:"webhook_id,omitempty"`
	EndpointSlug         string                             `json:"endpoint_slug,omitempty"`
	WebhookSecretPresent bool                               `json:"webhook_secret_present"`
	LastRun              *contract.AutomationLastRunPayload `json:"last_run,omitzero"`
	Ingress              *contract.GatewayIngressPayload    `json:"ingress,omitempty"`
	LoopTarget           *compactAutomationLoopTarget       `json:"loop_target,omitempty"`
}

func compactTrigger(p contract.TriggerPayload) compactTriggerRecord {
	result := compactTriggerRecord{
		ID:                   p.ID,
		ProfileID:            p.ProfileID,
		ProfileName:          p.ProfileName,
		Scope:                p.Scope,
		Name:                 p.Name,
		TargetKind:           p.TargetKind,
		TargetInstanceScope:  p.TargetInstanceScope,
		AgentName:            p.AgentName,
		WorkspaceID:          p.WorkspaceID,
		Event:                p.Event,
		Enabled:              p.Enabled,
		Source:               p.Source,
		WebhookID:            p.WebhookID,
		EndpointSlug:         p.EndpointSlug,
		WebhookSecretPresent: p.WebhookSecretPresent,
		LastRun:              p.LastRun,
		Ingress:              p.Ingress,
	}
	if p.LoopTarget != nil {
		value := compactAutomationTarget(*p.LoopTarget)
		result.LoopTarget = &value
	}
	return result
}

var triggerRecordViews = resultViews[contract.TriggerPayload]{
	Compact: func(p contract.TriggerPayload) any {
		return map[string]any{nativeAutomationToolsTriggerKey: compactTrigger(p)}
	},
	Full:    func(p contract.TriggerPayload) any { return map[string]any{nativeAutomationToolsTriggerKey: p} },
	Preview: func(p contract.TriggerPayload) string { return p.ID },
}

var triggerListViews = resultViews[contract.TriggersResponse]{
	Compact: func(p contract.TriggersResponse) any {
		var rows []compactTriggerRecord
		if p.Triggers != nil {
			rows = make([]compactTriggerRecord, len(p.Triggers))
			for i, value := range p.Triggers {
				rows[i] = compactTrigger(value)
			}
		}
		return map[string]any{"triggers": rows, "page": p.Page}
	},
	Preview: func(p contract.TriggersResponse) string {
		return fmt.Sprintf("%d of %d automation triggers", len(p.Triggers), p.Page.Total)
	},
}
