package daemon

import toolspkg "github.com/compozy/compozy/internal/tools"

func (n *daemonNativeTools) sessionOrchestrationToolBindings(
	sessionAvailability toolspkg.NativeAvailabilityFunc,
	waitAvailability toolspkg.NativeAvailabilityFunc,
	spawnAvailability toolspkg.NativeAvailabilityFunc,
	clarifyAvailability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDSessionWait: {
			call: n.sessionWait, availability: waitAvailability,
		},
		toolspkg.ToolIDSessionSpawn: {
			call: n.sessionSpawn, availability: spawnAvailability,
		},
		toolspkg.ToolIDSessionStop: {
			call: n.sessionStop, availability: sessionAvailability,
		},
		toolspkg.ToolIDSessionApprove: {
			call: n.sessionApprove, availability: sessionAvailability,
		},
		toolspkg.ToolIDSessionClarifyAnswer: {
			call: n.sessionClarifyAnswer, availability: clarifyAvailability,
		},
		toolspkg.ToolIDSessionPromptCancel: {
			call: n.sessionPromptCancel, availability: sessionAvailability,
		},
	}
}
