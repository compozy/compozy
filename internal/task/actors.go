package task

import (
	"fmt"
	"strings"
)

// FullAccessAuthority returns the v1 broad task-domain authority granted to
// authenticated first-class task surfaces after ingress-level authentication
// and capability checks succeed.
func FullAccessAuthority() Authority {
	return Authority{
		Read:            true,
		Write:           true,
		CreateGlobal:    true,
		CreateWorkspace: true,
	}
}

// DeriveHumanActorContext derives one trusted local-human actor context for
// CLI, web, HTTP, or UDS task ingress.
func DeriveHumanActorContext(actorRef string, originKind OriginKind, originRef string) (ActorContext, error) {
	return DeriveHumanActorContextForWorkspace(actorRef, "", originKind, originRef)
}

// DeriveHumanActorContextForWorkspace derives a trusted local-human actor bound to an optional route workspace.
func DeriveHumanActorContextForWorkspace(
	actorRef string,
	workspaceID string,
	originKind OriginKind,
	originRef string,
) (ActorContext, error) {
	switch originKind.Normalize() {
	case OriginKindCLI, OriginKindWeb, OriginKindUDS, OriginKindHTTP:
	default:
		return ActorContext{}, fmt.Errorf(
			"%w: human task ingress requires cli, web, uds, or http origin, got %q",
			ErrValidation,
			originKind,
		)
	}
	return deriveActorContext(
		ActorKindHuman,
		actorRef,
		originKind,
		originRef,
		CallerScope{WorkspaceID: strings.TrimSpace(workspaceID), Operator: true},
	)
}

// DeriveAgentSessionActorContext derives one trusted agent-session actor
// context. The session ref becomes both the immutable actor ref and origin ref.
func DeriveAgentSessionActorContext(sessionRef string, workspaceID string) (ActorContext, error) {
	return deriveActorContext(
		ActorKindAgentSession,
		sessionRef,
		OriginKindAgentSession,
		sessionRef,
		agentCallerScope(sessionRef, workspaceID),
	)
}

// DeriveAgentSessionActorContextForOrigin derives one trusted agent-session
// actor context for an authenticated agent ingress operation.
func DeriveAgentSessionActorContextForOrigin(
	sessionRef string,
	workspaceID string,
	originKind OriginKind,
	originRef string,
) (ActorContext, error) {
	switch originKind.Normalize() {
	case OriginKindCLI, OriginKindUDS, OriginKindHTTP, OriginKindAgentSession:
	default:
		return ActorContext{}, fmt.Errorf(
			"%w: agent-session task ingress requires cli, uds, http, or agent_session origin, got %q",
			ErrValidation,
			originKind,
		)
	}
	if originRef == "" {
		originRef = sessionRef
	}
	return deriveActorContext(
		ActorKindAgentSession,
		sessionRef,
		originKind,
		originRef,
		agentCallerScope(sessionRef, workspaceID),
	)
}

// DeriveAutomationLinkedAgentSessionActorContext derives one trusted
// agent-session actor context for work created explicitly by an automation-
// launched session. The session remains the immutable actor while the origin is
// anchored to the automation activation that launched it.
func DeriveAutomationLinkedAgentSessionActorContext(
	sessionRef string,
	workspaceID string,
	originRef string,
) (ActorContext, error) {
	if originRef == "" {
		originRef = sessionRef
	}
	return deriveActorContext(
		ActorKindAgentSession,
		sessionRef,
		OriginKindAutomation,
		originRef,
		agentCallerScope(sessionRef, workspaceID),
	)
}

// DeriveAutomationActorContext derives one trusted automation actor context.
// If originRef is empty, the actor ref is reused as the durable origin ref.
func DeriveAutomationActorContext(actorRef string, originRef string) (ActorContext, error) {
	if originRef == "" {
		originRef = actorRef
	}
	return deriveActorContext(ActorKindAutomation, actorRef, OriginKindAutomation, originRef, CallerScope{})
}

// DeriveExtensionActorContext derives one trusted extension actor context. If
// originRef is empty, the actor ref is reused as the durable origin ref.
func DeriveExtensionActorContext(actorRef string, originRef string) (ActorContext, error) {
	if originRef == "" {
		originRef = actorRef
	}
	return deriveActorContext(ActorKindExtension, actorRef, OriginKindExtension, originRef, CallerScope{})
}

// DeriveNetworkPeerActorContext derives one trusted network-peer actor
// context. If originRef is empty, the actor ref is reused as the durable origin
// ref so ingress layers may include peer or peer/channel details as needed.
func DeriveNetworkPeerActorContext(actorRef string, originRef string) (ActorContext, error) {
	if originRef == "" {
		originRef = actorRef
	}
	return deriveActorContext(ActorKindNetworkPeer, actorRef, OriginKindNetwork, originRef, CallerScope{})
}

// DeriveDaemonActorContext derives one trusted daemon-owned actor context. If
// originRef is empty, the actor ref is reused as the durable origin ref.
func DeriveDaemonActorContext(actorRef string, originRef string) (ActorContext, error) {
	if originRef == "" {
		originRef = actorRef
	}
	return deriveActorContext(ActorKindDaemon, actorRef, OriginKindDaemon, originRef, CallerScope{})
}

func deriveActorContext(
	actorKind ActorKind,
	actorRef string,
	originKind OriginKind,
	originRef string,
	scope CallerScope,
) (ActorContext, error) {
	ctx := ActorContext{
		Actor: ActorIdentity{
			Kind: actorKind,
			Ref:  actorRef,
		},
		Origin: Origin{
			Kind: originKind,
			Ref:  originRef,
		},
		Authority: FullAccessAuthority(),
		Scope:     scope,
	}
	if err := ctx.Validate(); err != nil {
		return ActorContext{}, err
	}
	return ctx, nil
}

func agentCallerScope(sessionRef string, workspaceID string) CallerScope {
	return CallerScope{
		SessionID:   strings.TrimSpace(sessionRef),
		WorkspaceID: strings.TrimSpace(workspaceID),
	}
}

func validateActorOriginPair(actor ActorIdentity, origin Origin) error {
	switch actor.Kind.Normalize() {
	case ActorKindHuman:
		switch origin.Kind.Normalize() {
		case OriginKindCLI, OriginKindWeb, OriginKindUDS, OriginKindHTTP:
			return nil
		}
	case ActorKindAgentSession:
		switch origin.Kind.Normalize() {
		case OriginKindAgentSession, OriginKindAutomation, OriginKindCLI, OriginKindUDS, OriginKindHTTP:
			return nil
		}
	case ActorKindAutomation:
		if origin.Kind.Normalize() == OriginKindAutomation {
			return nil
		}
	case ActorKindExtension:
		if origin.Kind.Normalize() == OriginKindExtension {
			return nil
		}
	case ActorKindNetworkPeer:
		if origin.Kind.Normalize() == OriginKindNetwork {
			return nil
		}
	case ActorKindDaemon:
		if origin.Kind.Normalize() == OriginKindDaemon {
			return nil
		}
	default:
		return fmt.Errorf("%w: actor.kind has unsupported value %q", ErrValidation, actor.Kind)
	}

	return fmt.Errorf(
		"%w: actor.kind %q is not allowed with origin.kind %q",
		ErrValidation,
		actor.Kind.Normalize(),
		origin.Kind.Normalize(),
	)
}
