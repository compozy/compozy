package hooks

func callHookEventDescriptors() map[HookEvent]EventDescriptor {
	descriptors := make(map[HookEvent]EventDescriptor, len(callHookEventDefinitions()))
	for _, definition := range callHookEventDefinitions() {
		descriptors[definition.event] = EventDescriptor{
			Event: definition.event, Family: definition.family, SyncEligible: definition.syncEligible,
			PayloadSchema: "CallPayload", PatchSchema: "CallObservationPatch",
		}
	}
	return descriptors
}
