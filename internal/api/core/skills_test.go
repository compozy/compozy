package core_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/api/testutil"
	aghconfig "github.com/compozy/agh/internal/config"
	registrypkg "github.com/compozy/agh/internal/registry"
	"github.com/compozy/agh/internal/skills"
	skillmarketplace "github.com/compozy/agh/internal/skills/marketplace"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

type stubSkillsRegistry = testutil.StubSkillsRegistry

var _ core.SkillsRegistry = (*testutil.StubSkillsRegistry)(nil)

func globalSkillProjection(
	t *testing.T,
	projected ...*skills.Skill,
) func(context.Context, *workspacepkg.ResolvedWorkspace) ([]*skills.Skill, error) {
	t.Helper()
	return func(_ context.Context, resolved *workspacepkg.ResolvedWorkspace) ([]*skills.Skill, error) {
		if resolved != nil {
			t.Fatalf("ForWorkspace() resolved = %#v, want global nil scope", resolved)
		}
		return projected, nil
	}
}

func newSkillsHandlerFixture(
	t *testing.T,
	registry core.SkillsRegistry,
	workspaces testutil.StubWorkspaceService,
) *gin.Engine {
	return newSkillsHandlerFixtureWithMarketplace(t, registry, workspaces, nil)
}

func newSkillsHandlerFixtureWithMarketplace(
	t *testing.T,
	registry core.SkillsRegistry,
	workspaces testutil.StubWorkspaceService,
	marketplace core.SkillMarketplaceService,
) *gin.Engine {
	return newSkillsHandlerFixtureWithMarketplaceAndResources(t, registry, workspaces, marketplace, nil)
}

func newSkillsHandlerFixtureWithMarketplaceAndResources(
	t *testing.T,
	registry core.SkillsRegistry,
	workspaces testutil.StubWorkspaceService,
	marketplace core.SkillMarketplaceService,
	skillResources core.SkillResourceSyncer,
) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	homePaths := testutil.NewTestHomePaths(t)
	cfg := aghconfig.DefaultWithHome(homePaths)
	cfg.HTTP.Host = "127.0.0.1"
	cfg.HTTP.Port = 2123
	cfg.Daemon.Socket = "/tmp/skills-test.sock"

	handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
		TransportName:    "skills-test",
		Sessions:         testutil.StubSessionManager{},
		Observer:         testutil.StubObserver{},
		Workspaces:       workspaces,
		SkillsRegistry:   registry,
		SkillResources:   skillResources,
		SkillMarketplace: marketplace,
		HomePaths:        homePaths,
		Config:           cfg,
		Logger:           testutil.DiscardLogger(),
		StartedAt:        time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
		Now:              func() time.Time { return time.Date(2026, 4, 3, 12, 0, 1, 0, time.UTC) },
		PollInterval:     5 * time.Millisecond,
		HTTPPort:         cfg.HTTP.Port,
	})

	engine := gin.New()
	engine.GET("/api/skills", handlers.ListSkills)
	engine.POST("/api/skills/marketplace/install", handlers.InstallSkillMarketplace)
	engine.POST("/api/skills/marketplace/update", handlers.UpdateSkillMarketplace)
	engine.DELETE("/api/skills/marketplace/:name", handlers.RemoveSkillMarketplace)
	engine.GET("/api/skills/:name", handlers.GetSkill)
	engine.GET("/api/skills/:name/content", handlers.GetSkillContent)
	engine.GET("/api/skills/:name/shadows", handlers.GetSkillShadows)
	engine.POST("/api/skills/:name/enable", handlers.EnableSkill)
	engine.POST("/api/skills/:name/disable", handlers.DisableSkill)

	return engine
}

type stubSkillMarketplaceService struct {
	SearchFn     func(ctx context.Context, query string, limit int) ([]registrypkg.Listing, error)
	SearchPageFn func(ctx context.Context, query string, offset int, limit int) ([]registrypkg.Listing, error)
	InfoFn       func(ctx context.Context, slug string) (*registrypkg.Detail, error)
	InstallFn    func(ctx context.Context, slug string, version string) (skillmarketplace.InstallResult, error)
	UpdateFn     func(ctx context.Context, req skillmarketplace.UpdateRequest) ([]skillmarketplace.UpdateResult, error)
	RemoveFn     func(ctx context.Context, name string) (skillmarketplace.RemoveResult, error)
}

func (s stubSkillMarketplaceService) Search(
	ctx context.Context,
	query string,
	offset int,
	limit int,
) ([]registrypkg.Listing, error) {
	if s.SearchPageFn != nil {
		return s.SearchPageFn(ctx, query, offset, limit)
	}
	if s.SearchFn != nil {
		return s.SearchFn(ctx, query, limit)
	}
	return nil, nil
}

func (s stubSkillMarketplaceService) Info(
	ctx context.Context,
	slug string,
) (*registrypkg.Detail, error) {
	if s.InfoFn != nil {
		return s.InfoFn(ctx, slug)
	}
	return nil, nil
}

func (s stubSkillMarketplaceService) Install(
	ctx context.Context,
	slug string,
	version string,
) (skillmarketplace.InstallResult, error) {
	if s.InstallFn != nil {
		return s.InstallFn(ctx, slug, version)
	}
	return skillmarketplace.InstallResult{}, nil
}

func (s stubSkillMarketplaceService) Update(
	ctx context.Context,
	req skillmarketplace.UpdateRequest,
) ([]skillmarketplace.UpdateResult, error) {
	if s.UpdateFn != nil {
		return s.UpdateFn(ctx, req)
	}
	return nil, nil
}

