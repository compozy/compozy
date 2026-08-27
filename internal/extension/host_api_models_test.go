package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	apicontract "github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/subprocess"
	"github.com/compozy/compozy/internal/testutil"
)

func TestHostAPIModelsListShouldReturnDaemonProjection(t *testing.T) {
	t.Parallel()

	t.Run("Should return daemon projection", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
		available := true
		inputCost := 1.0
		outputCost := 2.0
		cacheReadCost := 0.5
		cacheWriteCost := 3.0
		reasoningCost := 4.0
		defaultEffort := modelcatalog.ReasoningEffortHigh
		thinking := false
		releaseDate := "2026-06-26"
		service := &fakeHostAPIModelCatalogService{
			models: []modelcatalog.Model{
				{
					ProviderID:        "codex",
					ModelID:           "daemon-model",
					DisplayName:       "Daemon Model",
					Available:         &available,
					AvailabilityState: modelcatalog.AvailabilityStateAvailableLive,
					RefreshedAt:       now,
					Sources: []modelcatalog.SourceRef{
						{
							SourceID:    "config",
							SourceKind:  modelcatalog.SourceKindConfig,
							Priority:    modelcatalog.PriorityConfig,
							RefreshedAt: now,
							LastError:   "source failed with OAUTH_TOKEN=oauth-host-secret-token",
						},
					},
					ReasoningEfforts:       []modelcatalog.ReasoningEffort{modelcatalog.ReasoningEffortHigh},
					DefaultReasoningEffort: &defaultEffort,
					ConfigOptions: []modelcatalog.ModelOptionDescriptor{{
						ID: "thinking", Kind: modelcatalog.ModelOptionKindBoolean, CurrentBool: &thinking,
					}},
					TransportBindings:        []modelcatalog.ModelTransportBinding{{Thinking: &thinking}},
					CostInputPerMillion:      &inputCost,
					CostOutputPerMillion:     &outputCost,
					CostCacheReadPerMillion:  &cacheReadCost,
					CostCacheWritePerMillion: &cacheWriteCost,
					CostReasoningPerMillion:  &reasoningCost,
					Curated:                  true,
					Deprecated:               true,
					Hidden:                   true,
					Featured:                 true,
					ReleaseDate:              &releaseDate,
					ReasoningSource:          modelcatalog.ReasoningSourceACP,
					LastError:                "model failed with api_key=sk-host-secret-token",
				},
			},
		}
		handler := NewHostAPIHandler(
			nil,
			nil,
			nil,
			nil,
			WithHostAPIModelCatalogService(service),
			WithHostAPICapabilityChecker(newTestCapabilityChecker(
				"ext",
				SourceUser,
				[]string{"models/list"},
				[]string{"model.read"},
			)),
			WithHostAPINow(func() time.Time { return now }),
		)

		result, err := handler.Handle(
			withHostAPIInstanceKey(testutil.Context(t), InstanceKey{
				Name: "ext", ProfileID: "profile-catalog", WorkspaceID: "workspace-catalog",
			}),
			"ext",
			"models/list",
			json.RawMessage(`{"provider_id":"codex","source_id":"extension:ext-models","include_stale":true}`),
		)
		if err != nil {
			t.Fatalf("Handle(models/list) error = %v, want nil", err)
		}
		payload, ok := result.(apicontract.ProviderModelListResponse)
		if !ok {
			t.Fatalf("Handle(models/list) result = %T, want ProviderModelListResponse", result)
		}
		if len(payload.Models) != 1 {
			t.Fatalf("len(result.Models) = %d, want 1", len(payload.Models))
		}
		model := payload.Models[0]
		if model.ModelID != "daemon-model" || model.Sources[0].SourceID != "config" {
			t.Fatalf("models/list payload = %#v, want daemon projection from model catalog service", model)
		}
		if model.DefaultReasoningEffort == nil || *model.DefaultReasoningEffort != "high" {
			t.Fatalf("models/list default reasoning effort = %#v, want high", model.DefaultReasoningEffort)
		}
		if model.Cost == nil || model.Cost.InputPerMillion == nil || *model.Cost.InputPerMillion != inputCost ||
			model.Cost.OutputPerMillion == nil || *model.Cost.OutputPerMillion != outputCost ||
			model.Cost.CacheReadPerMillion == nil || *model.Cost.CacheReadPerMillion != cacheReadCost ||
			model.Cost.CacheWritePerMillion == nil || *model.Cost.CacheWritePerMillion != cacheWriteCost ||
			model.Cost.ReasoningPerMillion == nil || *model.Cost.ReasoningPerMillion != reasoningCost {
			t.Fatalf("models/list five-rate cost = %#v, want complete daemon projection", model.Cost)
		}
		if !model.Curated || !model.Deprecated || !model.Hidden || !model.Featured ||
			model.ReleaseDate != releaseDate || model.ReasoningSource != modelcatalog.ReasoningSourceACP {
			t.Fatalf("models/list curation metadata = %#v, want complete daemon projection", model)
		}
		if len(model.ConfigOptions) != 1 || model.ConfigOptions[0].ID != "thinking" ||
			model.ConfigOptions[0].CurrentBool == nil || *model.ConfigOptions[0].CurrentBool ||
			len(model.Configurations) != 1 || model.Configurations[0].Thinking == nil ||
			*model.Configurations[0].Thinking {
			t.Fatalf(
				"models/list advanced configuration = %#v / %#v, want thinking=false",
				model.ConfigOptions,
				model.Configurations,
			)
		}
		assertRedactedHostAPIModelPayload(t, model.LastError, "sk-host-secret-token")
		assertRedactedHostAPIModelPayload(t, model.Sources[0].LastError, "oauth-host-secret-token")
		if len(service.listOpts) != 1 {
			t.Fatalf("len(service.listOpts) = %d, want 1", len(service.listOpts))
		}
		opts := service.listOpts[0]
		if opts.ProviderID != "codex" || opts.SourceID != "extension:ext-models" || !opts.IncludeStale {
			t.Fatalf("ListModels opts = %#v, want decoded Host API filters", opts)
		}
		if opts.ExecutionContext.Scope != modelcatalog.ExecutionScopeWorkspace ||
			opts.ExecutionContext.ProfileID != "profile-catalog" ||
			opts.ExecutionContext.WorkspaceID != "workspace-catalog" {
			t.Fatalf("ListModels execution context = %#v, want calling extension scope", opts.ExecutionContext)
		}
	})
}

