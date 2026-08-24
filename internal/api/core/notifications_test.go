package core_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	presetspkg "github.com/compozy/compozy/internal/notifications/presets"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

func TestNotificationPresetHandlers(t *testing.T) {
	t.Parallel()

	t.Run("Should list presets with generated metadata", func(t *testing.T) {
		t.Parallel()

		var captured presetspkg.Query
		service := &stubNotificationPresetService{}
		service.listFn = func(_ context.Context, query presetspkg.Query) ([]presetspkg.Preset, error) {
			captured = query
			return []presetspkg.Preset{notificationPresetForHandlerTest("task_terminal")}, nil
		}
		engine := newNotificationPresetHandlerFixture(t, service)

		response := performRequest(
			t,
			engine,
			http.MethodGet,
			"/notifications/presets?enabled=true&built_in=true",
			nil,
		)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
		}
		if captured.Enabled == nil || !*captured.Enabled || captured.BuiltIn == nil ||
			!*captured.BuiltIn {
			t.Fatalf("captured query = %#v, want enabled and built_in filters", captured)
		}
		if service.lastListProfileID != store.DefaultProfileID {
			t.Fatalf("ListForProfile() profile ID = %q, want default", service.lastListProfileID)
		}
		var payload contract.NotificationPresetListResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(list response) error = %v", err)
		}
		if payload.Total != 1 || payload.Presets[0].Name != "task_terminal" ||
			payload.GeneratedAt.IsZero() {
			t.Fatalf("payload = %#v, want one preset with generated timestamp", payload)
		}
	})

	t.Run("Should resolve the selected profile once for a preset read", func(t *testing.T) {
		t.Parallel()

		profiles := &notificationProfileServiceStub{}
		service := &stubNotificationPresetService{
			listFn: func(context.Context, presetspkg.Query) ([]presetspkg.Preset, error) {
				return []presetspkg.Preset{notificationPresetForHandlerTest("task_terminal")}, nil
			},
		}
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			Profiles:      profiles,
			Notifications: service,
			Now: func() time.Time {
				return time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)
			},
		})
		engine := gin.New()
		engine.GET("/notifications/presets", handlers.ListNotificationPresets)

		response := performRequest(t, engine, http.MethodGet, "/notifications/presets?profile=marketing", nil)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
		}
		if profiles.resolveCalls != 1 {
			t.Fatalf("Resolve() calls = %d, want 1", profiles.resolveCalls)
		}
		if service.lastListProfileID != "profile-marketing" {
			t.Fatalf("ListForProfile() profile ID = %q, want profile-marketing", service.lastListProfileID)
		}
	})

	t.Run("Should create update and delete through the daemon service", func(t *testing.T) {
		t.Parallel()

		calls := make([]string, 0, 3)
		engine := newNotificationPresetHandlerFixture(t, &stubNotificationPresetService{
			createFn: func(_ context.Context, request presetspkg.CreateRequest) (presetspkg.Preset, error) {
				calls = append(calls, "create:"+request.Name)
				if request.Name != "provider_failure_copy" || len(request.Targets) != 1 ||
					request.Targets[0].BridgeID != "brg-1" {
					t.Fatalf("Create() request = %#v", request)
				}
				return notificationPresetForHandlerTest(request.Name), nil
			},
			updateFn: func(_ context.Context, name string, request presetspkg.UpdateRequest) (presetspkg.Preset, error) {
				calls = append(calls, "update:"+name)
				if name != "provider_failure_copy" || request.Filter == nil || *request.Filter != "severity = warning" {
					t.Fatalf("Update() request = name %q %#v", name, request)
				}
				preset := notificationPresetForHandlerTest(name)
				preset.Enabled = true
				return preset, nil
			},
			getForProfileFn: func(_ context.Context, name, profileID string) (presetspkg.Preset, error) {
				if profileID != store.DefaultProfileID {
					t.Fatalf("GetForProfile() profile ID = %q, want default", profileID)
				}
				preset := notificationPresetForHandlerTest(name)
				preset.Enabled = true
				return preset, nil
			},
			deleteFn: func(_ context.Context, name string) error {
				calls = append(calls, "delete:"+name)
				return nil
			},
		})

		createBody := []byte(
			`{"name":"provider_failure_copy","events":["provider.*"],"targets":[{"bridge_id":"brg-1","canonical_route":"#ops"}]}`,
		)
		createResponse := performRequest(
			t,
			engine,
			http.MethodPost,
			"/notifications/presets",
			createBody,
		)
		if createResponse.Code != http.StatusCreated {
			t.Fatalf(
				"create status = %d, want 201; body=%s",
				createResponse.Code,
				createResponse.Body.String(),
			)
		}
		updateResponse := performRequest(
			t,
			engine,
			http.MethodPut,
			"/notifications/presets/provider_failure_copy",
			[]byte(`{"filter":"severity = warning"}`),
		)
		if updateResponse.Code != http.StatusOK {
			t.Fatalf(
				"update status = %d, want 200; body=%s",
				updateResponse.Code,
				updateResponse.Body.String(),
			)
		}
		deleteResponse := performRequest(
			t,
			engine,
			http.MethodDelete,
			"/notifications/presets/provider_failure_copy",
			nil,
		)
		if deleteResponse.Code != http.StatusNoContent {
			t.Fatalf(
				"delete status = %d, want 204; body=%s",
				deleteResponse.Code,
				deleteResponse.Body.String(),
			)
		}
		want := []string{
			"create:provider_failure_copy",
			"update:provider_failure_copy",
			"delete:provider_failure_copy",
		}
		if len(calls) != len(want) {
			t.Fatalf("calls = %#v, want %#v", calls, want)
		}
		for index := range want {
			if calls[index] != want[index] {
				t.Fatalf("calls[%d] = %q, want %q", index, calls[index], want[index])
			}
		}
	})
}

