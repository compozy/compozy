package cmdpalette

import (
	"cmp"
	"context"
	"errors"
	"slices"
)

// Clients returns the attached shells for a workspace in deterministic id order.
func (s *Service) Clients(ctx context.Context, workspaceID WorkspaceID) ([]Client, error) {
	if ctx == nil {
		return nil, errors.New("cmd palette: clients context is required")
	}
	if workspaceID == "" {
		return nil, errors.New("cmd palette: workspace ID is required")
	}
	if s.clients == nil {
		return []Client{}, nil
	}
	clients, err := s.clients.Clients(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	result := append([]Client(nil), clients...)
	slices.SortFunc(result, func(a, b Client) int {
		return cmp.Compare(a.ID, b.ID)
	})
	return result, nil
}