func TestHostAPIModelsRefreshShouldReturnStatusPayloadOnSourceFailure(t *testing.T) {
	t.Parallel()

	t.Run("Should return status payload on source failure", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 7, 12, 15, 0, 0, time.UTC)
		secret := "sk-host-refresh-secret-token"
		service := &fakeHostAPIModelCatalogService{
			statuses: []modelcatalog.SourceStatus{
				{
					SourceID:     "extension:ext-models",
					SourceKind:   modelcatalog.SourceKindExtension,
					ProviderID:   "codex",
					Priority:     modelcatalog.PriorityExtension,
					LastRefresh:  now,
					LastError:    "extension unavailable api_key=" + secret,
					RefreshState: modelcatalog.RefreshStateFailed,
					Stale:        true,
				},
			},
			refreshErr: fmt.Errorf("%w: api_key=%s", modelcatalog.ErrAllSourcesFailed, secret),
		}
		handler := NewHostAPIHandler(
			nil,
			nil,
			nil,
			nil,
			WithHostAPIModelCatalogService(service),
			WithHostAPICapabilityChecker(newTestCapabilityChecker(
				"ext",
				SourceUser,
				[]string{"models/refresh"},
				[]string{"model.write"},
			)),
			WithHostAPINow(func() time.Time { return now }),
		)

		result, err := handler.Handle(
			withHostAPIInstanceKey(testutil.Context(t), InstanceKey{
				Name: "ext", ProfileID: "profile-refresh", WorkspaceID: "workspace-refresh",
			}),
			"ext",
			"models/refresh",
			json.RawMessage(`{"provider_id":"codex","source_id":"extension:ext-models","force":true}`),
		)
		if err != nil {
			t.Fatalf("Handle(models/refresh) error = %v, want status payload with error field", err)
		}
		payload, ok := result.(apicontract.ProviderModelRefreshResponse)
		if !ok {
			t.Fatalf("Handle(models/refresh) result = %T, want ProviderModelRefreshResponse", result)
		}
		if payload.Error == "" || len(payload.Sources) != 1 || payload.Sources[0].RefreshState != "failed" {
			t.Fatalf("models/refresh payload = %#v, want failed source status and error", payload)
		}
		assertRedactedHostAPIModelPayload(t, payload.Error, secret)
		assertRedactedHostAPIModelPayload(t, payload.Sources[0].LastError, secret)
		if len(service.refreshOpts) != 1 || !service.refreshOpts[0].Force {
			t.Fatalf("Refresh opts = %#v, want force refresh recorded", service.refreshOpts)
		}
		if got := service.refreshOpts[0].ExecutionContext; got.Scope != modelcatalog.ExecutionScopeWorkspace ||
			got.ProfileID != "profile-refresh" || got.WorkspaceID != "workspace-refresh" {
			t.Fatalf("Refresh execution context = %#v, want calling extension scope", got)
		}
	})
}

