package automation

import "strings"

func normalizeTriggerRegistration(registration TriggerRegistration) (TriggerRegistration, error) {
	trigger := cloneTrigger(registration.Trigger)
	trigger.ID = strings.TrimSpace(trigger.ID)
	trigger.Name = strings.TrimSpace(trigger.Name)
	trigger.TargetKind = trigger.TargetKind.Normalize()
	trigger.AgentName = strings.TrimSpace(trigger.AgentName)
	trigger.WorkspaceID = strings.TrimSpace(trigger.WorkspaceID)
	trigger.Prompt = strings.TrimSpace(trigger.Prompt)
	trigger.Event = strings.TrimSpace(trigger.Event)
	trigger.WebhookID = strings.TrimSpace(trigger.WebhookID)
	trigger.EndpointSlug = strings.TrimSpace(trigger.EndpointSlug)
	trigger.WebhookSecretRef = strings.TrimSpace(trigger.WebhookSecretRef)
	normalized := TriggerRegistration{Trigger: trigger}
	if err := normalized.Validate("trigger_registration"); err != nil {
		return TriggerRegistration{}, err
	}
	normalized.compiledFilter = compileTriggerFilter(normalized.Trigger.Filter)
	return normalized, nil
}

func registrationMatchesEnvelope(registration TriggerRegistration, envelope ActivationEnvelope) bool {
	trigger := registration.Trigger
	if !trigger.Enabled {
		return false
	}
	if strings.TrimSpace(trigger.Event) != strings.TrimSpace(envelope.Kind) {
		return false
	}
	if trigger.Scope != envelope.Scope {
		return false
	}
	if strings.TrimSpace(trigger.WorkspaceID) != strings.TrimSpace(envelope.WorkspaceID) {
		return false
	}
	if len(trigger.Filter) == 0 {
		return true
	}
	if len(registration.compiledFilter.entries) == len(trigger.Filter) {
		return registration.compiledFilter.matches(envelope)
	}
	return exactFilterMatch(trigger.Filter, envelope)
}

func cloneTriggerRegistration(src TriggerRegistration) TriggerRegistration {
	return TriggerRegistration{
		Trigger:        cloneTrigger(src.Trigger),
		compiledFilter: cloneTriggerFilter(src.compiledFilter),
	}
}
