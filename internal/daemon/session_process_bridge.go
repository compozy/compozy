package daemon

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/compozy/compozy/internal/procutil"

	"github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/sessionprocess"
	"github.com/compozy/compozy/internal/store"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

type sessionProcessSource struct {
	state *bootState
	home  string
}

func processOwner(info *session.Info) sessionprocess.Owner {
	o := sessionprocess.Owner{
		SessionID:         info.ID,
		WorkspaceID:       info.WorkspaceID,
		ProfileID:         info.ProfileID,
		Name:              info.Name,
		LeaderKernelToken: info.LeaderKernelToken,
		Live:              info.State != session.StateStopped && info.ArchivedAt == nil,
	}
	o.Starting = info.State == session.StateStarting || info.RuntimeStatus == session.RuntimeStatusBinding ||
		info.RuntimeStatus == session.RuntimeStatusRecovering ||
		info.RuntimeStatus == session.RuntimeStatusReconfiguring
	if info.Liveness != nil {
		o.LeaderPID = info.Liveness.SubprocessPID
		if info.Liveness.SubprocessStartedAt != nil {
			o.LeaderStartedAt = *info.Liveness.SubprocessStartedAt
		}
		if info.Liveness.Activity != nil {
			o.ToolBusy = info.Liveness.Activity.CurrentTool != ""
		}
	}
	return o
}
func (b sessionProcessSource) Owners(context.Context) ([]sessionprocess.Owner, error) {
	infos := b.state.sessions.List()
	owners := make([]sessionprocess.Owner, 0, len(infos))
	for _, info := range infos {
		if info != nil {
			owners = append(owners, processOwner(info))
		}
	}
	return owners, nil
}
func (b sessionProcessSource) Owner(ctx context.Context, id string) (sessionprocess.Owner, bool, error) {
	info, err := b.state.sessions.Status(ctx, id)
	if errors.Is(err, session.ErrSessionNotFound) || errors.Is(err, store.ErrSessionNotFound) {
		return sessionprocess.Owner{}, false, nil
	}
	if err != nil {
		return sessionprocess.Owner{}, false, err
	}
	if info == nil {
		return sessionprocess.Owner{}, false, nil
	}
	return processOwner(info), true, nil
}
func (b sessionProcessSource) Terminals(ctx context.Context) ([]sessionprocess.Terminal, error) {
	out := []sessionprocess.Terminal{}
	if b.state.processRegistry == nil {
		return out, nil
	}
	for _, item := range b.state.processRegistry.SessionProcessObservations() {
		record := item.Record
		if record.Owner.TerminalID == "" {
			continue
		}
		owner, ok, err := b.Owner(ctx, record.Owner.SessionID)
		if err != nil {
			return nil, err
		}
		if !ok {
			owner = sessionprocess.Owner{SessionID: record.Owner.SessionID}
		}
		out = append(
			out,
			sessionprocess.Terminal{
				PID:       record.PID,
				StartedAt: record.StartedAt,
				ID:        record.Owner.TerminalID,
				Owner:     owner,
			},
		)
	}
	return out, nil
}
func (b sessionProcessSource) CloseTerminal(ctx context.Context, p sessionprocess.Process) error {
	if b.state.terminals == nil {
		return errors.New("daemon: terminal service unavailable")
	}
	actor := terminalpkg.Actor{Kind: terminalpkg.ActorKindSystem, ID: "session-processes", ProfileID: p.Owner.ProfileID}
	grace, cancel := context.WithTimeout(ctx, 3*time.Second)
	_, err := b.state.terminals.Close(
		grace,
		p.Owner.WorkspaceID,
		terminalpkg.ID(p.TerminalID),
		actor,
		terminalpkg.SignalTERM,
	)
	cancel()
	if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
		current, identityErr := procutil.InspectProcess(p.PID)
		if errors.Is(identityErr, procutil.ErrProcessIdentityGone) {
			return nil
		}
		if identityErr != nil {
			return identityErr
		}
		if !current.StartedAt.Equal(p.StartedAt) {
			return nil
		}
		forced, finish := context.WithTimeout(ctx, time.Second)
		defer finish()
		_, err = b.state.terminals.Close(
			forced,
			p.Owner.WorkspaceID,
			terminalpkg.ID(p.TerminalID),
			actor,
			terminalpkg.SignalKILL,
		)
	}
	return err
}
func (b sessionProcessSource) ProcessesChanged(ctx context.Context, id string) {
	if publisher, ok := b.state.sessions.(sessionCatalogChangePublisher); ok {
		publisher.PublishSessionCatalogChanged(ctx, id)
	}
}