func (s stubSkillMarketplaceService) Remove(
	ctx context.Context,
	name string,
) (skillmarketplace.RemoveResult, error) {
	if s.RemoveFn != nil {
		return s.RemoveFn(ctx, name)
	}
	return skillmarketplace.RemoveResult{}, nil
}

var _ core.SkillMarketplaceService = (*stubSkillMarketplaceService)(nil)

type stubSkillResourceSyncer struct {
	SyncSkillsFn func(ctx context.Context) error
}

func (s stubSkillResourceSyncer) SyncSkills(ctx context.Context) error {
	if s.SyncSkillsFn != nil {
		return s.SyncSkillsFn(ctx)
	}
	return nil
}

var _ core.SkillResourceSyncer = (*stubSkillResourceSyncer)(nil)

func testSkill() *skills.Skill {
	return &skills.Skill{
		Meta: skills.SkillMeta{
			Name:        "test-skill",
			Description: "A test skill",
			Version:     "1.0.0",
			Metadata:    map[string]any{"key": "value"},
		},
		Source:  skills.SourceBundled,
		Dir:     "test-skill",
		Enabled: true,
	}
}

func testSkillWithProvenance() *skills.Skill {
	s := testSkill()
	s.Source = skills.SourceMarketplace
	s.Provenance = &skills.Provenance{
		Slug:        "test-org/test-skill",
		Registry:    "https://skills.example.com",
		Version:     "1.0.0",
		InstalledAt: time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC),
	}
	return s
}

func TestSkillPayloadFromSkill(t *testing.T) {
	t.Parallel()

	t.Run("Should convert all fields correctly including provenance", func(t *testing.T) {
		t.Parallel()

		skill := testSkillWithProvenance()
		skill.FilePath = "/user/skills/test-skill/SKILL.md"
		skill.Diagnostics = skills.SkillDiagnostics{
			VerificationStatus: skills.SkillVerificationStatusWarning,
			Warnings: []skills.Warning{{
				Severity: skills.SeverityWarning,
				Pattern:  "external-link",
				Message:  "Skill references an external link.",
			}},
			ShadowedDefinitions: []skills.SkillDefinitionRef{{
				Source:     "bundled",
				Path:       "test-skill/SKILL.md",
				DetectedAt: time.Date(2026, 4, 2, 9, 0, 0, 0, time.UTC),
			}},
		}
		skill.InstalledFromExtension = "review-tools"
		payload := core.SkillPayloadFromSkill(skill)

		if payload.Name != "test-skill" {
			t.Errorf("Name = %q, want %q", payload.Name, "test-skill")
		}
		if payload.Description != "A test skill" {
			t.Errorf("Description = %q, want %q", payload.Description, "A test skill")
		}
		if payload.Version != "1.0.0" {
			t.Errorf("Version = %q, want %q", payload.Version, "1.0.0")
		}
		if payload.Source != "marketplace" {
			t.Errorf("Source = %q, want %q", payload.Source, "marketplace")
		}
		if !payload.Enabled {
			t.Error("Enabled = false, want true")
		}
		if payload.Dir != "test-skill" {
			t.Errorf("Dir = %q, want %q", payload.Dir, "test-skill")
		}
		if payload.Metadata == nil || payload.Metadata["key"] != "value" {
			t.Errorf("Metadata = %v, want map with key=value", payload.Metadata)
		}
		if payload.Provenance == nil {
			t.Fatal("Provenance = nil, want non-nil")
		}
		if payload.Provenance.Slug != "test-org/test-skill" {
			t.Errorf("Provenance.Slug = %q, want %q", payload.Provenance.Slug, "test-org/test-skill")
		}
		if payload.Provenance.Registry != "https://skills.example.com" {
			t.Errorf("Provenance.Registry = %q", payload.Provenance.Registry)
		}
		if payload.Provenance.Version != "1.0.0" {
			t.Errorf("Provenance.Version = %q", payload.Provenance.Version)
		}
		if payload.Provenance.InstalledAt == nil || payload.Provenance.InstalledAt.IsZero() {
			t.Fatalf("Provenance.InstalledAt = %#v, want populated timestamp", payload.Provenance.InstalledAt)
		}
		if payload.Provenance.PrecedenceTier != "marketplace" {
			t.Errorf("Provenance.PrecedenceTier = %q, want marketplace", payload.Provenance.PrecedenceTier)
		}
		if payload.Provenance.InstalledFromExtension != "review-tools" {
			t.Errorf(
				"Provenance.InstalledFromExtension = %q, want review-tools",
				payload.Provenance.InstalledFromExtension,
			)
		}
		if got, want := len(payload.Provenance.ShadowedBy), 1; got != want {
			t.Fatalf("Provenance.ShadowedBy len = %d, want %d", got, want)
		}
		if payload.Provenance.ShadowedBy[0].Tier != "bundled" {
			t.Errorf("Provenance.ShadowedBy[0].Tier = %q, want bundled", payload.Provenance.ShadowedBy[0].Tier)
		}
		if got, want := len(payload.Diagnostics), 2; got != want {
			t.Fatalf("Diagnostics len = %d, want %d", got, want)
		}
		winner := payload.Diagnostics[0]
		if winner.State != contract.SkillDiagnosticStateValid {
			t.Fatalf("Diagnostics[0].State = %q, want %q", winner.State, contract.SkillDiagnosticStateValid)
		}
		if winner.VerificationStatus != contract.SkillVerificationStatusWarning {
			t.Fatalf(
				"Diagnostics[0].VerificationStatus = %q, want %q",
				winner.VerificationStatus,
				contract.SkillVerificationStatusWarning,
			)
		}
		if got, want := winner.Warnings[0].Severity, "warning"; got != want {
			t.Fatalf("Diagnostics[0].Warnings[0].Severity = %q, want %q", got, want)
		}
		shadowed := payload.Diagnostics[1]
		if shadowed.State != contract.SkillDiagnosticStateShadowed {
			t.Fatalf("Diagnostics[1].State = %q, want %q", shadowed.State, contract.SkillDiagnosticStateShadowed)
		}
		if shadowed.WinningPath != "/user/skills/test-skill/SKILL.md" {
			t.Fatalf(
				"Diagnostics[1].WinningPath = %q, want active skill path",
				shadowed.WinningPath,
			)
		}
	})

	t.Run("Should omit empty optional fields", func(t *testing.T) {
		t.Parallel()

		skill := &skills.Skill{
			Meta: skills.SkillMeta{
				Name:        "minimal",
				Description: "Minimal skill",
			},
			Source:  skills.SourceBundled,
			Dir:     "minimal",
			Enabled: true,
		}

		payload := core.SkillPayloadFromSkill(skill)

		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}

		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}

		for _, key := range []string{"version", "metadata"} {
			if _, exists := m[key]; exists {
				t.Errorf("JSON contains %q but field should be omitted", key)
			}
		}
		provenance, ok := m["provenance"].(map[string]any)
		if !ok {
			t.Fatalf("JSON provenance = %#v, want object", m["provenance"])
		}
		if got, want := provenance["precedence_tier"], "bundled"; got != want {
			t.Fatalf("provenance.precedence_tier = %#v, want %q", got, want)
		}
	})

	t.Run("Should return zero payload for nil skill", func(t *testing.T) {
		t.Parallel()

		payload := core.SkillPayloadFromSkill(nil)
		if payload.Name != "" {
			t.Errorf("Name = %q, want empty", payload.Name)
		}
	})
}

