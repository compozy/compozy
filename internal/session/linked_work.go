package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

var ErrLinkedWorkRepository = errors.New("linked work belongs to another repository")
var ErrLinkedWorkInvalid = errors.New("linked work requires a branch and repository")

type WorkLinkStore interface {
	SessionWorkFamily(context.Context, []string) ([]store.SessionWorkMember, error)
	SessionWorkLinks(context.Context, []string) (map[string][]store.SessionWorkLink, error)
	SaveSessionWorkLink(context.Context, store.SessionWorkLink, bool) error
	UnlinkSessionWork(context.Context, store.SessionWorkLink) error
	SessionPendingWork(context.Context, []string) (map[string]bool, error)
}

type LinkedWorkItem struct {
	Repository         string `json:"repository"`
	Branch             string `json:"branch"`
	PRNumber           int    `json:"pr_number,omitempty"`
	WorktreeID         string `json:"worktree_id,omitempty"`
	WorktreeName       string `json:"worktree_name,omitempty"`
	WorktreeRemoved    bool   `json:"worktree_removed"`
	Provenance         string `json:"provenance"`
	RunsHere           bool   `json:"runs_here"`
	SourceSessionID    string `json:"source_session_id,omitempty"`
	SourceSessionTitle string `json:"source_session_title,omitempty"`
	PullRequestView
}

type LinkedWorkSummary struct {
	Total    int             `json:"total"`
	NeedsYou int             `json:"needs_you"`
	Lead     *LinkedWorkItem `json:"lead,omitempty"`
}

func (m *Manager) LinkSessionWork(ctx context.Context, sessionID string, link store.SessionWorkLink, automatic bool) error {
	info, err := m.Status(ctx, sessionID)
	if err != nil {
		return err
	}
	if info == nil {
		return ErrSessionNotFound
	}
	m.mu.RLock()
	repoReader, ok := m.pullRequestReader.(PullRequestRepositoryReader)
	links, configured := m.branchSnapshots.(WorkLinkStore)
	m.mu.RUnlock()
	if !ok || !configured {
		return errors.New("session: linked work unavailable")
	}
	repo := repoReader.RepositoryKeys(ctx, store.ReadScope{ProfileID: info.ProfileID}, []string{info.WorkspaceID})[info.WorkspaceID]
	if link.Repository == "" {
		link.Repository = repo
	}
	if repo == "" || link.Repository != repo || link.WorkspaceID != "" && link.WorkspaceID != info.WorkspaceID {
		return ErrLinkedWorkRepository
	}
	if err := m.resolveWorkNumber(ctx, info, &link); err != nil {
		return err
	}
	link.Branch = strings.TrimSpace(link.Branch)
	if link.Branch == "" || link.PRNumber < 0 {
		return ErrLinkedWorkInvalid
	}
	if link.Provenance == "" {
		link.Provenance = "user"
	}
	if link.Provenance != "created" && link.Provenance != "user" && link.Provenance != "agent" {
		return ErrLinkedWorkInvalid
	}
	link.SessionID, link.WorkspaceID = info.ID, info.WorkspaceID
	if err := links.SaveSessionWorkLink(ctx, link, automatic); err != nil {
		return fmt.Errorf("session: link work: %w", err)
	}
	m.PublishSessionCatalogChanged(ctx, info.ID)
	return nil
}

func (m *Manager) UnlinkSessionWork(ctx context.Context, sessionID, repository, branch string) error {
	info, err := m.Status(ctx, sessionID)
	if err != nil {
		return err
	}
	if info == nil {
		return ErrSessionNotFound
	}
	m.mu.RLock()
	links, ok := m.branchSnapshots.(WorkLinkStore)
	m.mu.RUnlock()
	if !ok {
		return errors.New("session: linked work unavailable")
	}
	if strings.TrimSpace(branch) == "" {
		return ErrLinkedWorkInvalid
	}
	if repository == "" {
		m.mu.RLock()
		reader, configured := m.pullRequestReader.(PullRequestRepositoryReader)
		m.mu.RUnlock()
		if !configured {
			return ErrLinkedWorkInvalid
		}
		repository = reader.RepositoryKeys(ctx, store.ReadScope{ProfileID: info.ProfileID}, []string{info.WorkspaceID})[info.WorkspaceID]
	}
	link := store.SessionWorkLink{SessionID: info.ID, WorkspaceID: info.WorkspaceID, Repository: repository, Branch: branch, Provenance: "user"}
	// Persist a tombstone even for the derived current branch.
	if err := links.SaveSessionWorkLink(ctx, link, true); err != nil {
		return fmt.Errorf("session: prepare unlink: %w", err)
	}
	if err := links.UnlinkSessionWork(ctx, link); err != nil {
		return err
	}
	m.PublishSessionCatalogChanged(ctx, info.ID)
	return nil
}

// BoardColumnWithLinkedWork preserves the legacy own-branch rule, including
// unknown sessions, while links require a settled badge and an empty input queue.
func BoardColumnWithLinkedWork(badge Badge, own PullRequestState, linkedOpen, pending bool) BoardColumn {
	if ClassForBadge(badge) == AttentionNeedsYou {
		return BoardNeedsYou
	}
	if pending {
		return BoardWorking
	}
	column := BoardColumnFor(badge, own)
	if column == BoardDone && linkedOpen && (badge == BadgeIdle || badge == BadgeDone || badge == BadgeStopped) {
		return BoardInReview
	}
	return column
}
