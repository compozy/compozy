package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	bundlepkg "github.com/compozy/agh/internal/bundles"
	"github.com/compozy/agh/internal/resources"
	"github.com/gin-gonic/gin"
)

const (
	resourcesDaemonControlValue = "daemon-control"
	resourcesSystemKey          = "system"
)

var errResourceServiceUnavailable = errors.New("resource service is not configured")

// ResourceServiceConfig configures the shared operator-facing resource service.
type ResourceServiceConfig struct {
	RawStore      resources.RawStore
	CodecRegistry *resources.CodecRegistry
	Actor         resources.MutationActor
	Trigger       func(context.Context, resources.ResourceKind, resources.ReconcileReason) error
}

type operatorResourceService struct {
	rawStore      resources.RawStore
	codecRegistry *resources.CodecRegistry
	actor         resources.MutationActor
	trigger       func(context.Context, resources.ResourceKind, resources.ReconcileReason) error
}

// NewOperatorResourceService constructs the shared desired-state CRUD service
// used by HTTP and UDS handlers. Resource writes validate against registered
// codecs when available and otherwise fall back to the raw kernel semantics so
// the generic control plane remains usable before family codecs land.
func NewOperatorResourceService(cfg *ResourceServiceConfig) (ResourceService, error) {
	if cfg == nil {
		return nil, errors.New("apicore: resource service config is required")
	}
	if cfg.RawStore == nil {
		return nil, errors.New("apicore: resource raw store is required")
	}

	actor := cfg.Actor
	if actor.Kind == "" {
		actor = defaultResourceControlActor()
	}

	return &operatorResourceService{
		rawStore:      cfg.RawStore,
		codecRegistry: cfg.CodecRegistry,
		actor:         actor,
		trigger:       cfg.Trigger,
	}, nil
}

func defaultResourceControlActor() resources.MutationActor {
	return resources.MutationActor{
		Kind: resources.MutationActorKindDaemon,
		ID:   resourcesDaemonControlValue,
		Source: resources.ResourceSource{
			Kind: resources.ResourceSourceKind("daemon"),
			ID:   resourcesSystemKey,
		},
		MaxScope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
	}
}

func (s *operatorResourceService) List(
	ctx context.Context,
	filter resources.ResourceFilter,
) ([]resources.RawRecord, error) {
	return s.rawStore.ListRaw(ctx, s.actor, filter)
}

func (s *operatorResourceService) Get(
	ctx context.Context,
	kind resources.ResourceKind,
	id string,
) (resources.RawRecord, error) {
	return s.rawStore.GetRaw(ctx, s.actor, kind, id)
}

func (s *operatorResourceService) Put(
	ctx context.Context,
	draft resources.RawDraft,
) (resources.RawRecord, error) {
	if err := validateGenericResourceMutationKind(draft.Kind); err != nil {
		return resources.RawRecord{}, err
	}
	specJSON := append([]byte(nil), draft.SpecJSON...)
	if s.codecRegistry != nil {
		canonical, _, err := resources.ValidateAndCanonicalizeIfRegistered(
			ctx,
			s.codecRegistry,
			draft.Kind,
			draft.Scope,
			specJSON,
		)
		if err != nil {
			return resources.RawRecord{}, err
		}
		specJSON = canonical
	}

	next := draft
	next.SpecJSON = specJSON
	record, err := s.rawStore.PutRaw(ctx, s.actor, next)
	if err != nil {
		return resources.RawRecord{}, err
	}
	if s.trigger != nil {
		if err := s.trigger(ctx, next.Kind, resources.ReconcileReasonWrite); err != nil {
			return record, nil
		}
	}
	return record, nil
}

func (s *operatorResourceService) Delete(
	ctx context.Context,
	kind resources.ResourceKind,
	id string,
	expectedVersion int64,
) error {
	if err := validateGenericResourceMutationKind(kind); err != nil {
		return err
	}
	if err := s.rawStore.DeleteRaw(ctx, s.actor, kind, id, expectedVersion); err != nil {
		return err
	}
	if s.trigger != nil {
		if err := s.trigger(ctx, kind, resources.ReconcileReasonWrite); err != nil {
			return nil
		}
	}
	return nil
}

func validateGenericResourceMutationKind(kind resources.ResourceKind) error {
	if kind.Normalize() == bundlepkg.BundleActivationResourceKind {
		return fmt.Errorf(
			"%w: resource kind %q must be mutated through the bundle activation service",
			resources.ErrDirectMutationNotAllowed,
			kind.Normalize(),
		)
	}
	return nil
}

