package daemon

import (
	"context"
	"errors"
	"fmt"

	"slices"
	"sort"
	"strings"

	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

type existingReviewCandidate struct {
	info  *session.Info
	score int
}

func (r *reviewRouter) selectExistingRoute(
	ctx context.Context,
	taskRecord taskpkg.Task,
	review *taskpkg.ReviewProfile,
	original originalWorkerIdentity,
	requester originalWorkerIdentity,
	resolved *workspacepkg.ResolvedWorkspace,
) (*session.Info, error) {
	infos, err := r.sessions.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("daemon: list reviewer sessions: %w", err)
	}
	candidates := make([]existingReviewCandidate, 0, len(infos))
	for _, info := range infos {
		if info == nil {
			continue
		}
		score, ok, err := r.existingCandidateScore(ctx, taskRecord, review, original, requester, resolved, info)
		if err != nil {
			return nil, err
		}
		if ok {
			candidates = append(candidates, existingReviewCandidate{info: info, score: score})
		}
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	sort.SliceStable(candidates, func(i int, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return strings.TrimSpace(candidates[i].info.ID) < strings.TrimSpace(candidates[j].info.ID)
	})
	return candidates[0].info, nil
}

func (r *reviewRouter) existingCandidateScore(
	ctx context.Context,
	taskRecord taskpkg.Task,
	review *taskpkg.ReviewProfile,
	original originalWorkerIdentity,
	requester originalWorkerIdentity,
	resolved *workspacepkg.ResolvedWorkspace,
	info *session.Info,
) (int, bool, error) {
	if info.State != session.StateActive {
		return 0, false, nil
	}
	if strings.TrimSpace(taskRecord.WorkspaceID) != "" &&
		strings.TrimSpace(info.WorkspaceID) != strings.TrimSpace(taskRecord.WorkspaceID) {
		return 0, false, nil
	}
	if r.isOriginalWorker(info, original) || r.isOriginalWorker(info, requester) {
		return 0, false, nil
	}
	agentName := strings.TrimSpace(info.AgentName)
	channelID := strings.TrimSpace(info.NetworkParticipation.ChannelID)
	peerID := reviewRouterPeerID(info)
	if !selectorAllows(review.AgentName, review.AllowedAgentNames, agentName) ||
		!selectorAllows("", review.AllowedChannelIDs, channelID) ||
		!selectorAllows("", review.AllowedPeerIDs, peerID) {
		return 0, false, nil
	}
	if ok, err := r.agentHasCapabilities(ctx, resolved, agentName, review.RequiredCapabilities); err != nil {
		if errors.Is(err, workspacepkg.ErrAgentNotAvailable) {
			return 0, false, nil
		}
		return 0, false, err
	} else if !ok {
		return 0, false, nil
	}
	score := 0
	if slices.Contains(review.PreferredAgentNames, agentName) || strings.TrimSpace(review.AgentName) == agentName {
		score += 4
	}
	if slices.Contains(review.PreferredPeerIDs, peerID) {
		score += 3
	}
	if slices.Contains(review.PreferredChannelIDs, channelID) {
		score += 2
	}
	if ok, err := r.agentHasCapabilities(ctx, resolved, agentName, review.PreferredCapabilities); err != nil {
		if errors.Is(err, workspacepkg.ErrAgentNotAvailable) {
			return score, true, nil
		}
		return 0, false, err
	} else if ok && len(review.PreferredCapabilities) > 0 {
		score++
	}
	return score, true, nil
}