type notificationProfileServiceStub struct {
	core.ProfileService
	resolveCalls int
}

func (s *notificationProfileServiceStub) Resolve(
	_ context.Context,
	_ profilepkg.ResolveInput,
) (profilepkg.Resolution, error) {
	s.resolveCalls++
	return profilepkg.Resolution{
		Profile: profilepkg.Profile{
			ID: "profile-marketing", Name: "marketing", State: profilepkg.StateActive,
		},
		Source: profilepkg.ResolutionSourceFlag,
	}, nil
}

func newNotificationPresetHandlerFixture(
	t *testing.T,
	service core.NotificationPresetService,
) *gin.Engine {
	t.Helper()
	handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
		TransportName:      "api-core-test",
		MaskInternalErrors: false,
		Notifications:      service,
		Now: func() time.Time {
			return time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)
		},
	})
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.GET("/notifications/presets", handlers.ListNotificationPresets)
	engine.POST("/notifications/presets", handlers.CreateNotificationPreset)
	engine.GET("/notifications/presets/:name", handlers.GetNotificationPreset)
	engine.PUT("/notifications/presets/:name", handlers.UpdateNotificationPreset)
	engine.DELETE("/notifications/presets/:name", handlers.DeleteNotificationPreset)
	return engine
}

func notificationPresetForHandlerTest(name string) presetspkg.Preset {
	return presetspkg.Preset{
		Name:      name,
		Events:    []string{"task.run_*"},
		Targets:   []presetspkg.Target{{BridgeID: "brg-1", CanonicalRoute: "#ops"}},
		Enabled:   false,
		CreatedAt: time.Date(2026, 5, 21, 11, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 5, 21, 11, 0, 0, 0, time.UTC),
	}
}

type stubNotificationPresetService struct {
	listFn            func(context.Context, presetspkg.Query) ([]presetspkg.Preset, error)
	getFn             func(context.Context, string) (presetspkg.Preset, error)
	createFn          func(context.Context, presetspkg.CreateRequest) (presetspkg.Preset, error)
	updateFn          func(context.Context, string, presetspkg.UpdateRequest) (presetspkg.Preset, error)
	deleteFn          func(context.Context, string) error
	getForProfileFn   func(context.Context, string, string) (presetspkg.Preset, error)
	lastListProfileID string
}

func (s *stubNotificationPresetService) ListForProfile(
	ctx context.Context,
	query presetspkg.Query,
	profileID string,
) ([]presetspkg.Preset, error) {
	s.lastListProfileID = profileID
	if s.listFn != nil {
		return s.listFn(ctx, query)
	}
	return nil, errors.New("unexpected notification preset ListForProfile call")
}

func (s *stubNotificationPresetService) GetForProfile(
	ctx context.Context,
	name string,
	profileID string,
) (presetspkg.Preset, error) {
	if s.getForProfileFn != nil {
		return s.getForProfileFn(ctx, name, profileID)
	}
	return s.Get(ctx, name)
}

func (s *stubNotificationPresetService) SetEnablement(
	context.Context,
	presetspkg.EnablementChange,
) (presetspkg.Preset, error) {
	return presetspkg.Preset{}, errors.New("unexpected notification preset SetEnablement call")
}

func (s *stubNotificationPresetService) Get(
	ctx context.Context,
	name string,
) (presetspkg.Preset, error) {
	if s.getFn != nil {
		return s.getFn(ctx, name)
	}
	return presetspkg.Preset{}, errors.New("unexpected notification preset Get call")
}

func (s *stubNotificationPresetService) Create(
	ctx context.Context,
	request presetspkg.CreateRequest,
) (presetspkg.Preset, error) {
	if s.createFn != nil {
		return s.createFn(ctx, request)
	}
	return presetspkg.Preset{}, errors.New("unexpected notification preset Create call")
}

func (s *stubNotificationPresetService) Update(
	ctx context.Context,
	name string,
	request presetspkg.UpdateRequest,
) (presetspkg.Preset, error) {
	if s.updateFn != nil {
		return s.updateFn(ctx, name, request)
	}
	return presetspkg.Preset{}, errors.New("unexpected notification preset Update call")
}

func (s *stubNotificationPresetService) Delete(ctx context.Context, name string) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, name)
	}
	return errors.New("unexpected notification preset Delete call")
}

var _ core.NotificationPresetService = (*stubNotificationPresetService)(nil)
var _ core.NotificationPresetEnablementService = (*stubNotificationPresetService)(nil)