// ListResources lists desired-state resources under the shared operator service.
func (h *BaseHandlers) ListResources(c *gin.Context) {
	service := h.Resources
	if service == nil {
		h.respondError(c, http.StatusServiceUnavailable, errResourceServiceUnavailable)
		return
	}

	filter, err := ParseResourceFilter(c)
	if err != nil {
		h.respondError(c, statusForResourceRequestError(err), err)
		return
	}

	records, err := service.List(c.Request.Context(), filter)
	if err != nil {
		h.respondError(c, StatusForResourceError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.ResourcesResponse{Records: ResourceRecordPayloadsFromRaw(records)})
}

// GetResource returns one desired-state resource by kind and id.
func (h *BaseHandlers) GetResource(c *gin.Context) {
	service := h.Resources
	if service == nil {
		h.respondError(c, http.StatusServiceUnavailable, errResourceServiceUnavailable)
		return
	}

	kind, id, err := parseResourcePath(c)
	if err != nil {
		h.respondError(c, statusForResourceRequestError(err), err)
		return
	}

	record, err := service.Get(c.Request.Context(), kind, id)
	if err != nil {
		h.respondError(c, StatusForResourceError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.ResourceResponse{Record: ResourceRecordPayloadFromRaw(record)})
}

// PutResource creates or updates one desired-state resource.
func (h *BaseHandlers) PutResource(c *gin.Context) {
	service := h.Resources
	if service == nil {
		h.respondError(c, http.StatusServiceUnavailable, errResourceServiceUnavailable)
		return
	}

	kind, id, err := parseResourcePath(c)
	if err != nil {
		h.respondError(c, statusForResourceRequestError(err), err)
		return
	}

	var req contract.PutResourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode resource put request: %w", h.transportName(), err),
		)
		return
	}

	draft, err := parseResourcePutDraft(kind, id, req)
	if err != nil {
		h.respondError(c, statusForResourceRequestError(err), err)
		return
	}

	record, err := service.Put(c.Request.Context(), draft)
	if err != nil {
		h.respondError(c, StatusForResourceError(err), err)
		return
	}

	status := http.StatusOK
	if draft.ExpectedVersion == 0 {
		status = http.StatusCreated
	}
	c.JSON(status, contract.ResourceResponse{Record: ResourceRecordPayloadFromRaw(record)})
}