func TestStatusForSkillError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"Should return 200 for nil error", nil, http.StatusOK},
		{"Should return 404 for skill not found", core.ErrSkillNotFound, http.StatusNotFound},
		{"Should return 400 for validation error", core.ErrSkillValidation, http.StatusBadRequest},
		{"Should return 500 for unknown error", http.ErrServerClosed, http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := core.StatusForSkillError(tt.err)
			if got != tt.wantStatus {
				t.Errorf("StatusForSkillError(%v) = %d, want %d", tt.err, got, tt.wantStatus)
			}
		})
	}
}

func TestStatusForSkillMarketplaceError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"Should return 200 for nil error", nil, http.StatusOK},
		{"Should return 400 for validation error", skillmarketplace.ErrValidation, http.StatusBadRequest},
		{"Should return 404 for not found error", skillmarketplace.ErrNotFound, http.StatusNotFound},
		{
			"Should return 422 for non-marketplace installed skills",
			skillmarketplace.ErrNotMarketplace,
			http.StatusUnprocessableEntity,
		},
		{
			"Should return 503 when marketplace is not configured",
			skillmarketplace.ErrNotConfigured,
			http.StatusServiceUnavailable,
		},
		{
			"Should return 503 when marketplace install is unavailable after discovery",
			skillmarketplace.ErrUnavailable,
			http.StatusServiceUnavailable,
		},
		{"Should return 500 for unknown error", http.ErrServerClosed, http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := core.StatusForSkillMarketplaceError(tt.err)
			if got != tt.wantStatus {
				t.Errorf(
					"StatusForSkillMarketplaceError(%v) = %d, want %d",
					tt.err,
					got,
					tt.wantStatus,
				)
			}
		})
	}
}

