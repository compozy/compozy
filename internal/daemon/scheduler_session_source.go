package daemon

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	contractpkg "github.com/compozy/agh/internal/api/contract"
	schedulerpkg "github.com/compozy/agh/internal/scheduler"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/situation"
)

type schedulerSituationContext interface {
	ContextForSession(context.Context, *session.Info) (contractpkg.AgentContextPayload, error)
}

var _ schedulerSituationContext = (*situation.Service)(nil)

type schedulerSessionSource struct {
	sessions  SessionManager
	situation schedulerSituationContext
	logger    *slog.Logger
}

func (s schedulerSessionSource) Sessions(ctx context.Context) ([]schedulerpkg.SessionSnapshot, error) {
	if s.sessions == nil {
		return nil, errors.New("daemon: scheduler session source requires session manager")
	}
	infos := s.sessions.List()
	snapshots := make([]schedulerpkg.SessionSnapshot, 0, len(infos))
	for _, info := range infos {
		if info == nil {
			continue
		}
		capabilities, capabilityState, err := s.capabilities(ctx, info)
		if err != nil {
			if ctx.Err() != nil {
				return nil, err
			}
			if s.logger != nil {
				s.logger.Warn(
					"scheduler.session_context.error",
					"session_id", info.ID,
					"error", err,
				)
			}
		}
		snapshots = append(snapshots, schedulerpkg.SessionSnapshot{
			ID:              strings.TrimSpace(info.ID),
			AgentName:       strings.TrimSpace(info.AgentName),
			WorkspaceID:     strings.TrimSpace(info.WorkspaceID),
			Channel:         strings.TrimSpace(info.NetworkParticipation.ChannelID),
			Type:            strings.TrimSpace(string(info.Type)),
			State:           strings.TrimSpace(string(info.State)),
			Prompting:       isSchedulerSessionPrompting(s.sessions, info.ID),
			Capabilities:    capabilities,
			CapabilityState: capabilityState,
			CreatedAt:       info.CreatedAt,
		})
	}
	return snapshots, nil
}

func (s schedulerSessionSource) capabilities(
	ctx context.Context,
	info *session.Info,
) ([]string, schedulerpkg.CapabilityState, error) {
	if s.situation == nil {
		return nil, schedulerpkg.CapabilityStateKnown, nil
	}
	payload, err := s.situation.ContextForSession(ctx, info)
	if err != nil {
		return nil, schedulerpkg.CapabilityStateUnknown, err
	}
	capabilities := make([]string, 0, len(payload.Capabilities.Capabilities))
	for _, capability := range payload.Capabilities.Capabilities {
		if id := strings.TrimSpace(capability.ID); id != "" {
			capabilities = append(capabilities, id)
		}
	}
	return capabilities, schedulerpkg.CapabilityStateKnown, nil
}

func isSchedulerSessionPrompting(sessions SessionManager, sessionID string) bool {
	promptState, ok := sessions.(schedulerPromptState)
	return ok && promptState.IsPrompting(strings.TrimSpace(sessionID))
}
