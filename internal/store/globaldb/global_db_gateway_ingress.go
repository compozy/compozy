package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	automationmodel "github.com/compozy/compozy/internal/automation/model"
	bridgecontract "github.com/compozy/compozy/internal/bridges/contract"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ gateway.IngressStore = (*GatewayRepo)(nil)

// ResolveIngressSubject loads scope and routing from the canonical subject table.
func (g *GatewayRepo) ResolveIngressSubject(
	ctx context.Context,
	ref gateway.IngressSubjectRef,
) (gateway.IngressSubject, error) {
	if err := g.checkReady(ctx, "resolve gateway ingress subject"); err != nil {
		return gateway.IngressSubject{}, err
	}
	return resolveGatewayIngressSubject(ctx, g.queries, ref)
}

func resolveGatewayIngressSubject(
	ctx context.Context,
	queries *sqlcgen.Queries,
	ref gateway.IngressSubjectRef,
) (gateway.IngressSubject, error) {
	if err := ref.Validate(); err != nil {
		return gateway.IngressSubject{}, err
	}
	ref.ID = strings.TrimSpace(ref.ID)
	switch ref.Kind {
	case gateway.IngressSubjectWebhookTrigger:
		row, err := queries.GetGatewayWebhookIngressSubject(ctx, ref.ID)
		if errors.Is(err, sql.ErrNoRows) {
			return gateway.IngressSubject{}, gateway.ErrIngressSubjectNotFound
		}
		if err != nil {
			return gateway.IngressSubject{}, fmt.Errorf("store: resolve webhook ingress subject: %w", err)
		}
		endpoint, err := automationmodel.FormatWebhookEndpoint(row.EndpointSlug, row.WebhookID)
		if err != nil {
			return gateway.IngressSubject{}, fmt.Errorf("store: format webhook ingress endpoint: %w", err)
		}
		prefix := "/api/webhooks/global"
		if row.ScopeKind == string(gateway.IngressScopeWorkspace) {
			prefix = "/api/webhooks/workspaces/" + url.PathEscape(row.WorkspaceID.String)
		}
		return gatewayIngressSubject(ref, row.ScopeKind, row.WorkspaceID, prefix+"/"+endpoint)
	case gateway.IngressSubjectBridgeInstance:
		row, err := queries.GetGatewayBridgeIngressSubject(ctx, ref.ID)
		if errors.Is(err, sql.ErrNoRows) {
			return gateway.IngressSubject{}, gateway.ErrIngressSubjectNotFound
		}
		if err != nil {
			return gateway.IngressSubject{}, fmt.Errorf("store: resolve bridge ingress subject: %w", err)
		}
		instance := bridgecontract.BridgeInstance{ProviderConfig: []byte(row.ProviderConfig)}
		if _, err := bridgecontract.WebhookPublicURL(instance); err == nil {
			return gateway.IngressSubject{}, gateway.ErrIngressSubjectNotFound
		}
		subject, err := gatewayIngressSubject(
			ref,
			row.ScopeKind,
			row.WorkspaceID,
			"/api/bridge-callbacks/"+url.PathEscape(ref.ID),
		)
		if err != nil {
			return gateway.IngressSubject{}, err
		}
		if _, err := bridgecontract.WebhookLocalTarget(instance); err != nil {
			subject.TargetUnavailable = true
		}
		return subject, nil
	default:
		return gateway.IngressSubject{}, gateway.ErrIngressSubjectNotFound
	}
}

func gatewayIngressSubject(
	ref gateway.IngressSubjectRef,
	scope string,
	workspaceID sql.NullString,
	callbackPath string,
) (gateway.IngressSubject, error) {
	subject := gateway.IngressSubject{
		IngressSubjectRef: ref,
		Scope:             gateway.IngressScopeKind(scope),
		WorkspaceID:       strings.TrimSpace(workspaceID.String),
		Path:              callbackPath,
	}
	switch subject.Scope {
	case gateway.IngressScopeGlobal:
		if subject.WorkspaceID != "" {
			return gateway.IngressSubject{}, errors.New("store: global ingress subject has a workspace")
		}
	case gateway.IngressScopeWorkspace:
		if subject.WorkspaceID == "" {
			return gateway.IngressSubject{}, errors.New("store: workspace ingress subject has no workspace")
		}
	default:
		return gateway.IngressSubject{}, fmt.Errorf("store: invalid ingress subject scope %q", subject.Scope)
	}
	return subject, nil
}