func TestSkillMarketplaceHandlers(t *testing.T) {
	t.Parallel()

	t.Run("Should install marketplace skill and refresh registry", func(t *testing.T) {
		t.Parallel()

		refreshed := false
		installedSkill := &skills.Skill{
			Meta: skills.SkillMeta{
				Name: "review",
			},
			Dir:     "/tmp/agh/skills/review",
			Source:  skills.SourceMarketplace,
			Enabled: true,
			Provenance: &skills.Provenance{
				Slug:     "@agh/review",
				Registry: "clawhub",
				Version:  "1.2.0",
				Hash:     "sha256:abc",
			},
		}
		registry := &stubSkillsRegistry{
			RefreshGlobalFn: func(context.Context) error {
				refreshed = true
				return nil
			},
			GetFn: func(name string) (*skills.Skill, bool) {
				if name == "review" {
					return installedSkill, true
				}
				return nil, false
			},
		}
		marketplace := &stubSkillMarketplaceService{
			InstallFn: func(_ context.Context, slug string, version string) (skillmarketplace.InstallResult, error) {
				if slug != "@agh/review" {
					t.Errorf("Install slug = %q, want @agh/review", slug)
				}
				if version != "1.2.0" {
					t.Errorf("Install version = %q, want 1.2.0", version)
				}
				return skillmarketplace.InstallResult{
					Name:     "review",
					Slug:     "@agh/review",
					Version:  "1.2.0",
					Registry: "clawhub",
					Path:     "/tmp/agh/skills/review",
					Hash:     "sha256:abc",
					Status:   "installed",
				}, nil
			},
		}
		engine := newSkillsHandlerFixtureWithMarketplace(
			t,
			registry,
			testutil.StubWorkspaceService{},
			marketplace,
		)
		rec := testutil.PerformRequest(
			t,
			engine,
			http.MethodPost,
			"/api/skills/marketplace/install",
			testutil.MustJSONBody(t, contract.SkillMarketplaceInstallRequest{
				Slug:    "@agh/review",
				Version: "1.2.0",
			}),
		)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}
		if !refreshed {
			t.Fatal("RefreshGlobal() was not called after install")
		}

		var resp contract.SkillMarketplaceInstallResponse
		testutil.DecodeJSONResponse(t, rec, &resp)
		if resp.Skill.Status != "installed" {
			t.Fatalf("skill.Status = %q, want installed", resp.Skill.Status)
		}
	})

	t.Run("Should sync skill resources before verifying marketplace install visibility", func(t *testing.T) {
		t.Parallel()

		synced := false
		installedSkill := &skills.Skill{
			Meta: skills.SkillMeta{
				Name: "review",
			},
			Dir:     "/tmp/agh/skills/review",
			Source:  skills.SourceMarketplace,
			Enabled: true,
			Provenance: &skills.Provenance{
				Slug:     "@agh/review",
				Registry: "clawhub",
				Version:  "1.2.0",
				Hash:     "sha256:abc",
			},
		}
		registry := &stubSkillsRegistry{
			RefreshGlobalFn: func(context.Context) error {
				t.Fatal("RefreshGlobal() should not be used when skill resource syncer is configured")
				return nil
			},
			GetFn: func(name string) (*skills.Skill, bool) {
				if name == "review" && synced {
					return installedSkill, true
				}
				return nil, false
			},
		}
		marketplace := &stubSkillMarketplaceService{
			InstallFn: func(context.Context, string, string) (skillmarketplace.InstallResult, error) {
				return skillmarketplace.InstallResult{
					Name:     "review",
					Slug:     "@agh/review",
					Version:  "1.2.0",
					Registry: "clawhub",
					Path:     "/tmp/agh/skills/review",
					Hash:     "sha256:abc",
					Status:   "installed",
				}, nil
			},
		}
		skillResources := &stubSkillResourceSyncer{
			SyncSkillsFn: func(context.Context) error {
				synced = true
				return nil
			},
		}
		engine := newSkillsHandlerFixtureWithMarketplaceAndResources(
			t,
			registry,
			testutil.StubWorkspaceService{},
			marketplace,
			skillResources,
		)
		rec := testutil.PerformRequest(
			t,
			engine,
			http.MethodPost,
			"/api/skills/marketplace/install",
			testutil.MustJSONBody(t, contract.SkillMarketplaceInstallRequest{
				Slug: "@agh/review",
			}),
		)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}
		if !synced {
			t.Fatal("SyncSkills() was not called before install visibility verification")
		}
	})

	t.Run("Should reject marketplace install when refreshed registry cannot see skill", func(t *testing.T) {
		t.Parallel()

		registry := &stubSkillsRegistry{
			RefreshGlobalFn: func(context.Context) error {
				return nil
			},
			GetFn: func(string) (*skills.Skill, bool) {
				return nil, false
			},
		}
		marketplace := &stubSkillMarketplaceService{
			InstallFn: func(context.Context, string, string) (skillmarketplace.InstallResult, error) {
				return skillmarketplace.InstallResult{
					Name:     "review",
					Slug:     "@agh/review",
					Version:  "1.2.0",
					Registry: "clawhub",
					Path:     "/tmp/agh/skills/review",
					Hash:     "sha256:abc",
					Status:   "installed",
				}, nil
			},
		}
		engine := newSkillsHandlerFixtureWithMarketplace(
			t,
			registry,
			testutil.StubWorkspaceService{},
			marketplace,
		)
		rec := testutil.PerformRequest(
			t,
			engine,
			http.MethodPost,
			"/api/skills/marketplace/install",
			testutil.MustJSONBody(t, contract.SkillMarketplaceInstallRequest{
				Slug: "@agh/review",
			}),
		)

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf(
				"status = %d, want %d; body=%s",
				rec.Code,
				http.StatusServiceUnavailable,
				rec.Body.String(),
			)
		}
		if !strings.Contains(rec.Body.String(), "not visible after skill discovery") {
			t.Fatalf("body = %s, want registry visibility reason", rec.Body.String())
		}
	})

	t.Run("Should not refresh registry for update check only", func(t *testing.T) {
		t.Parallel()

		refreshCount := 0
		registry := &stubSkillsRegistry{
			RefreshGlobalFn: func(context.Context) error {
				refreshCount++
				return nil
			},
		}
		marketplace := &stubSkillMarketplaceService{
			UpdateFn: func(_ context.Context, req skillmarketplace.UpdateRequest) ([]skillmarketplace.UpdateResult, error) {
				if req.Name != "review" {
					t.Errorf("Update name = %q, want review", req.Name)
				}
				if !req.CheckOnly {
					t.Error("Update CheckOnly = false, want true")
				}
				return []skillmarketplace.UpdateResult{{
					Name:           "review",
					Slug:           "@agh/review",
					CurrentVersion: "1.1.0",
					LatestVersion:  "1.2.0",
					Path:           "/tmp/agh/skills/review",
					Status:         skillmarketplace.UpdateStatusAvailable,
				}}, nil
			},
		}
		engine := newSkillsHandlerFixtureWithMarketplace(
			t,
			registry,
			testutil.StubWorkspaceService{},
			marketplace,
		)
		rec := testutil.PerformRequest(
			t,
			engine,
			http.MethodPost,
			"/api/skills/marketplace/update",
			testutil.MustJSONBody(t, contract.SkillMarketplaceUpdateRequest{
				Name:      "review",
				CheckOnly: true,
			}),
		)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}
		if refreshCount != 0 {
			t.Fatalf("RefreshGlobal() calls = %d, want 0", refreshCount)
		}
	})

	t.Run("Should remove marketplace skill and refresh registry", func(t *testing.T) {
		t.Parallel()

		refreshed := false
		registry := &stubSkillsRegistry{
			RefreshGlobalFn: func(context.Context) error {
				refreshed = true
				return nil
			},
		}
		marketplace := &stubSkillMarketplaceService{
			RemoveFn: func(_ context.Context, name string) (skillmarketplace.RemoveResult, error) {
				if name != "review" {
					t.Errorf("Remove name = %q, want review", name)
				}
				return skillmarketplace.RemoveResult{
					Name:   "review",
					Slug:   "@agh/review",
					Path:   "/tmp/agh/skills/review",
					Status: "removed",
				}, nil
			},
		}
		engine := newSkillsHandlerFixtureWithMarketplace(
			t,
			registry,
			testutil.StubWorkspaceService{},
			marketplace,
		)
		rec := testutil.PerformRequest(
			t,
			engine,
			http.MethodDelete,
			"/api/skills/marketplace/review",
			nil,
		)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}
		if !refreshed {
			t.Fatal("RefreshGlobal() was not called after remove")
		}
	})

	t.Run("Should reject empty marketplace remove names", func(t *testing.T) {
		t.Parallel()

		marketplace := &stubSkillMarketplaceService{
			RemoveFn: func(context.Context, string) (skillmarketplace.RemoveResult, error) {
				t.Fatal("RemoveFn should not be called for empty name")
				return skillmarketplace.RemoveResult{}, nil
			},
		}
		engine := newSkillsHandlerFixtureWithMarketplace(
			t,
			&stubSkillsRegistry{},
			testutil.StubWorkspaceService{},
			marketplace,
		)
		rec := testutil.PerformRequest(
			t,
			engine,
			http.MethodDelete,
			"/api/skills/marketplace/%20%20%20",
			nil,
		)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
		}
	})

	t.Run("Should map non-marketplace removal to unprocessable entity", func(t *testing.T) {
		t.Parallel()

		marketplace := &stubSkillMarketplaceService{
			RemoveFn: func(context.Context, string) (skillmarketplace.RemoveResult, error) {
				return skillmarketplace.RemoveResult{}, skillmarketplace.ErrNotMarketplace
			},
		}
		engine := newSkillsHandlerFixtureWithMarketplace(
			t,
			&stubSkillsRegistry{},
			testutil.StubWorkspaceService{},
			marketplace,
		)
		rec := testutil.PerformRequest(
			t,
			engine,
			http.MethodDelete,
			"/api/skills/marketplace/manual",
			nil,
		)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf(
				"status = %d, want %d; body=%s",
				rec.Code,
				http.StatusUnprocessableEntity,
				rec.Body.String(),
			)
		}
	})
}

