package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/admission"
	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

const (
	daemonAcceptedValue = "accepted"
	daemonStoppedValue  = "stopped"
)

type instanceMainAttempts interface {
	InstanceMainAttempt(context.Context, string) (string, error)
}

type instanceMainSessions struct {
	sessions      *session.Manager
	attempts      instanceMainAttempts
	workAdmission admission.Checker
}

var _ agentinstance.MainSessionStarter = instanceMainSessions{}
var _ agentinstance.ReconciliationSessions = instanceMainSessions{}

func (r instanceMainSessions) Start(ctx context.Context, request agentinstance.StartMainSession) (string, error) {
	instance := request.Instance
	outcome, err := r.StartOutcome(ctx, instance)
	if err != nil {
		return "", err
	}
	if outcome.State == daemonAcceptedValue {
		return outcome.SessionID, nil
	}
	if outcome.State == daemonStoppedValue {
		if err := r.DiscardFailedStart(ctx, instance, outcome.SessionID); err != nil {
			return outcome.SessionID, err
		}
		outcome = agentinstance.StartOutcome{State: taskRecoverySessionMissing}
	}
	if outcome.State != taskRecoverySessionMissing {
		return outcome.SessionID, agentinstance.ErrStarting
	}
	opts := session.CreateOpts{
		InstanceID: instance.ID,
		ProfileID:  instance.ProfileID,
		AgentName:  instance.AgentName,
		Workspace:  instance.WorkspaceID,
		Type:       session.SessionTypeUser,
		Origin: workspaceaccess.Origin{
			Kind:    workspaceaccess.ActorDaemon,
			ID:      instance.ID,
			Ceiling: instance.GrantCeiling,
		},
	}
	if instance.WorkspaceID == "" {
		opts.Scope = workspacepkg.ScopeUser
	}
	info, err := r.sessions.CreateAccepted(ctx, new(session.CreateAcceptedOpts{Session: opts}))
	if err != nil {
		id, lookupErr := r.attempts.InstanceMainAttempt(context.WithoutCancel(ctx), instance.ID)
		return id, errors.Join(err, lookupErr)
	}
	return info.ID, nil
}

func (r instanceMainSessions) Stop(ctx context.Context, id string) error {
	info, err := r.sessions.Status(ctx, id)
	if err != nil {
		return err
	}
	if info.State == session.StateStopped {
		return nil
	}
	return r.sessions.Stop(ctx, id)
}

func (r instanceMainSessions) CheckMain(ctx context.Context, id string) error {
	info, err := r.sessions.StoredStatus(ctx, id)
	if err != nil {
		return fmt.Errorf("agentinstance: read main session %s: %w", id, err)
	}
	if mainStoppedByShutdown(info) {
		if r.workAdmission != nil {
			if err := r.workAdmission.Check(ctx); err != nil {
				return fmt.Errorf("agentinstance: check recovery readiness for main session %s: %w", id, err)
			}
		}
		return fmt.Errorf(
			"%w: main session %s remains stopped after daemon shutdown; inspect its restart history",
			session.ErrSessionNotActive, id,
		)
	}
	return nil
}

func mainStoppedByShutdown(info *session.Info) bool {
	return info.State == session.StateStopped && info.StopReason == store.StopShutdown
}

func (r instanceMainSessions) StartOutcome(
	ctx context.Context,
	instance agentinstance.Instance,
) (agentinstance.StartOutcome, error) {
	id, err := r.attempts.InstanceMainAttempt(ctx, instance.ID)
	if err != nil {
		return agentinstance.StartOutcome{}, err
	}
	if id == "" {
		return agentinstance.StartOutcome{State: taskRecoverySessionMissing}, nil
	}
	info, err := r.sessions.Status(ctx, id)
	if err != nil {
		return agentinstance.StartOutcome{}, err
	}
	state := daemonAcceptedValue
	if info.State == session.StateStopped {
		state = daemonStoppedValue
	}
	if info.State == session.StateStarting {
		state = "pending"
	}
	if info.Failure != nil && info.Failure.Kind == store.FailureStartup {
		state = "failed"
	}
	return agentinstance.StartOutcome{SessionID: id, State: state}, nil
}

func (r instanceMainSessions) DiscardFailedStart(
	ctx context.Context,
	instance agentinstance.Instance,
	id string,
) error {
	_, err := r.sessions.ArchiveWithStop(ctx, instance.WorkspaceID, id)
	return err
}
