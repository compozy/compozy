package session

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/store"
)

type WorkCandidate struct {
	Repository     string `json:"repository"`
	HeadRepository string `json:"head_repository,omitempty"`
	Branch         string `json:"branch"`
	PRNumber       int    `json:"pr_number,omitempty"`
	Title          string `json:"title,omitempty"`
	URL            string `json:"url,omitempty"`
	State          string `json:"state,omitempty"`
}
type WorkCandidates struct {
	Items               []WorkCandidate `json:"items"`
	HasMore             bool            `json:"has_more"`
	SupportsPRDiscovery bool            `json:"supports_pr_discovery"`
}
type WorkCandidateReader interface {
	WorkCandidates(context.Context, string, string, string, int, int) (WorkCandidates, error)
	WorkPR(context.Context, string, int) (WorkCandidate, error)
}

func (m *Manager) SessionWorkCandidates(
	ctx context.Context,
	id, query, kind string,
	page, limit int,
) (WorkCandidates, error) {
	info, err := m.Status(ctx, id)
	if err != nil {
		return WorkCandidates{}, err
	}
	if info == nil {
		return WorkCandidates{}, ErrSessionNotFound
	}
	m.mu.RLock()
	reader, ok := m.pullRequestReader.(WorkCandidateReader)
	m.mu.RUnlock()
	if !ok {
		return WorkCandidates{Items: []WorkCandidate{}}, nil
	}
	return reader.WorkCandidates(ctx, info.WorkspaceID, query, kind, max(1, page), min(100, max(1, limit)))
}
func (m *Manager) resolveWorkNumber(ctx context.Context, info *Info, link *store.SessionWorkLink) error {
	if link.PRNumber == 0 {
		return nil
	}
	m.mu.RLock()
	reader, ok := m.pullRequestReader.(WorkCandidateReader)
	m.mu.RUnlock()
	if !ok {
		return errors.New("session: pull request discovery unavailable")
	}
	candidate, err := reader.WorkPR(ctx, info.WorkspaceID, link.PRNumber)
	if err != nil {
		return err
	}
	if candidate.HeadRepository != "" && candidate.HeadRepository != link.Repository {
		return ErrLinkedWorkForkUnsupported
	}
	if candidate.Repository != link.Repository {
		return ErrLinkedWorkRepository
	}
	if link.Branch != "" && link.Branch != candidate.Branch {
		return ErrLinkedWorkInvalid
	}
	link.Branch = candidate.Branch
	return nil
}
