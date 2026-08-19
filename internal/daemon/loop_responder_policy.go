package daemon

import (
	"context"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/session"
	storepkg "github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

const maxResponderLineageDepth = 256

type daemonLoopResponderPolicy struct {
	runs     loopResponderRunReader
	sessions loopSessionStatusReader
}

type loopResponderRunReader interface {
	GetLoopRun(context.Context, looppkg.WorkspaceID, looppkg.RunID) (looppkg.Run, error)
}

var _ looppkg.ResponderPolicy = daemonLoopResponderPolicy{}

func (p daemonLoopResponderPolicy) DeniesSelfOperation(
	ctx context.Context,
	workspaceID string,
	runID string,
	actor taskpkg.ActorContext,
) (bool, error) {
	if actor.Actor.Kind.Normalize() != taskpkg.ActorKindAgentSession {
		return false, nil
	}
	workspaceID = strings.TrimSpace(workspaceID)
	actorID := strings.TrimSpace(actor.Actor.Ref)
	if p.runs == nil || p.sessions == nil || workspaceID == "" || actorID == "" ||
		strings.TrimSpace(actor.Scope.WorkspaceID) != workspaceID {
		return true, nil
	}
	run, err := p.runs.GetLoopRun(ctx, looppkg.WorkspaceID(workspaceID), looppkg.RunID(strings.TrimSpace(runID)))
	if err != nil {
		return false, err
	}
	starterID := ""
	if run.StartedBy.Kind.Normalize() == taskpkg.ActorKindAgentSession {
		starterID = strings.TrimSpace(run.StartedBy.Ref)
	}
	visited := make(map[string]struct{}, maxResponderLineageDepth)
	currentID := actorID
	for range maxResponderLineageDepth {
		if currentID == starterID && starterID != "" {
			return true, nil
		}
		if _, exists := visited[currentID]; exists {
			return true, nil
		}
		visited[currentID] = struct{}{}
		info, statusErr := p.sessions.Status(ctx, currentID)
		if statusErr != nil || !validResponderSession(info, currentID, workspaceID) {
			return true, nil
		}
		lineage := storepkg.NormalizeSessionLineage(info.ID, info.Lineage)
		parentID := strings.TrimSpace(lineage.ParentSessionID)
		if parentID == "" {
			return false, nil
		}
		currentID = parentID
	}
	return true, nil
}

func validResponderSession(info *session.Info, sessionID string, workspaceID string) bool {
	return info != nil && strings.TrimSpace(info.ID) == sessionID &&
		strings.TrimSpace(info.WorkspaceID) == workspaceID
}
