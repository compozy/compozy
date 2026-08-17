package hooks

const HookSessionAttentionChanged HookEvent = "session.attention.changed"

func sessionAttentionHookEventDefinitions() []hookEventDefinition {
	return []hookEventDefinition{{
		event:        HookSessionAttentionChanged,
		family:       HookEventFamilySession,
		syncEligible: false,
	}}
}