func TestListSkills(t *testing.T) {
	t.Parallel()

	t.Run("Should return global skill list when workspace is missing", func(t *testing.T) {
		t.Parallel()

		registry := &stubSkillsRegistry{
			ForWorkspaceFn: func(_ context.Context, resolved *workspacepkg.ResolvedWorkspace) ([]*skills.Skill, error) {
				if resolved != nil {
					t.Fatalf("ForWorkspace() resolved = %#v, want nil global scope", resolved)
				}
				return []*skills.Skill{testSkill()}, nil
			},
		}
		engine := newSkillsHandlerFixture(t, registry, testutil.StubWorkspaceService{})
		rec := testutil.PerformRequest(t, engine, http.MethodGet, "/api/skills", nil)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var resp struct {
			Skills []contract.SkillPayload `json:"skills"`
		}
		testutil.DecodeJSONResponse(t, rec, &resp)

		if len(resp.Skills) != 1 {
			t.Fatalf("len(skills) = %d, want 1", len(resp.Skills))
		}
		if resp.Skills[0].Name != "test-skill" {
			t.Errorf("skills[0].Name = %q, want %q", resp.Skills[0].Name, "test-skill")
		}
		if !resp.Skills[0].Activation.Active {
			t.Fatalf("skills[0].Activation = %#v, want active", resp.Skills[0].Activation)
		}
	})

	t.Run("Should expose an enabled skill as inactive with its unmet gate reason", func(t *testing.T) {
		t.Parallel()

		skill := testSkill()
		skill.Activation = skills.SkillActivation{
			Evaluated: true,
			Reasons: []skills.ActivationReason{{
				Gate:    skills.ActivationGatePlatforms,
				Code:    skills.ActivationReasonPlatformMismatch,
				Missing: []string{"linux"},
				Message: "gate platforms unmet: linux",
			}},
		}
		registry := &stubSkillsRegistry{
			ForWorkspaceFn: func(_ context.Context, _ *workspacepkg.ResolvedWorkspace) ([]*skills.Skill, error) {
				return []*skills.Skill{skill}, nil
			},
		}
		engine := newSkillsHandlerFixture(t, registry, testutil.StubWorkspaceService{})
		rec := testutil.PerformRequest(t, engine, http.MethodGet, "/api/skills", nil)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}
		var resp contract.SkillsResponse
		testutil.DecodeJSONResponse(t, rec, &resp)
		if len(resp.Skills) != 1 {
			t.Fatalf("len(skills) = %d, want 1", len(resp.Skills))
		}
		got := resp.Skills[0]
		if !got.Enabled || got.Activation.Active {
			t.Fatalf("skill enabled=%t activation=%#v, want enabled and inactive", got.Enabled, got.Activation)
		}
		if len(got.Activation.Reasons) != 1 ||
			got.Activation.Reasons[0].Code != contract.SkillActivationReasonPlatformMismatch {
			t.Fatalf("skill activation reasons = %#v, want platform mismatch", got.Activation.Reasons)
		}
		if len(got.Diagnostics) == 0 || got.Diagnostics[0].State != contract.SkillDiagnosticStateInactive {
			t.Fatalf("skill diagnostics = %#v, want inactive state", got.Diagnostics)
		}
	})

	t.Run("Should return skill list for valid workspace", func(t *testing.T) {
		t.Parallel()

		skill := testSkill()
		registry := &stubSkillsRegistry{
			ForWorkspaceFn: func(_ context.Context, resolved *workspacepkg.ResolvedWorkspace) ([]*skills.Skill, error) {
				if resolved == nil || resolved.ID != "ws-1" {
					t.Errorf("ForWorkspace got workspace %v, want ws-1", resolved)
				}
				return []*skills.Skill{skill}, nil
			},
		}
		workspaces := testutil.StubWorkspaceService{
			ResolveFn: func(_ context.Context, _ string) (workspacepkg.ResolvedWorkspace, error) {
				return workspacepkg.ResolvedWorkspace{
					Workspace: workspacepkg.Workspace{
						ID:      "ws-1",
						RootDir: "/workspace",
						Name:    "test",
					},
				}, nil
			},
		}

		engine := newSkillsHandlerFixture(t, registry, workspaces)
		rec := testutil.PerformRequest(t, engine, http.MethodGet, "/api/skills?workspace=ws-1", nil)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var resp struct {
			Skills []contract.SkillPayload `json:"skills"`
		}
		testutil.DecodeJSONResponse(t, rec, &resp)

		if len(resp.Skills) != 1 {
			t.Fatalf("len(skills) = %d, want 1", len(resp.Skills))
		}
		if resp.Skills[0].Name != "test-skill" {
			t.Errorf("skills[0].Name = %q, want %q", resp.Skills[0].Name, "test-skill")
		}
	})
}

