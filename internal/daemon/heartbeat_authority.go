package daemon

import (
	"context"
	"path/filepath"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/session"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func (e *instanceCheckEnvironment) notifyCheckInAuthority(ctx context.Context, target *heartbeat.CheckTarget) error {
	digest, err := heartbeat.AuthorityDigest(
		target.Policy.Frontmatter.CheckIns,
		heartbeat.AuthorityActions(target.Policy.Frontmatter.Actions),
	)
	if err != nil || digest == "" || target.Instance.MainSessionID == "" {
		return err
	}
	dir, err := filepath.EvalSymlinks(target.DefinitionDir)
	if err != nil {
		return err
	}
	approved, err := e.instances.ApprovedAuthority(ctx, target.Instance.ProfileID, dir)
	if err != nil || approved == digest {
		return err
	}
	first, err := e.instances.MarkAuthorityNotice(ctx, target.Instance.ProfileID, dir, digest)
	if err != nil || !first {
		return err
	}
	_, err = e.sessions.PublishOperatorNotification(
		ctx,
		session.NotifyRequest{
			Kind:        session.NotifyKindHeartbeatAuthorityUnapproved,
			SessionID:   target.Instance.MainSessionID,
			ProfileID:   target.Instance.ProfileID,
			WorkspaceID: target.Instance.WorkspaceID,
			Title:       "Check-in changes need your approval",
			Body:        "Review the check-in authority block for " + target.Instance.AgentName + ".",
		},
	)
	return err
}

func projectInstanceAuthority(
	ctx context.Context,
	instance agentinstance.Instance,
	agent compozyconfig.AgentDef,
	artifacts *session.AgentArtifacts,
	workspace *workspacepkg.ResolvedWorkspace,
	payload *contract.AgentInstancePayload,
	reader instanceAttentionReader,
) {
	payload.AuthorityStatus = "none"
	if agent.SourcePath == "" {
		return
	}
	request := heartbeat.ResolveRequest{
		AgentPath: agent.SourcePath,
		WorkspaceRoot: timerPolicyRoot(
			heartbeat.AuthoringTarget{AgentPath: agent.SourcePath, WorkspaceRoot: workspace.RootDir},
		),
		Config: workspace.Config.Agents.Heartbeat,
		CheckInLimits: &heartbeat.CheckInLimits{
			MaxToolCalls:  workspace.Config.Agents.AlwaysOn.CheckInMaxToolCalls,
			MaxWriteCalls: workspace.Config.Agents.AlwaysOn.CheckInMaxWriteCalls,
		},
	}
	policy, err := heartbeat.Resolve(ctx, request)
	if artifacts != nil && artifacts.PackageOwned {
		policy, err = heartbeat.Parse(
			ctx,
			heartbeat.ParseRequest{
				SourcePath:    artifacts.HeartbeatSourcePath,
				Content:       []byte(artifacts.HeartbeatBody),
				Config:        request.Config,
				CheckInLimits: request.CheckInLimits,
			},
		)
	}
	if err != nil {
		return
	}
	digest, err := heartbeat.AuthorityDigest(
		policy.Frontmatter.CheckIns,
		heartbeat.AuthorityActions(policy.Frontmatter.Actions),
	)
	if err != nil || digest == "" {
		return
	}
	payload.AuthorityStatus = "unapproved"
	approvalReader, ok := reader.(interface {
		ApprovedAuthority(context.Context, string, string) (string, error)
	})
	if !ok {
		return
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(agent.SourcePath))
	if err != nil {
		return
	}
	approved, err := approvalReader.ApprovedAuthority(ctx, instance.ProfileID, dir)
	if err == nil && approved == digest {
		payload.AuthorityStatus = "approved"
	}
}
