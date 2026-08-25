package hooks

const (
	HookCallCreated          HookEvent = "call.created"
	HookCallSettled          HookEvent = "call.settled"
	HookCallCanceled         HookEvent = "call.canceled"
	HookCallPublished        HookEvent = "call.published"
	HookCallMessageSent      HookEvent = "call.message_sent"
	HookCallMessageDelivered HookEvent = "call.message_delivered"
	HookCallSubtreeDrained   HookEvent = "call.subtree_drained"
)

func callHookEventDefinitions() []hookEventDefinition {
	return []hookEventDefinition{
		{event: HookCallCreated, family: HookEventFamilyCall, syncEligible: false},
		{event: HookCallSettled, family: HookEventFamilyCall, syncEligible: false},
		{event: HookCallCanceled, family: HookEventFamilyCall, syncEligible: false},
		{event: HookCallPublished, family: HookEventFamilyCall, syncEligible: false},
		{event: HookCallMessageSent, family: HookEventFamilyCall, syncEligible: false},
		{event: HookCallMessageDelivered, family: HookEventFamilyCall, syncEligible: false},
		{event: HookCallSubtreeDrained, family: HookEventFamilyCall, syncEligible: false},
	}
}
