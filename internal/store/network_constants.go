package store

import "regexp"

const (
	typesSentKey     = "sent"
	typesRejectedKey = "rejected"
	typesThreadIDKey = "thread_id"
	typesDirectIDKey = "direct_id"
)

const (
	// NetworkSurfaceThread stores a public thread conversation container.
	NetworkSurfaceThread = "thread"
	// NetworkSurfaceDirect stores a two-party direct-room conversation container.
	NetworkSurfaceDirect = "direct"

	// NetworkFanoutPolicyCapabilityMatch activates peers by declared capability.
	NetworkFanoutPolicyCapabilityMatch = "capability_match"
	// NetworkFanoutPolicyCoordinator activates the configured channel coordinator.
	NetworkFanoutPolicyCoordinator = "coordinator"
	// NetworkFanoutPolicyAllMembers activates every eligible local channel member.
	NetworkFanoutPolicyAllMembers = "all_members"

	// NetworkSubscriptionModeMute suppresses unmentioned matching traffic.
	NetworkSubscriptionModeMute = "mute"
	// NetworkSubscriptionModeFull preserves full prompt injection.
	NetworkSubscriptionModeFull = "full"

	// NetworkKindGreet stores a presence announcement.
	NetworkKindGreet = "greet"
	// NetworkKindWhois stores a peer identity request or response.
	NetworkKindWhois = "whois"
	// NetworkKindSay stores a text conversation message.
	NetworkKindSay = "say"
	// NetworkKindCapability stores a capability transfer message.
	NetworkKindCapability = "capability"
	// NetworkKindReceipt stores an admission receipt message.
	NetworkKindReceipt = "receipt"
	// NetworkKindTrace stores a work lifecycle trace message.
	NetworkKindTrace = "trace"

	// NetworkWorkStateSubmitted is the initial work state.
	NetworkWorkStateSubmitted = "submitted"
	// NetworkWorkStateWorking marks active work.
	NetworkWorkStateWorking = "working"
	// NetworkWorkStateNeedsInput marks blocked work awaiting input.
	NetworkWorkStateNeedsInput = "needs_input"
	// NetworkWorkStateCompleted marks successful terminal work.
	NetworkWorkStateCompleted = "completed"
	// NetworkWorkStateFailed marks failed terminal work.
	NetworkWorkStateFailed = "failed"
	// NetworkWorkStateCanceled marks canceled terminal work.
	NetworkWorkStateCanceled = "canceled"
)

var (
	networkThreadIDPattern = regexp.MustCompile(`^thread_[a-z0-9][a-z0-9_-]{2,95}$`)
	networkDirectIDPattern = regexp.MustCompile(`^direct_[a-f0-9]{32}$`)
	networkPeerIDPattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
)