func TestHostAPIModelsRefreshShouldReturnSuccessfulSourceStatus(t *testing.T) {
	t.Parallel()

	t.Run("Should return successful source status", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 7, 12, 20, 0, 0, time.UTC)
		service := &fakeHostAPIModelCatalogService{
			statuses: []modelcatalog.SourceStatus{
				{
					SourceID:     "extension:ext-models",
					SourceKind:   modelcatalog.SourceKindExtension,
					ProviderID:   "codex",
					Priority:     modelcatalog.PriorityExtension,
					LastRefresh:  now,
					RefreshState: modelcatalog.RefreshStateSucceeded,
					RowCount:     1,
				},
			},
		}
		handler := NewHostAPIHandler(
			nil,
			nil,
			nil,
			nil,
			WithHostAPIModelCatalogService(service),
			WithHostAPICapabilityChecker(newTestCapabilityChecker(
				"ext",
				SourceUser,
				[]string{"models/refresh"},
				[]string{"model.write"},
			)),
		)

		result, err := handler.Handle(
			withHostAPIInstanceKey(testutil.Context(t), InstanceKey{
				Name: "ext", ProfileID: "profile-status", WorkspaceID: "workspace-status",
			}),
			"ext",
			"models/refresh",
			json.RawMessage(`{"provider_id":"codex"}`),
		)
		if err != nil {
			t.Fatalf("Handle(models/refresh) error = %v, want nil", err)
		}
		payload, ok := result.(apicontract.ProviderModelRefreshResponse)
		if !ok {
			t.Fatalf("Handle(models/refresh) result = %T, want ProviderModelRefreshResponse", result)
		}
		if payload.Error != "" || len(payload.Sources) != 1 || payload.Sources[0].RefreshState != "succeeded" {
			t.Fatalf("models/refresh payload = %#v, want successful source status", payload)
		}
	})
}

func TestHostAPIModelsStatusShouldReturnDaemonSourceStatus(t *testing.T) {
	t.Parallel()

	t.Run("Should return daemon source status", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 7, 12, 30, 0, 0, time.UTC)
		service := &fakeHostAPIModelCatalogService{
			statuses: []modelcatalog.SourceStatus{
				{
					SourceID:     "extension:ext-models",
					SourceKind:   modelcatalog.SourceKindExtension,
					ProviderID:   "codex",
					Priority:     modelcatalog.PriorityExtension,
					LastRefresh:  now,
					RefreshState: modelcatalog.RefreshStateSucceeded,
					RowCount:     1,
				},
			},
		}
		handler := NewHostAPIHandler(
			nil,
			nil,
			nil,
			nil,
			WithHostAPIModelCatalogService(service),
			WithHostAPICapabilityChecker(newTestCapabilityChecker(
				"ext",
				SourceUser,
				[]string{"models/status"},
				[]string{"model.read"},
			)),
		)

		result, err := handler.Handle(
			withHostAPIInstanceKey(testutil.Context(t), InstanceKey{
				Name: "ext", ProfileID: "profile-status", WorkspaceID: "workspace-status",
			}),
			"ext",
			"models/status",
			json.RawMessage(`{"provider_id":"codex"}`),
		)
		if err != nil {
			t.Fatalf("Handle(models/status) error = %v, want nil", err)
		}
		payload, ok := result.(apicontract.ProviderModelStatusResponse)
		if !ok {
			t.Fatalf("Handle(models/status) result = %T, want ProviderModelStatusResponse", result)
		}
		if len(payload.Sources) != 1 || payload.Sources[0].SourceID != "extension:ext-models" {
			t.Fatalf("models/status payload = %#v, want extension status", payload)
		}
		if len(service.statusOpts) != 1 || service.statusOpts[0].ProviderID != "codex" {
			t.Fatalf("ListSourceStatus opts = %#v, want codex", service.statusOpts)
		}
		if got := service.statusOpts[0].ExecutionContext; got.Scope != modelcatalog.ExecutionScopeWorkspace ||
			got.ProfileID != "profile-status" || got.WorkspaceID != "workspace-status" {
			t.Fatalf("Status execution context = %#v, want calling extension scope", got)
		}
	})
}

