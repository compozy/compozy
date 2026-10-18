package hooks

import "slices"

func cloneSessionContext(payload SessionContext) SessionContext {
	if payload.SessionRuntimeContext != nil {
		payload.SessionRuntimeContext = new(*payload.SessionRuntimeContext)
	}
	if payload.SessionSoulContext != nil {
		payload.SessionSoulContext = new(*payload.SessionSoulContext)
	}
	return payload
}

func cloneAutomationSchedulePayload(payload *AutomationSchedulePayload) *AutomationSchedulePayload {
	if payload == nil {
		return nil
	}

	return new(*payload)
}

func clonePermissionToolCall(call PermissionToolCall) PermissionToolCall {
	call.Locations = cloneToolLocations(call.Locations)
	return call
}

func clonePermissionOptions(options []PermissionOption) []PermissionOption {
	if options == nil {
		return nil
	}

	return slices.Clone(options)
}

func cloneToolLocations(locations []ToolLocation) []ToolLocation {
	if locations == nil {
		return nil
	}

	return slices.Clone(locations)
}

func cloneStringSlice(values []string) []string {
	if values == nil {
		return nil
	}

	return slices.Clone(values)
}

func cloneAnyMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}

	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = cloneAnyValue(value)
	}
	return dst
}

func cloneAnySlice(values []any) []any {
	if values == nil {
		return nil
	}

	cloned := make([]any, len(values))
	for i, value := range values {
		cloned[i] = cloneAnyValue(value)
	}
	return cloned
}
