package hooks

func sessionAttentionHookEventDescriptors() map[HookEvent]EventDescriptor {
	return map[HookEvent]EventDescriptor{
		HookSessionAttentionChanged: {
			Event:         HookSessionAttentionChanged,
			Family:        HookEventFamilySession,
			SyncEligible:  false,
			PayloadSchema: "SessionAttentionChangedPayload",
			PatchSchema:   "SessionAttentionObservationPatch",
		},
	}
}
