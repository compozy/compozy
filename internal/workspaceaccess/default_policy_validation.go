package workspaceaccess

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

var workspaceIDPattern = regexp.MustCompile(`^(?:ws_[0-9a-f]{16}|[0-9A-HJ-KMNP-TV-Z]{26})$`)

func normalizeRequest(req Request) Request {
	req.Actor.SessionID = strings.TrimSpace(req.Actor.SessionID)
	req.Actor.WorkspaceID = strings.TrimSpace(req.Actor.WorkspaceID)
	req.Actor.AgentName = strings.TrimSpace(req.Actor.AgentName)
	req.Target.WorkspaceID = strings.TrimSpace(req.Target.WorkspaceID)
	req.Target.ProfileID = strings.TrimSpace(req.Target.ProfileID)
	req.Target.Surface = strings.TrimSpace(req.Target.Surface)
	return req
}

func validateRequest(ctx context.Context, req Request) error {
	if ctx == nil {
		return invalidRequest("workspace access: context is required")
	}
	if !validActorKind(req.Actor.Kind) {
		return invalidRequest(fmt.Sprintf("workspace access: unknown actor kind %q", req.Actor.Kind))
	}
	if req.Target.AllProfiles {
		if !req.ReadOnly || req.Target.ProfileID != "" {
			return invalidRequest("workspace access: all profiles requires a read without a concrete profile")
		}
	} else if req.Target.ProfileID == "" && req.Target.Kind != TargetUnresolved &&
		(req.Actor.Kind != ActorAgentSession || req.Actor.Operator) {
		// An agent session that names no profile acts in its own; authorize resolves it.
		return invalidRequest("workspace access: target profile is required")
	}
	if err := validateTarget(req.Target); err != nil {
		return err
	}
	if !validSeam(req.Seam) {
		return invalidRequest(fmt.Sprintf("workspace access: unknown seam %q", req.Seam))
	}
	if req.Actor.Kind == ActorAgentSession && req.Actor.SessionID == "" {
		return invalidRequest("workspace access: agent session id is required")
	}
	return nil
}

func validActorKind(kind ActorKind) bool {
	switch kind {
	case ActorAgentSession, ActorHuman, ActorExtension, ActorAutomation, ActorDaemon:
		return true
	default:
		return false
	}
}

func validSeam(seam Seam) bool {
	switch seam {
	case SeamIdentity, SeamTask, SeamTool, SeamSpawn, SeamCatalog:
		return true
	default:
		return false
	}
}

func validateTarget(target TargetRef) error {
	switch target.Kind {
	case TargetWorkspace:
		if !workspaceIDPattern.MatchString(target.WorkspaceID) {
			return invalidRequest(
				fmt.Sprintf("workspace access: target workspace id %q is not canonical", target.WorkspaceID),
			)
		}
	case TargetUnresolved:
		if target.WorkspaceID != "" || target.ProfileID != "" || target.AllProfiles {
			return invalidRequest("workspace access: unresolved target cannot carry an owner")
		}
	case TargetUser, TargetAggregate:
		if target.WorkspaceID != "" {
			return invalidRequest("workspace access: non-workspace target cannot carry a workspace id")
		}
		if target.Kind == TargetAggregate && target.Surface == "" {
			return invalidRequest("workspace access: aggregate surface is required")
		}
	default:
		return invalidRequest("workspace access: target kind is required")
	}
	return nil
}
