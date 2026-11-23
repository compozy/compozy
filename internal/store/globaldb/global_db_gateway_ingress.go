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
	"github.com/compozy/compozy/internal/ingress"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ ingress.Store = (*GatewayRepo)(nil)

// ResolveIngressSubject loads scope and routing from the canonical subject table.
func (g *GatewayRepo) ResolveIngressSubject(
	ctx context.Context,
	ref ingress.SubjectRef,
) (ingress.Subject, error) {
	if err := g.checkReady(ctx, "resolve gateway ingress subject"); err != nil {
		return ingress.Subject{}, err
	}
	return resolveGatewayIngressSubject(ctx, g.readQueries, ref)
}

func resolveGatewayIngressSubject(
	ctx context.Context,
	queries *sqlcgen.Queries,
	ref ingress.SubjectRef,
) (ingress.Subject, error) {
	if err := ref.Validate(); err != nil {
		return ingress.Subject{}, err
	}
	ref.ID = strings.TrimSpace(ref.ID)
	switch ref.Kind {
	case ingress.SubjectWebhookTrigger:
		row, err := queries.GetGatewayWebhookIngressSubject(ctx, ref.ID)
		if errors.Is(err, sql.ErrNoRows) {
			return ingress.Subject{}, ingress.ErrSubjectNotFound
		}
		if err != nil {
			return ingress.Subject{}, fmt.Errorf("store: resolve webhook ingress subject: %w", err)
		}
		endpoint, err := automationmodel.FormatWebhookEndpoint(row.EndpointSlug, row.WebhookID)
		if err != nil {
			return ingress.Subject{}, fmt.Errorf("store: format webhook ingress endpoint: %w", err)
		}
		prefix := "/api/webhooks/global"
		if row.ScopeKind == string(ingress.ScopeWorkspace) {
			prefix = "/api/webhooks/workspaces/" + url.PathEscape(row.WorkspaceID.String)
		}
		subject, err := gatewayIngressSubject(ref, row.ScopeKind, row.WorkspaceID, prefix+"/"+endpoint)
		if err != nil {
			return ingress.Subject{}, err
		}
		profileID := strings.TrimSpace(row.ProfileID)
		if profileID == "" {
			return ingress.Subject{}, errors.New("store: ingress subject profile is unavailable")
		}
		subject.ProfileID = profileID
		return subject, nil
	default:
		return ingress.Subject{}, ingress.ErrSubjectNotFound
	}
}

func gatewayIngressSubject(
	ref ingress.SubjectRef,
	scope string,
	workspaceID sql.NullString,
	callbackPath string,
) (ingress.Subject, error) {
	scope, workspaceID, err := gatewayIngressDomainScope(scope, workspaceID)
	if err != nil {
		return ingress.Subject{}, err
	}
	subject := ingress.Subject{
		SubjectRef:  ref,
		Scope:       ingress.ScopeKind(scope),
		WorkspaceID: strings.TrimSpace(workspaceID.String),
		Path:        callbackPath,
	}
	switch subject.Scope {
	case ingress.ScopeGlobal:
		if subject.WorkspaceID != "" {
			return ingress.Subject{}, errors.New("store: global ingress subject has a workspace")
		}
	case ingress.ScopeWorkspace:
		if subject.WorkspaceID == "" {
			return ingress.Subject{}, errors.New("store: workspace ingress subject has no workspace")
		}
	default:
		return ingress.Subject{}, fmt.Errorf("store: invalid ingress subject scope %q", subject.Scope)
	}
	return subject, nil
}

func gatewayIngressDomainScope(scope string, workspaceID sql.NullString) (string, sql.NullString, error) {
	switch resources.ResourceScopeKind(scope) {
	case resources.ResourceScopeKindUser, resources.ResourceScopeKindProfile:
		return string(ingress.ScopeGlobal), sql.NullString{}, nil
	case resources.ResourceScopeKindWorkspace:
		return string(ingress.ScopeWorkspace), workspaceID, nil
	case resources.ResourceScopeKindWorkspaceProfile:
		resourceScope := resources.ResourceScope{
			Kind: resources.ResourceScopeKindWorkspaceProfile,
			ID:   workspaceID.String,
		}.Normalize()
		if err := resourceScope.Validate("ingress_subject.scope"); err != nil {
			return "", sql.NullString{}, fmt.Errorf("store: invalid ingress resource scope: %w", err)
		}
		workspace, _, _ := strings.Cut(resourceScope.ID, "@pf:")
		return string(ingress.ScopeWorkspace), sql.NullString{String: workspace, Valid: true}, nil
	default:
		return scope, workspaceID, nil
	}
}