// DeleteResource deletes one desired-state resource by optimistic version.
func (h *BaseHandlers) DeleteResource(c *gin.Context) {
	service := h.Resources
	if service == nil {
		h.respondError(c, http.StatusServiceUnavailable, errResourceServiceUnavailable)
		return
	}

	kind, id, err := parseResourcePath(c)
	if err != nil {
		h.respondError(c, statusForResourceRequestError(err), err)
		return
	}

	var req contract.DeleteResourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode resource delete request: %w", h.transportName(), err),
		)
		return
	}
	if req.ExpectedVersion <= 0 {
		err := fmt.Errorf("%w: expected_version must be positive", resources.ErrValidation)
		h.respondError(c, statusForResourceRequestError(err), err)
		return
	}

	if err := service.Delete(c.Request.Context(), kind, id, req.ExpectedVersion); err != nil {
		h.respondError(c, StatusForResourceError(err), err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ParseResourceFilter parses the shared `/api/resources` list filters.
func ParseResourceFilter(c *gin.Context) (resources.ResourceFilter, error) {
	if c == nil {
		return resources.ResourceFilter{}, fmt.Errorf(
			"%w: resource filter context is required",
			resources.ErrValidation,
		)
	}

	limit, err := ParseOptionalInt(c.Query("limit"))
	if err != nil {
		return resources.ResourceFilter{}, err
	}

	pathKind := strings.TrimSpace(c.Param("kind"))
	queryKind := strings.TrimSpace(c.Query("kind"))
	switch {
	case pathKind != "" && queryKind != "" && pathKind != queryKind:
		return resources.ResourceFilter{}, fmt.Errorf(
			"%w: resource kind path %q does not match query %q",
			resources.ErrValidation,
			pathKind,
			queryKind,
		)
	case pathKind != "":
		queryKind = pathKind
	}

	filter := resources.ResourceFilter{
		Kind:  resources.ResourceKind(queryKind),
		Limit: limit,
	}
	if filter.Kind != "" {
		if err := filter.Kind.Validate("filter.kind"); err != nil {
			return resources.ResourceFilter{}, err
		}
	}

	scope, hasScope, err := parseResourceScopeQuery(c.Query("scope_kind"), c.Query("scope_id"), "filter.scope")
	if err != nil {
		return resources.ResourceFilter{}, err
	}
	if hasScope {
		filter.Scope = &scope
	}

	owner, hasOwner, err := parseResourceOwnerQuery(c.Query("owner_kind"), c.Query("owner_id"), "filter.owner")
	if err != nil {
		return resources.ResourceFilter{}, err
	}
	if hasOwner {
		filter.Owner = &owner
	}

	source, hasSource, err := parseResourceSourceQuery(c.Query("source_kind"), c.Query("source_id"), "filter.source")
	if err != nil {
		return resources.ResourceFilter{}, err
	}
	if hasSource {
		filter.Source = &source
	}

	return filter, nil
}

// ResourceRecordPayloadsFromRaw converts raw resource records into shared transport DTOs.
func ResourceRecordPayloadsFromRaw(records []resources.RawRecord) []contract.ResourceRecordPayload {
	if len(records) == 0 {
		return []contract.ResourceRecordPayload{}
	}

	payloads := make([]contract.ResourceRecordPayload, 0, len(records))
	for _, record := range records {
		payloads = append(payloads, ResourceRecordPayloadFromRaw(record))
	}
	return payloads
}

// ResourceRecordPayloadFromRaw converts one raw resource record into a shared transport DTO.
func ResourceRecordPayloadFromRaw(record resources.RawRecord) contract.ResourceRecordPayload {
	return contract.ResourceRecordPayload{
		Kind:      record.Kind,
		ID:        record.ID,
		Version:   record.Version,
		Scope:     record.Scope,
		Owner:     record.Owner,
		Source:    record.Source,
		Spec:      append(json.RawMessage(nil), record.SpecJSON...),
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}
}

func statusForResourceRequestError(err error) int {
	status := StatusForResourceError(err)
	if status == http.StatusInternalServerError {
		return http.StatusBadRequest
	}
	return status
}

func parseResourcePath(c *gin.Context) (resources.ResourceKind, string, error) {
	kind := resources.ResourceKind(strings.TrimSpace(c.Param("kind")))
	id := strings.TrimSpace(c.Param("id"))

	if err := kind.Validate("kind"); err != nil {
		return "", "", err
	}
	if id == "" {
		return "", "", fmt.Errorf("%w: id is required", resources.ErrValidation)
	}

	return kind, id, nil
}

func parseResourcePutDraft(
	kind resources.ResourceKind,
	id string,
	req contract.PutResourceRequest,
) (resources.RawDraft, error) {
	scope := req.Scope.Normalize()
	if err := scope.Validate("scope"); err != nil {
		return resources.RawDraft{}, err
	}
	if req.ExpectedVersion < 0 {
		return resources.RawDraft{}, fmt.Errorf(
			"%w: expected_version cannot be negative: %d",
			resources.ErrValidation,
			req.ExpectedVersion,
		)
	}

	return resources.RawDraft{
		Kind:            kind,
		ID:              id,
		Scope:           scope,
		ExpectedVersion: req.ExpectedVersion,
		SpecJSON:        append([]byte(nil), req.Spec...),
	}, nil
}

func parseResourceScopeQuery(
	rawKind string,
	rawID string,
	path string,
) (resources.ResourceScope, bool, error) {
	scopeKind := strings.TrimSpace(rawKind)
	scopeID := strings.TrimSpace(rawID)
	if scopeKind == "" && scopeID == "" {
		return resources.ResourceScope{}, false, nil
	}

	scope := resources.ResourceScope{
		Kind: resources.ResourceScopeKind(scopeKind),
		ID:   scopeID,
	}.Normalize()
	if err := scope.Validate(path); err != nil {
		return resources.ResourceScope{}, false, err
	}
	return scope, true, nil
}

func parseResourceOwnerQuery(
	rawKind string,
	rawID string,
	path string,
) (resources.ResourceOwner, bool, error) {
	ownerKind := strings.TrimSpace(rawKind)
	ownerID := strings.TrimSpace(rawID)
	if ownerKind == "" && ownerID == "" {
		return resources.ResourceOwner{}, false, nil
	}

	owner := resources.ResourceOwner{
		Kind: resources.ResourceOwnerKind(ownerKind),
		ID:   ownerID,
	}.Normalize()
	if err := owner.Validate(path); err != nil {
		return resources.ResourceOwner{}, false, err
	}
	return owner, true, nil
}

func parseResourceSourceQuery(
	rawKind string,
	rawID string,
	path string,
) (resources.ResourceSource, bool, error) {
	sourceKind := strings.TrimSpace(rawKind)
	sourceID := strings.TrimSpace(rawID)
	if sourceKind == "" && sourceID == "" {
		return resources.ResourceSource{}, false, nil
	}

	source := resources.ResourceSource{
		Kind: resources.ResourceSourceKind(sourceKind),
		ID:   sourceID,
	}.Normalize()
	if err := source.Validate(path); err != nil {
		return resources.ResourceSource{}, false, err
	}
	return source, true, nil
}