func TestGetSkill(t *testing.T) {
	t.Parallel()

	t.Run("Should return 404 for unknown name", func(t *testing.T) {
		t.Parallel()

		registry := &stubSkillsRegistry{
			ForWorkspaceFn: globalSkillProjection(t),
		}
		engine := newSkillsHandlerFixture(t, registry, testutil.StubWorkspaceService{})
		rec := testutil.PerformRequest(t, engine, http.MethodGet, "/api/skills/nonexistent", nil)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d; body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
		}
	})

	t.Run("Should return skill metadata for valid name", func(t *testing.T) {
		t.Parallel()

		skill := testSkillWithProvenance()
		registry := &stubSkillsRegistry{
			ForWorkspaceFn: globalSkillProjection(t, skill),
		}
		engine := newSkillsHandlerFixture(t, registry, testutil.StubWorkspaceService{})
		rec := testutil.PerformRequest(t, engine, http.MethodGet, "/api/skills/test-skill", nil)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var resp struct {
			Skill contract.SkillPayload `json:"skill"`
		}
		testutil.DecodeJSONResponse(t, rec, &resp)

		if resp.Skill.Name != "test-skill" {
			t.Errorf("skill.Name = %q, want %q", resp.Skill.Name, "test-skill")
		}
		if resp.Skill.Provenance == nil {
			t.Error("skill.Provenance = nil, want non-nil")
		}
	})

	t.Run("Should resolve workspace-only skills when workspace query provided", func(t *testing.T) {
		t.Parallel()

		workspaceSkill := testSkill()
		workspaceSkill.Source = skills.SourceWorkspace
		workspaceSkill.Dir = "/workspace/.agh/skills/test-skill"

		registry := &stubSkillsRegistry{
			GetFn: func(_ string) (*skills.Skill, bool) {
				return nil, false
			},
			ForWorkspaceFn: func(_ context.Context, resolved *workspacepkg.ResolvedWorkspace) ([]*skills.Skill, error) {
				if resolved == nil || resolved.ID != "ws-1" {
					t.Errorf("ForWorkspace got workspace %v, want ws-1", resolved)
				}
				return []*skills.Skill{workspaceSkill}, nil
			},
		}
		workspaces := testutil.StubWorkspaceService{
			ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
				if ref != "ws-1" {
					t.Errorf("Resolve got ref %q, want ws-1", ref)
				}
				return workspacepkg.ResolvedWorkspace{
					Workspace: workspacepkg.Workspace{
						ID:      "ws-1",
						RootDir: "/workspace",
						Name:    "test",
					},
				}, nil
			},
		}

		engine := newSkillsHandlerFixture(t, registry, workspaces)
		rec := testutil.PerformRequest(t, engine, http.MethodGet, "/api/skills/test-skill?workspace=ws-1", nil)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var resp struct {
			Skill contract.SkillPayload `json:"skill"`
		}
		testutil.DecodeJSONResponse(t, rec, &resp)
		if resp.Skill.Source != "workspace" {
			t.Errorf("skill.Source = %q, want %q", resp.Skill.Source, "workspace")
		}
	})
}

