package hooks

func cloneSessionContext(payload SessionContext) SessionContext {
	if payload.SessionSoulContext == nil {
		return payload
	}
	soul := *payload.SessionSoulContext
	payload.SessionSoulContext = &soul
	return payload
}

func cloneSandboxProfilePayload(payload SandboxProfilePayload) SandboxProfilePayload {
	payload.Env = cloneStringMap(payload.Env)
	payload.SecretEnv = cloneStringMap(payload.SecretEnv)
	return payload
}

func cloneAutomationSchedulePayload(payload *AutomationSchedulePayload) *AutomationSchedulePayload {
	if payload == nil {
		return nil
	}

	cloned := *payload
	return &cloned
}

func clonePermissionToolCall(call PermissionToolCall) PermissionToolCall {
	call.Locations = cloneToolLocations(call.Locations)
	return call
}

func clonePermissionOptions(options []PermissionOption) []PermissionOption {
	if options == nil {
		return nil
	}

	cloned := make([]PermissionOption, len(options))
	copy(cloned, options)
	return cloned
}

func cloneToolLocations(locations []ToolLocation) []ToolLocation {
	if locations == nil {
		return nil
	}

	cloned := make([]ToolLocation, len(locations))
	copy(cloned, locations)
	return cloned
}

func cloneStringSlice(values []string) []string {
	if values == nil {
		return nil
	}

	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
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
