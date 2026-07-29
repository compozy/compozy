package workspaceaccess

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var workspaceIDPattern = regexp.MustCompile(`^(?:ws_[0-9a-f]{16}|[0-9A-HJ-KMNP-TV-Z]{26})$`)

func normalizeRequest(req Request) Request {
	req.Actor.SessionID = strings.TrimSpace(req.Actor.SessionID)
	req.Actor.WorkspaceID = strings.TrimSpace(req.Actor.WorkspaceID)
	req.Actor.AgentName = strings.TrimSpace(req.Actor.AgentName)
	req.TargetWorkspaceID = strings.TrimSpace(req.TargetWorkspaceID)
	return req
}

func validateRequest(ctx context.Context, req Request) error {
	if ctx == nil {
		return errors.New("workspace access: context is required")
	}
	if !validActorKind(req.Actor.Kind) {
		return fmt.Errorf("workspace access: unknown actor kind %q", req.Actor.Kind)
	}
	if !workspaceIDPattern.MatchString(req.TargetWorkspaceID) {
		return fmt.Errorf("workspace access: target workspace id %q is not canonical", req.TargetWorkspaceID)
	}
	if !validSeam(req.Seam) {
		return fmt.Errorf("workspace access: unknown seam %q", req.Seam)
	}
	if req.Actor.Kind == ActorAgentSession && req.Actor.SessionID == "" {
		return errors.New("workspace access: agent session id is required")
	}
	return nil
}

func validActorKind(kind ActorKind) bool {
	switch kind {
	case ActorAgentSession, ActorHuman, ActorExtension, ActorAutomation, ActorNetworkPeer, ActorDaemon:
		return true
	default:
		return false
	}
}

func validSeam(seam Seam) bool {
	switch seam {
	case SeamIdentity, SeamTask, SeamTool, SeamSpawn, SeamCoordination:
		return true
	default:
		return false
	}
}
