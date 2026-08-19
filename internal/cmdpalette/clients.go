package cmdpalette

import (
	"context"
	"errors"
	"sort"
)

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
	sort.Slice(result, func(left, right int) bool { return result[left].ID < result[right].ID })
	return result, nil
}
