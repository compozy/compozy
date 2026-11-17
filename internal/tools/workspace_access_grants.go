package tools

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/workspaceaccess"
)

// workspaceAccessScopeAllProfiles stands for every profile in a remembered read scope.
const workspaceAccessScopeAllProfiles = "*"

// WorkspaceAccessReadGrantKey is the durable key an operator "Always allow" leaves for one read
// crossing: the acting agent in its own workspace and profile, the tool, and the target scope.
// The session or turn input never enters the key, so later reads of the same scope match. Writes,
// unnamed agents, and targets that resolve to no owner are never remembered.
func WorkspaceAccessReadGrantKey(
	scope Scope,
	call CallRequest,
	descriptor Descriptor,
	actor workspaceaccess.ActorRef,
	target workspaceaccess.TargetRef,
) *ApprovalGrantKey {
	if !descriptor.ReadOnly {
		return nil
	}
	profileID := strings.TrimSpace(scope.ProfileID)
	accessScope := workspaceAccessReadScope(target, profileID)
	if accessScope == "" {
		return nil
	}
	workspaceID := strings.TrimSpace(call.WorkspaceID)
	if workspaceID == "" {
		workspaceID = strings.TrimSpace(scope.WorkspaceID)
	}
	agentName := strings.TrimSpace(actor.AgentName)
	if agentName == "" {
		agentName = strings.TrimSpace(scope.AgentName)
	}
	if agentName == "" {
		return nil
	}
	key := ApprovalGrantKey{
		ProfileID:   profileID,
		WorkspaceID: workspaceID,
		AgentName:   agentName,
		ToolID:      descriptor.ID,
		InputDigest: workspaceAccessScopeDigest(accessScope),
		AccessScope: accessScope,
	}.Normalize()
	if key.Validate() != nil {
		return nil
	}
	return &key
}

// workspaceAccessReadScope renders the target scope a read grant covers:
// read:workspace:<id>@<profile>, read:user@<profile>, or read:all-projects@<profile>, where
// <profile> is "*" for every profile. An empty result means the target cannot be remembered.
func workspaceAccessReadScope(target workspaceaccess.TargetRef, actorProfileID string) string {
	profile := strings.TrimSpace(target.ProfileID)
	switch {
	case target.AllProfiles:
		profile = workspaceAccessScopeAllProfiles
	case profile == "":
		profile = actorProfileID
	}
	if profile == "" {
		return ""
	}
	switch target.Kind {
	case workspaceaccess.TargetWorkspace:
		workspaceID := strings.TrimSpace(target.WorkspaceID)
		if workspaceID == "" {
			return ""
		}
		return fmt.Sprintf("read:workspace:%s@%s", workspaceID, profile)
	case workspaceaccess.TargetUser:
		return "read:user@" + profile
	case workspaceaccess.TargetAggregate:
		return "read:all-projects@" + profile
	default:
		return ""
	}
}

// workspaceAccessScopeDigest keys the grant row, so each target scope keeps its own decision.
func workspaceAccessScopeDigest(accessScope string) string {
	// This preimage cannot be tool-input JSON, so the two grant domains cannot alias.
	sum := sha256.Sum256([]byte("workspace-access-read\x00" + accessScope))
	return "sha256:" + hex.EncodeToString(sum[:])
}