func TestHostAPIModelsListShouldRequirePermission(t *testing.T) {
	t.Parallel()

	t.Run("Should require models list permission", func(t *testing.T) {
		t.Parallel()

		handler := NewHostAPIHandler(
			nil,
			nil,
			nil,
			nil,
			WithHostAPIModelCatalogService(&fakeHostAPIModelCatalogService{}),
			WithHostAPICapabilityChecker(newTestCapabilityChecker(
				"ext",
				SourceUser,
				nil,
			)),
		)

		_, err := handler.Handle(testutil.Context(t), "ext", "models/list", nil)
		if err == nil {
			t.Fatal("Handle(models/list) error = nil, want capability denied")
		}

		rpcErr, rpcErrMatched := errors.AsType[*subprocess.RPCError](err)
		if !rpcErrMatched {
			t.Fatalf("Handle(models/list) error = %T, want *RPCError", err)
		}
		if rpcErr.Code != CapabilityDeniedCode {
			t.Fatalf("RPCError.Code = %d, want %d", rpcErr.Code, CapabilityDeniedCode)
		}
	})
}

func TestHostAPIModelsShouldMapValidationAndAvailabilityErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		method      string
		params      json.RawMessage
		permissions []string
		service     modelcatalog.Service
		wantCode    int
	}{
		{
			name:        "Should reject invalid source id",
			method:      "models/list",
			params:      json.RawMessage(`{"source_id":"bad source"}`),
			permissions: []string{"models/list"},
			service:     &fakeHostAPIModelCatalogService{},
			wantCode:    HostAPIInvalidParamsCode,
		},
		{
			name:        "Should reject invalid provider id",
			method:      "models/list",
			params:      json.RawMessage(`{"provider_id":"Bad"}`),
			permissions: []string{"models/list"},
			service:     &fakeHostAPIModelCatalogService{},
			wantCode:    HostAPIInvalidParamsCode,
		},
		{
			name:        "Should map unregistered source to invalid params",
			method:      "models/list",
			params:      json.RawMessage(`{"source_id":"extension:missing"}`),
			permissions: []string{"models/list"},
			service:     &fakeHostAPIModelCatalogService{listErr: modelcatalog.ErrSourceNotRegistered},
			wantCode:    HostAPIInvalidParamsCode,
		},
		{
			name:        "Should map refresh failure without statuses to unavailable",
			method:      "models/refresh",
			params:      json.RawMessage(`{"source_id":"extension:missing"}`),
			permissions: []string{"models/refresh"},
			service:     &fakeHostAPIModelCatalogService{refreshErr: modelcatalog.ErrAllSourcesFailed},
			wantCode:    HostAPIUnavailableCode,
		},
		{
			name:        "Should map missing status service to unavailable",
			method:      "models/status",
			params:      json.RawMessage(`{}`),
			permissions: []string{"models/status"},
			wantCode:    HostAPIUnavailableCode,
		},
		{
			name:        "Should map status service failure to unavailable",
			method:      "models/status",
			params:      json.RawMessage(`{}`),
			permissions: []string{"models/status"},
			service:     &fakeHostAPIModelCatalogService{statusErr: errors.New("status offline")},
			wantCode:    HostAPIUnavailableCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := []HostAPIOption{
				WithHostAPICapabilityChecker(newTestCapabilityChecker("ext", SourceUser, tt.permissions)),
			}
			if tt.service != nil {
				opts = append(opts, WithHostAPIModelCatalogService(tt.service))
			}
			handler := NewHostAPIHandler(nil, nil, nil, nil, opts...)
			_, err := handler.Handle(testutil.Context(t), "ext", tt.method, tt.params)
			if err == nil {
				t.Fatal("Handle() error = nil, want RPC error")
			}

			rpcErr, rpcErrMatched := errors.AsType[*subprocess.RPCError](err)
			if !rpcErrMatched {
				t.Fatalf("Handle() error = %T, want *RPCError", err)
			}
			if rpcErr.Code != tt.wantCode {
				t.Fatalf("RPCError.Code = %d, want %d", rpcErr.Code, tt.wantCode)
			}
		})
	}
}