func TestGetSkillShadows(t *testing.T) {
	t.Parallel()

	t.Run("Should return winner and shadow declarations from resolver diagnostics", func(t *testing.T) {
		t.Parallel()

		skill := testSkill()
		skill.Source = skills.SourceWorkspace
		skill.FilePath = "/workspace/.agh/skills/test-skill/SKILL.md"
		skill.Diagnostics.ShadowedDefinitions = []skills.SkillDefinitionRef{{
			Source:     "marketplace",
			Path:       "/home/agh/skills/test-skill/SKILL.md",
			DetectedAt: time.Date(2026, 4, 2, 9, 0, 0, 0, time.UTC),
		}}
		registry := &stubSkillsRegistry{ForWorkspaceFn: globalSkillProjection(t, skill)}
		engine := newSkillsHandlerFixture(t, registry, testutil.StubWorkspaceService{})
		rec := testutil.PerformRequest(t, engine, http.MethodGet, "/api/skills/test-skill/shadows", nil)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var resp contract.SkillShadowsResponse
		testutil.DecodeJSONResponse(t, rec, &resp)
		if resp.Name != "test-skill" {
			t.Fatalf("Name = %q, want test-skill", resp.Name)
		}
		if !resp.Winner.ResolvedToWinner || resp.Winner.Tier != "workspace" {
			t.Fatalf("Winner = %#v, want workspace winner", resp.Winner)
		}
		if got, want := len(resp.Shadows), 2; got != want {
			t.Fatalf("len(Shadows) = %d, want %d", got, want)
		}
		if resp.Shadows[0].Tier != "workspace" || !resp.Shadows[0].ResolvedToWinner {
			t.Fatalf("Shadows[0] = %#v, want winner first", resp.Shadows[0])
		}
		if resp.Shadows[1].Tier != "marketplace" || resp.Shadows[1].ResolvedToWinner {
			t.Fatalf("Shadows[1] = %#v, want marketplace loser", resp.Shadows[1])
		}
	})

	t.Run("Should return 404 when skill is missing", func(t *testing.T) {
		t.Parallel()

		registry := &stubSkillsRegistry{
			ForWorkspaceFn: globalSkillProjection(t),
		}
		engine := newSkillsHandlerFixture(t, registry, testutil.StubWorkspaceService{})
		rec := testutil.PerformRequest(t, engine, http.MethodGet, "/api/skills/missing/shadows", nil)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
		}
	})
}

func TestGetSkillContent(t *testing.T) {
	t.Parallel()

	t.Run("Should return explicit skill body", func(t *testing.T) {
		t.Parallel()

		skill := testSkill()
		registry := &stubSkillsRegistry{
			ForWorkspaceFn: globalSkillProjection(t, skill),
			LoadContentFn: func(_ context.Context, loaded *skills.Skill) (string, error) {
				if loaded != skill {
					t.Fatalf("LoadContent() skill = %#v, want %#v", loaded, skill)
				}
				return "# Test Skill\nBody content", nil
			},
		}
		engine := newSkillsHandlerFixture(t, registry, testutil.StubWorkspaceService{})
		rec := testutil.PerformRequest(t, engine, http.MethodGet, "/api/skills/test-skill/content", nil)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var resp contract.SkillContentResponse
		testutil.DecodeJSONResponse(t, rec, &resp)
		if resp.Content != "# Test Skill\nBody content" {
			t.Fatalf("content = %q, want %q", resp.Content, "# Test Skill\nBody content")
		}
	})

	t.Run("Should resolve workspace skill content when workspace query provided", func(t *testing.T) {
		t.Parallel()

		workspaceSkill := testSkill()
		workspaceSkill.Source = skills.SourceWorkspace
		workspaceSkill.Dir = "/workspace/.agh/skills/test-skill"

		registry := &stubSkillsRegistry{
			ForWorkspaceFn: func(_ context.Context, resolved *workspacepkg.ResolvedWorkspace) ([]*skills.Skill, error) {
				if resolved == nil || resolved.ID != "ws-1" {
					t.Errorf("ForWorkspace got workspace %v, want ws-1", resolved)
				}
				return []*skills.Skill{workspaceSkill}, nil
			},
			LoadContentFn: func(_ context.Context, loaded *skills.Skill) (string, error) {
				if loaded != workspaceSkill {
					t.Fatalf("LoadContent() skill = %#v, want workspace skill", loaded)
				}
				return "Workspace body", nil
			},
		}
		workspaces := testutil.StubWorkspaceService{
			ResolveFn: func(_ context.Context, _ string) (workspacepkg.ResolvedWorkspace, error) {
				return workspacepkg.ResolvedWorkspace{
					Workspace: workspacepkg.Workspace{ID: "ws-1", RootDir: "/workspace", Name: "test"},
				}, nil
			},
		}

		engine := newSkillsHandlerFixture(t, registry, workspaces)
		rec := testutil.PerformRequest(t, engine, http.MethodGet, "/api/skills/test-skill/content?workspace=ws-1", nil)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var resp contract.SkillContentResponse
		testutil.DecodeJSONResponse(t, rec, &resp)
		if resp.Content != "Workspace body" {
			t.Fatalf("content = %q, want %q", resp.Content, "Workspace body")
		}
	})
}