const ingressEndpointTier = "public"

// GetIngressEndpoint loads the durable public address identity.
func (g *GatewayRepo) GetIngressEndpoint(
	ctx context.Context,
) (ingress.EndpointState, error) {
	if err := g.checkReady(ctx, "load gateway ingress endpoint"); err != nil {
		return ingress.EndpointState{}, err
	}
	row, err := g.readQueries.GetGatewayEndpointState(ctx, ingressEndpointTier)
	if errors.Is(err, sql.ErrNoRows) {
		return ingress.EndpointState{}, ingress.ErrEndpointUnverified
	}
	if err != nil {
		return ingress.EndpointState{}, fmt.Errorf("store: load gateway ingress endpoint: %w", err)
	}
	return gatewayEndpointStateFromGenerated(row)
}

// SyncIngressEndpoint preserves generation for a stable URL and advances it on change.
func (g *GatewayRepo) SyncIngressEndpoint(
	ctx context.Context,
	endpointURL string,
	now time.Time,
) (_ ingress.EndpointState, changed bool, err error) {
	if err := g.checkReady(ctx, "sync gateway ingress endpoint"); err != nil {
		return ingress.EndpointState{}, false, err
	}
	endpointURL = strings.TrimSpace(endpointURL)
	if endpointURL == "" {
		return ingress.EndpointState{}, false, ingress.ErrEndpointUnverified
	}
	if now.IsZero() {
		return ingress.EndpointState{}, false, errors.New("store: gateway endpoint timestamp is required")
	}
	var state ingress.EndpointState
	if err := store.ExecuteWrite(ctx, g.db, func(writeCtx context.Context, tx *store.WriteTx) error {
		queries := sqlcgen.New(tx)
		previous, previousErr := queries.GetGatewayEndpointState(writeCtx, ingressEndpointTier)
		if previousErr != nil && !errors.Is(previousErr, sql.ErrNoRows) {
			return fmt.Errorf("store: read prior ingress endpoint: %w", previousErr)
		}
		row, writeErr := queries.UpsertGatewayEndpointState(
			writeCtx,
			sqlcgen.UpsertGatewayEndpointStateParams{
				Tier: ingressEndpointTier, EndpointUrl: endpointURL, UpdatedAt: store.FormatTimestamp(now),
			},
		)
		if writeErr != nil {
			return fmt.Errorf("store: upsert gateway ingress endpoint: %w", writeErr)
		}
		state, writeErr = gatewayEndpointStateFromGenerated(row)
		if writeErr != nil {
			return writeErr
		}
		changed = errors.Is(previousErr, sql.ErrNoRows) || previous.EndpointUrl != row.EndpointUrl
		return nil
	}); err != nil {
		return ingress.EndpointState{}, false, fmt.Errorf("store: sync gateway ingress endpoint: %w", err)
	}
	return state, changed, nil
}

func (g *GatewayRepo) MarkIngressEndpointUnavailable(ctx context.Context, now time.Time) error {
	if err := g.checkReady(ctx, "mark gateway ingress endpoint unavailable"); err != nil {
		return err
	}
	if now.IsZero() {
		return errors.New("store: gateway endpoint timestamp is required")
	}
	if err := store.ExecuteWrite(ctx, g.db, func(writeCtx context.Context, tx *store.WriteTx) error {
		queries := sqlcgen.New(tx)
		if _, writeErr := queries.MarkGatewayEndpointUnavailable(
			writeCtx,
			sqlcgen.MarkGatewayEndpointUnavailableParams{
				Tier: ingressEndpointTier, UpdatedAt: store.FormatTimestamp(now),
			},
		); writeErr != nil {
			return fmt.Errorf("store: mark gateway ingress endpoint unavailable: %w", writeErr)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("store: update gateway ingress endpoint availability: %w", err)
	}
	return nil
}

func gatewayEndpointStateFromGenerated(row sqlcgen.GatewayEndpointState) (ingress.EndpointState, error) {
	generation, err := positiveGatewayUnsignedValue("endpoint generation", row.EndpointGeneration)
	if err != nil {
		return ingress.EndpointState{}, err
	}
	updatedAt, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return ingress.EndpointState{}, fmt.Errorf("store: parse gateway endpoint timestamp: %w", err)
	}
	return ingress.EndpointState{
		URL:        row.EndpointUrl,
		Generation: generation, Reachable: row.Reachable, UpdatedAt: updatedAt,
	}, nil
}