func TestHostAPIModelsShouldRedactUnavailableRPCErrorData(t *testing.T) {
	t.Parallel()

	t.Run("Should redact unavailable RPC error data", func(t *testing.T) {
		t.Parallel()

		secret := "oauth-rpc-secret-token"
		handler := NewHostAPIHandler(
			nil,
			nil,
			nil,
			nil,
			WithHostAPIModelCatalogService(&fakeHostAPIModelCatalogService{
				listErr: errors.New("catalog unavailable OAUTH_TOKEN=" + secret),
			}),
			WithHostAPICapabilityChecker(newTestCapabilityChecker(
				"ext",
				SourceUser,
				[]string{"models/list"},
				[]string{"model.read"},
			)),
		)
		_, err := handler.Handle(testutil.Context(t), "ext", "models/list", json.RawMessage(`{}`))
		if err == nil {
			t.Fatal("Handle(models/list) error = nil, want RPC error")
		}

		rpcErr, rpcErrMatched := errors.AsType[*subprocess.RPCError](err)
		if !rpcErrMatched {
			t.Fatalf("Handle(models/list) error = %T, want *RPCError", err)
		}
		data := string(rpcErr.Data)
		if strings.Contains(data, secret) {
			t.Fatalf("RPC error data = %s, want secret redacted", data)
		}
		if !strings.Contains(data, "[REDACTED]") {
			t.Fatalf("RPC error data = %s, want redaction marker", data)
		}
	})
}

func TestHostAPIModelHelpersShouldHandleEmptyValues(t *testing.T) {
	t.Parallel()

	t.Run("Should handle empty values", func(t *testing.T) {
		t.Parallel()

		var nilHandler *HostAPIHandler
		if nilHandler.hostAPINow().IsZero() {
			t.Fatal("hostAPINow(nil) returned zero time, want UTC fallback")
		}
		if got := hostAPICostPayloadFromModel(modelcatalog.Model{}); got != nil {
			t.Fatalf("hostAPICostPayloadFromModel(empty) = %#v, want nil", got)
		}
		payload := hostAPIProviderModelPayloadFromModel(modelcatalog.Model{})
		if payload.DefaultReasoningEffort != nil {
			t.Fatalf(
				"hostAPIProviderModelPayloadFromModel(empty).DefaultReasoningEffort = %#v, want nil",
				payload.DefaultReasoningEffort,
			)
		}
	})
}

type fakeHostAPIModelCatalogService struct {
	models      []modelcatalog.Model
	statuses    []modelcatalog.SourceStatus
	listOpts    []modelcatalog.ListOptions
	refreshOpts []modelcatalog.RefreshOptions
	statusOpts  []modelcatalog.StatusOptions
	listErr     error
	refreshErr  error
	statusErr   error
}

func (s *fakeHostAPIModelCatalogService) ListModels(
	_ context.Context,
	opts modelcatalog.ListOptions,
) ([]modelcatalog.Model, error) {
	s.listOpts = append(s.listOpts, opts)
	return append([]modelcatalog.Model(nil), s.models...), s.listErr
}

func (s *fakeHostAPIModelCatalogService) Refresh(
	_ context.Context,
	opts modelcatalog.RefreshOptions,
) ([]modelcatalog.SourceStatus, error) {
	s.refreshOpts = append(s.refreshOpts, opts)
	return append([]modelcatalog.SourceStatus(nil), s.statuses...), s.refreshErr
}

func (s *fakeHostAPIModelCatalogService) ListSourceStatus(
	_ context.Context,
	opts modelcatalog.StatusOptions,
) ([]modelcatalog.SourceStatus, error) {
	s.statusOpts = append(s.statusOpts, opts)
	return append([]modelcatalog.SourceStatus(nil), s.statuses...), s.statusErr
}

func assertRedactedHostAPIModelPayload(t *testing.T, value string, secret string) {
	t.Helper()

	if strings.Contains(value, secret) {
		t.Fatalf("Host API payload value = %q, want secret redacted", value)
	}
	if !strings.Contains(value, "[REDACTED]") {
		t.Fatalf("Host API payload value = %q, want redaction marker", value)
	}
}