func (b sessionProcessSource) RecordTermination(
	ctx context.Context,
	p sessionprocess.Process,
	result sessionprocess.Result,
) error {
	content, err := json.Marshal(
		map[string]any{
			"owner_profile_id":         p.Owner.ProfileID,
			"owner_workspace_id":       p.Owner.WorkspaceID,
			"pid":                      p.PID,
			"started_at":               p.StartedAt,
			"attribution":              p.Attribution,
			"outcome":                  result.Outcome,
			watchEventsPayloadErrorKey: result.Error,
		},
	)
	if err != nil {
		return err
	}
	outcome := "success"
	if result.Outcome != "terminated" && result.Outcome != "already_exited" {
		outcome = "failure"
	}
	auditProfile := p.Owner.ProfileID
	if b.state.profiles != nil {
		profiles, e := b.state.profiles.ListIdentities(ctx)
		if e != nil {
			return e
		}
		active := false
		for _, profile := range profiles {
			if profile.ID == auditProfile && string(profile.State) == "active" {
				active = true
				break
			}
		}
		// Retain original ownership in the audit content when that profile can no longer receive writes.
		if !active {
			auditProfile = store.DefaultProfileID
		}
	}
	summary := daemonEventSummary(
		store.EventSummary{
			ProfileID:   auditProfile,
			WorkspaceID: p.Owner.WorkspaceID,
			SessionID:   p.Owner.SessionID,
			AgentName:   "session-processes",
			Type:        events.SessionProcessesTerminated,
			Outcome:     outcome,
			Summary:     "Session process termination: " + result.Outcome,
			Timestamp:   time.Now().UTC(),
		},
		content,
	)
	return b.state.registry.WriteEventSummaries(ctx, []store.EventSummary{summary})
}
func newSessionProcessService(ctx context.Context, state *bootState, home string) (*sessionprocess.Service, error) {
	persistence, ok := state.registry.(sessionprocess.Store)
	if !ok {
		return nil, errors.New("daemon: process observation store unavailable")
	}
	service, err := sessionprocess.New(
		ctx,
		sessionProcessSource{state: state, home: home},
		persistence,
		state.logger,
		session.TerminationBudget(state.cfg.Session.Stop)+2*time.Minute,
	)
	if err != nil {
		return nil, fmt.Errorf("daemon: create session process service: %w", err)
	}
	return service, nil
}

// Unknown owners are admitted only with explicit markers for this daemon home and a known profile.
// A bare legacy session marker cannot safely select a profile after its session row is deleted.
func (b sessionProcessSource) OrphanOwner(
	ctx context.Context,
	p procutil.ScannedProcess,
) (sessionprocess.Owner, bool, error) {
	if p.Home == "" || p.Profile == "" || filepath.Clean(p.Home) != filepath.Clean(b.home) || b.state.profiles == nil {
		return sessionprocess.Owner{}, false, nil
	}
	profiles, err := b.state.profiles.ListIdentities(ctx)
	if err != nil {
		return sessionprocess.Owner{}, false, err
	}
	for _, profile := range profiles {
		if profile.ID == p.Profile {
			return sessionprocess.Owner{
				SessionID:   p.SessionID,
				WorkspaceID: p.Workspace,
				ProfileID:   p.Profile,
				Name:        p.SessionID,
			}, true, nil
		}
	}
	return sessionprocess.Owner{}, false, nil
}
