package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"

	automationmodel "github.com/compozy/compozy/internal/automation/model"
	"github.com/compozy/compozy/internal/ingress"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ ingress.Store = (*IngressRepo)(nil)

// ResolveIngressSubject loads scope and routing from the canonical subject table.
func (g *IngressRepo) ResolveIngressSubject(
	ctx context.Context,
	ref ingress.SubjectRef,
) (ingress.Subject, error) {
	if err := g.checkReady(ctx, "resolve ingress subject"); err != nil {
		return ingress.Subject{}, err
	}
	return resolveIngressSubject(ctx, g.readQueries, ref)
}

func resolveIngressSubject(
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
		row, err := queries.GetWebhookIngressSubject(ctx, ref.ID)
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
		subject, err := ingressSubject(ref, row.ScopeKind, row.WorkspaceID, prefix+"/"+endpoint)
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

func ingressSubject(
	ref ingress.SubjectRef,
	scope string,
	workspaceID sql.NullString,
	callbackPath string,
) (ingress.Subject, error) {
	scope, workspaceID, err := ingressDomainScope(scope, workspaceID)
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

func ingressDomainScope(scope string, workspaceID sql.NullString) (string, sql.NullString, error) {
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