func TestEnableSkill(t *testing.T) {
	t.Parallel()

	t.Run("Should return ok true on success", func(t *testing.T) {
		t.Parallel()

		skill := testSkill()
		skill.Enabled = false
		registry := &stubSkillsRegistry{
			GetFn: func(name string) (*skills.Skill, bool) {
				if name == "test-skill" {
					return skill, true
				}
				return nil, false
			},
			ForWorkspaceFn: func(_ context.Context, resolved *workspacepkg.ResolvedWorkspace) ([]*skills.Skill, error) {
				if resolved == nil || resolved.ID != "ws-1" {
					t.Errorf("ForWorkspace got workspace %v, want ws-1", resolved)
				}
				return []*skills.Skill{skill}, nil
			},
			SetEnabledFn: func(name string, resolved *workspacepkg.ResolvedWorkspace, enabled bool) error {
				if name != "test-skill" {
					t.Errorf("SetEnabled got name %q, want %q", name, "test-skill")
				}
				if resolved == nil || resolved.ID != "ws-1" {
					t.Errorf("SetEnabled got workspace %v, want ws-1", resolved)
				}
				skill.Enabled = enabled
				return nil
			},
		}
		workspaces := testutil.StubWorkspaceService{
			ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
				if ref != "ws-1" {
					t.Errorf("Resolve got ref %q, want ws-1", ref)
				}
				return workspacepkg.ResolvedWorkspace{
					Workspace: workspacepkg.Workspace{ID: "ws-1", RootDir: "/workspace", Name: "test"},
				}, nil
			},
		}
		engine := newSkillsHandlerFixture(t, registry, workspaces)
		rec := testutil.PerformRequest(t, engine, http.MethodPost, "/api/skills/test-skill/enable?workspace=ws-1", nil)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var resp contract.SkillActionResponse
		testutil.DecodeJSONResponse(t, rec, &resp)

		if !resp.OK {
			t.Error("ok = false, want true")
		}
		if !skill.Enabled {
			t.Error("skill.Enabled = false after enable, want true")
		}
	})

	t.Run("Should return 404 when skill not found", func(t *testing.T) {
		t.Parallel()

		registry := &stubSkillsRegistry{
			GetFn: func(_ string) (*skills.Skill, bool) {
				return nil, false
			},
		}
		workspaces := testutil.StubWorkspaceService{
			ResolveFn: func(_ context.Context, _ string) (workspacepkg.ResolvedWorkspace, error) {
				return workspacepkg.ResolvedWorkspace{
					Workspace: workspacepkg.Workspace{ID: "ws-1", RootDir: "/workspace", Name: "test"},
				}, nil
			},
		}
		engine := newSkillsHandlerFixture(t, registry, workspaces)
		rec := testutil.PerformRequest(t, engine, http.MethodPost, "/api/skills/missing/enable?workspace=ws-1", nil)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d; body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
		}
	})
}

func TestDisableSkill(t *testing.T) {
	t.Parallel()

	t.Run("Should return ok true on success", func(t *testing.T) {
		t.Parallel()

		skill := testSkill()
		skill.Enabled = true
		registry := &stubSkillsRegistry{
			GetFn: func(name string) (*skills.Skill, bool) {
				if name == "test-skill" {
					return skill, true
				}
				return nil, false
			},
			ForWorkspaceFn: func(_ context.Context, resolved *workspacepkg.ResolvedWorkspace) ([]*skills.Skill, error) {
				if resolved == nil || resolved.ID != "ws-1" {
					t.Errorf("ForWorkspace got workspace %v, want ws-1", resolved)
				}
				return []*skills.Skill{skill}, nil
			},
			SetEnabledFn: func(name string, resolved *workspacepkg.ResolvedWorkspace, enabled bool) error {
				if name != "test-skill" {
					t.Errorf("SetEnabled got name %q, want %q", name, "test-skill")
				}
				if resolved == nil || resolved.ID != "ws-1" {
					t.Errorf("SetEnabled got workspace %v, want ws-1", resolved)
				}
				skill.Enabled = enabled
				return nil
			},
		}
		workspaces := testutil.StubWorkspaceService{
			ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
				if ref != "ws-1" {
					t.Errorf("Resolve got ref %q, want ws-1", ref)
				}
				return workspacepkg.ResolvedWorkspace{
					Workspace: workspacepkg.Workspace{ID: "ws-1", RootDir: "/workspace", Name: "test"},
				}, nil
			},
		}
		engine := newSkillsHandlerFixture(t, registry, workspaces)
		rec := testutil.PerformRequest(t, engine, http.MethodPost, "/api/skills/test-skill/disable?workspace=ws-1", nil)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var resp contract.SkillActionResponse
		testutil.DecodeJSONResponse(t, rec, &resp)

		if !resp.OK {
			t.Error("ok = false, want true")
		}
		if skill.Enabled {
			t.Error("skill.Enabled = true after disable, want false")
		}
	})

	t.Run("Should return 404 when skill not found", func(t *testing.T) {
		t.Parallel()

		registry := &stubSkillsRegistry{
			GetFn: func(_ string) (*skills.Skill, bool) {
				return nil, false
			},
		}
		workspaces := testutil.StubWorkspaceService{
			ResolveFn: func(_ context.Context, _ string) (workspacepkg.ResolvedWorkspace, error) {
				return workspacepkg.ResolvedWorkspace{
					Workspace: workspacepkg.Workspace{ID: "ws-1", RootDir: "/workspace", Name: "test"},
				}, nil
			},
		}
		engine := newSkillsHandlerFixture(t, registry, workspaces)
		rec := testutil.PerformRequest(t, engine, http.MethodPost, "/api/skills/missing/disable?workspace=ws-1", nil)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d; body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
		}
	})
}

func TestSkillsRegistryNotConfigured(t *testing.T) {
	t.Parallel()

	engine := newSkillsHandlerFixture(t, nil, testutil.StubWorkspaceService{})

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"ListSkills", http.MethodGet, "/api/skills?workspace=ws-1"},
		{"GetSkill", http.MethodGet, "/api/skills/test"},
		{"GetSkillContent", http.MethodGet, "/api/skills/test/content"},
		{"GetSkillShadows", http.MethodGet, "/api/skills/test/shadows"},
		{"EnableSkill", http.MethodPost, "/api/skills/test/enable?workspace=ws-1"},
		{"DisableSkill", http.MethodPost, "/api/skills/test/disable?workspace=ws-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := testutil.PerformRequest(t, engine, tt.method, tt.path, nil)
			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf(
					"%s status = %d, want %d; body=%s",
					tt.name,
					rec.Code,
					http.StatusServiceUnavailable,
					rec.Body.String(),
				)
			}
		})
	}
}