// GetIngressEndpoint loads the durable address identity for one tier.
func (g *GatewayRepo) GetIngressEndpoint(
	ctx context.Context,
	tier gateway.Tier,
) (gateway.IngressEndpointState, error) {
	if err := g.checkReady(ctx, "load gateway ingress endpoint"); err != nil {
		return gateway.IngressEndpointState{}, err
	}
	row, err := g.queries.GetGatewayEndpointState(ctx, string(tier))
	if errors.Is(err, sql.ErrNoRows) {
		return gateway.IngressEndpointState{}, gateway.ErrEndpointUnverified
	}
	if err != nil {
		return gateway.IngressEndpointState{}, fmt.Errorf("store: load gateway ingress endpoint: %w", err)
	}
	return gatewayEndpointStateFromGenerated(row)
}

// SyncIngressEndpoint preserves generation for a stable URL and advances it on change.
func (g *GatewayRepo) SyncIngressEndpoint(
	ctx context.Context,
	tier gateway.Tier,
	endpointURL string,
	now time.Time,
) (_ gateway.IngressEndpointState, changed bool, err error) {
	if err := g.checkReady(ctx, "sync gateway ingress endpoint"); err != nil {
		return gateway.IngressEndpointState{}, false, err
	}
	if err := tier.Validate(); err != nil {
		return gateway.IngressEndpointState{}, false, err
	}
	endpointURL = strings.TrimSpace(endpointURL)
	if endpointURL == "" {
		return gateway.IngressEndpointState{}, false, gateway.ErrEndpointUnverified
	}
	if now.IsZero() {
		return gateway.IngressEndpointState{}, false, errors.New("store: gateway endpoint timestamp is required")
	}
	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return gateway.IngressEndpointState{}, false, fmt.Errorf("store: begin ingress endpoint sync: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			err = errors.Join(err, rollbackTx(tx, "gateway ingress endpoint sync"))
		}
	}()
	queries := sqlcgen.New(tx)
	previous, previousErr := queries.GetGatewayEndpointState(ctx, string(tier))
	if previousErr != nil && !errors.Is(previousErr, sql.ErrNoRows) {
		return gateway.IngressEndpointState{}, false, fmt.Errorf("store: read prior ingress endpoint: %w", previousErr)
	}
	row, err := queries.UpsertGatewayEndpointState(ctx, sqlcgen.UpsertGatewayEndpointStateParams{
		Tier: string(tier), EndpointUrl: endpointURL, UpdatedAt: store.FormatTimestamp(now),
	})
	if err != nil {
		return gateway.IngressEndpointState{}, false, fmt.Errorf("store: upsert gateway ingress endpoint: %w", err)
	}
	state, err := gatewayEndpointStateFromGenerated(row)
	if err != nil {
		return gateway.IngressEndpointState{}, false, err
	}
	changed = errors.Is(previousErr, sql.ErrNoRows) || previous.EndpointUrl != row.EndpointUrl
	if err := tx.Commit(); err != nil {
		return gateway.IngressEndpointState{}, false, fmt.Errorf("store: commit ingress endpoint sync: %w", err)
	}
	committed = true
	return state, changed, nil
}

func (g *GatewayRepo) MarkIngressEndpointUnavailable(ctx context.Context, tier gateway.Tier, now time.Time) error {
	if err := g.checkReady(ctx, "mark gateway ingress endpoint unavailable"); err != nil {
		return err
	}
	if err := tier.Validate(); err != nil {
		return err
	}
	if now.IsZero() {
		return errors.New("store: gateway endpoint timestamp is required")
	}
	if _, err := g.queries.MarkGatewayEndpointUnavailable(ctx, sqlcgen.MarkGatewayEndpointUnavailableParams{
		Tier: string(tier), UpdatedAt: store.FormatTimestamp(now),
	}); err != nil {
		return fmt.Errorf("store: mark gateway ingress endpoint unavailable: %w", err)
	}
	return nil
}

func gatewayEndpointStateFromGenerated(row sqlcgen.GatewayEndpointState) (gateway.IngressEndpointState, error) {
	if row.EndpointGeneration <= 0 {
		return gateway.IngressEndpointState{}, errors.New("store: invalid gateway endpoint generation")
	}
	updatedAt, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return gateway.IngressEndpointState{}, fmt.Errorf("store: parse gateway endpoint timestamp: %w", err)
	}
	return gateway.IngressEndpointState{
		Tier: gateway.Tier(row.Tier), URL: row.EndpointUrl,
		Generation: uint64(row.EndpointGeneration), Reachable: row.Reachable, UpdatedAt: updatedAt,
	}, nil
}
