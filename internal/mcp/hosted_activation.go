package mcp

import (
	"context"
	"maps"
	"strings"

	"github.com/compozy/compozy/internal/tools"
)

// HostedServedRequest acknowledges the advertised revision used by an actual tools/list.
type HostedServedRequest struct {
	BindID string `json:"bind_id"`
	Digest string `json:"digest"`
}

// ReportServedCatalog accepts receipts only from the authenticated owner of the active bind.
func (s *HostedService) ReportServedCatalog(ctx context.Context, req HostedServedRequest, peer PeerInfo) error {
	record, err := s.recordForBind(ctx, req.BindID, peer)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ownsBindRecord(record) {
		return ErrHostedBindNotFound
	}
	s.binds[record.bindID].servedDigest = strings.TrimSpace(req.Digest)
	return nil
}

// ActivationCatalog resolves current entitlement and the active proxy receipt for one session.
func (s *HostedService) ActivationCatalog(
	ctx context.Context,
	sessionID string,
) (tools.AdvertisedCatalog, string, error) {
	s.mu.Lock()
	launch := s.launches[sessionID]
	if launch == nil || launch.nonceHash == "" {
		s.mu.Unlock()
		return tools.AdvertisedCatalog{}, "", ErrHostedBindNotFound
	}
	record := s.binds[launch.bindID].clone()
	if record == nil && launch.everBound {
		s.mu.Unlock()
		return tools.AdvertisedCatalog{}, "", ErrHostedBindNotFound
	}
	if record == nil {
		record = &hostedBindRecord{
			sessionID:   launch.sessionID,
			profileID:   launch.profileID,
			workspaceID: launch.workspaceID,
			agentName:   launch.agentName,
		}
	}
	served := record.servedDigest
	s.mu.Unlock()
	registry := s.currentRegistry()
	if registry == nil {
		return tools.AdvertisedCatalog{}, "", ErrHostedRegistryRequired
	}
	response, err := buildHostedProjection(ctx, registry, record)
	return response.Catalog, served, err
}

// BoundActivationCatalog returns the exact advertised catalog captured by the initial bind.
func (s *HostedService) BoundActivationCatalog(sessionID string) (tools.AdvertisedCatalog, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	launch := s.launches[sessionID]
	if launch == nil {
		return tools.AdvertisedCatalog{}, false
	}
	record := s.binds[launch.bindID]
	if record == nil || record.initialCatalog.Digest == "" {
		return tools.AdvertisedCatalog{}, false
	}
	return tools.AdvertisedCatalog{
		Digest:        record.initialCatalog.Digest,
		Servers:       maps.Clone(record.initialCatalog.Servers),
		ServerDigests: maps.Clone(record.initialCatalog.ServerDigests),
	}, true
}

// ServedCatalogDigest reads the last accepted tools/list receipt without rebuilding its projection.
func (s *HostedService) ServedCatalogDigest(sessionID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	launch := s.launches[sessionID]
	if launch == nil {
		return ""
	}
	record := s.binds[launch.bindID]
	if record == nil {
		return ""
	}
	return record.servedDigest
}
