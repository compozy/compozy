package profile

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	providerpkg "github.com/compozy/compozy/internal/providers"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/vault"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestManagerProfileLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should read one profile with its ownership and credential summary", func(t *testing.T) {
		t.Parallel()

		manager, database, _ := newTestManager(t)
		ctx := testutil.Context(t)
		created, err := manager.Create(ctx, CreateInput{Name: "marketing"})
		if err != nil {
			t.Fatalf("Create(marketing) error = %v", err)
		}
		if _, err := database.DB().ExecContext(ctx, `
			INSERT INTO profile_credential_requirements (
				profile_id, provider, slot, source_extension, declaration_digest, created_at
			) VALUES (?, 'anthropic', 'api_key', 'review-kit', 'digest-review-kit', ?)`,
			created.ID,
			formatTimestamp(time.Now()),
		); err != nil {
			t.Fatalf("insert credential requirement error = %v", err)
		}

		detail, err := manager.GetWithCounts(ctx, "marketing")
		if err != nil {
			t.Fatalf("GetWithCounts(marketing) error = %v", err)
		}
		if detail.ID != created.ID || detail.Name != "marketing" || detail.WorkItems != 0 || !detail.NeedsSetup {
			t.Fatalf("GetWithCounts(marketing) = %#v", detail)
		}
		if len(detail.CredentialRequirements) != 1 ||
			detail.CredentialRequirements[0].Provider != "anthropic" ||
			detail.CredentialRequirements[0].Slot != "api_key" ||
			detail.CredentialRequirements[0].SourceExtension != "review-kit" ||
			!detail.CredentialRequirements[0].Missing {
			t.Fatalf("credential requirements = %#v", detail.CredentialRequirements)
		}
		names, err := manager.ListNames(ctx)
		if err != nil {
			t.Fatalf("ListNames() error = %v", err)
		}
		if !slices.Equal(names, []string{"default", "marketing"}) {
			t.Fatalf("ListNames() = %#v, want default and marketing", names)
		}
	})

	t.Run("Should include extension placements in rename previews and revisions", func(t *testing.T) {
		t.Parallel()

		manager, _, _ := newTestManager(t)
		ctx := testutil.Context(t)
		if _, err := manager.Create(ctx, CreateInput{Name: "dev"}); err != nil {
			t.Fatalf("Create(dev) error = %v", err)
		}
		placements := []PlacementRef{
			{Extension: "z-kit", Resource: "release", ProfileName: "dev"},
			{Extension: "a-kit", Resource: "review", ProfileName: "dev"},
		}
		manager.placements = placementCatalogFunc(func(context.Context, string) ([]PlacementRef, error) {
			return slices.Clone(placements), nil
		})

		plan, err := manager.PrepareRename(ctx, "dev", "engineering")
		if err != nil {
			t.Fatalf("PrepareRename() error = %v", err)
		}
		want := []PlacementRef{
			{Extension: "a-kit", Resource: "review", ProfileName: "dev"},
			{Extension: "z-kit", Resource: "release", ProfileName: "dev"},
		}
		if !slices.Equal(plan.DormantPlacements, want) {
			t.Fatalf("PrepareRename().DormantPlacements = %#v, want %#v", plan.DormantPlacements, want)
		}

		placements = append(placements, PlacementRef{
			Extension: "new-kit", Resource: "deploy", ProfileName: "dev",
		})
		_, err = manager.Rename(ctx, "dev", RenameOptions{
			NewName: "engineering", Repos: RepoChoice{None: true}, PlanRevision: plan.Revision,
		})
		if !errors.Is(err, ErrPlanStale) {
			t.Fatalf("Rename(changed placements) error = %v, want ErrPlanStale", err)
		}
		if _, err := manager.GetByName(ctx, "dev"); err != nil {
			t.Fatalf("GetByName(dev after stale placement plan) error = %v", err)
		}
	})

	t.Run(
		"Should never recreate a deleted extension-declared profile while its marker exists [IT-052]",
		func(t *testing.T) {
			t.Parallel()

			manager, database, _ := newTestManager(t)
			ctx := testutil.Context(t)
			if _, err := database.DB().ExecContext(ctx, `
			INSERT INTO extensions (name, version, source, manifest_path, installed_at, checksum)
			VALUES ('growth-kit', '1.0.0', 'user', '/tmp/growth-kit/extension.toml', ?, 'checksum')`,
				formatTimestamp(time.Now()),
			); err != nil {
				t.Fatalf("insert extension error = %v", err)
			}
			created, err := manager.CreateDeclared(ctx, DeclaredInput{
				Extension: "growth-kit",
				Name:      "growth",
				Seed: DeclaredSeed{
					Color: "#5fbf85", Icon: "chart-line",
					Defaults: PersonaDefaults{Agent: "growth-analyst"},
				},
			})
			if err != nil {
				t.Fatalf("CreateDeclared(first) error = %v", err)
			}
			plan, err := manager.PrepareDelete(ctx, created.Name)
			if err != nil {
				t.Fatalf("PrepareDelete() error = %v", err)
			}
			if _, err := manager.Delete(ctx, created.Name, plan.Revision); err != nil {
				t.Fatalf("Delete() error = %v", err)
			}
			replayed, err := manager.CreateDeclared(ctx, DeclaredInput{
				Extension: "growth-kit", Name: "growth",
				Seed: DeclaredSeed{Color: "#e0635a", Icon: "flame"},
			})
			if err != nil {
				t.Fatalf("CreateDeclared(replay) error = %v", err)
			}
			if replayed.ID != created.ID || replayed.Name != created.Name {
				t.Fatalf("CreateDeclared(replay) = %#v, want original marker identity %#v", replayed, created)
			}
			var profiles int
			if err := database.DB().QueryRowContext(
				ctx, `SELECT COUNT(*) FROM profiles WHERE name = 'growth'`,
			).Scan(&profiles); err != nil {
				t.Fatalf("count recreated profiles error = %v", err)
			}
			if profiles != 0 {
				t.Fatalf("growth profile count = %d, want 0 after marker-gated replay", profiles)
			}
			if _, err := database.DB().
				ExecContext(ctx, `DELETE FROM extensions WHERE name = 'growth-kit'`); err != nil {
				t.Fatalf("uninstall extension error = %v", err)
			}
			if _, err := database.DB().ExecContext(ctx, `
			INSERT INTO extensions (name, version, source, manifest_path, installed_at, checksum)
			VALUES ('growth-kit', '2.0.0', 'user', '/tmp/growth-kit/extension.toml', ?, 'checksum-2')`,
				formatTimestamp(time.Now()),
			); err != nil {
				t.Fatalf("reinstall extension error = %v", err)
			}
			fresh, err := manager.CreateDeclared(ctx, DeclaredInput{
				Extension: "growth-kit", Name: "growth",
				Seed: DeclaredSeed{Color: "#e0635a", Icon: "flame"},
			})
			if err != nil {
				t.Fatalf("CreateDeclared(fresh install) error = %v", err)
			}
			if fresh.ID == created.ID || fresh.Color != "#e0635a" || fresh.Icon != "flame" {
				t.Fatalf("CreateDeclared(fresh install) = %#v, want a new seeded profile", fresh)
			}
		},
	)

	t.Run("Should bind an existing profile independently for each extension", func(t *testing.T) {
		t.Parallel()

		manager, database, _ := newTestManager(t)
		ctx := testutil.Context(t)
		for _, name := range []string{"kit-a", "kit-b"} {
			if _, err := database.DB().ExecContext(ctx, `
				INSERT INTO extensions (name, version, source, manifest_path, installed_at, checksum)
				VALUES (?, '1.0.0', 'user', ?, ?, 'checksum')`,
				name,
				"/tmp/"+name+"/extension.toml",
				formatTimestamp(time.Now()),
			); err != nil {
				t.Fatalf("insert extension %q error = %v", name, err)
			}
		}
		existing, err := manager.Create(ctx, CreateInput{
			Name: "shared", Color: "#112233", Icon: "briefcase",
		})
		if err != nil {
			t.Fatalf("Create(shared) error = %v", err)
		}
		for _, extensionName := range []string{"kit-a", "kit-b"} {
			bound, bindErr := manager.CreateDeclared(ctx, DeclaredInput{
				Extension: extensionName, Name: "shared",
				Seed: DeclaredSeed{Color: "#e0635a", Icon: "flame"},
			})
			if bindErr != nil {
				t.Fatalf("CreateDeclared(%s bind) error = %v", extensionName, bindErr)
			}
			if bound.ID != existing.ID || bound.Color != existing.Color || bound.Icon != existing.Icon {
				t.Fatalf("CreateDeclared(%s bind) = %#v, want untouched %#v", extensionName, bound, existing)
			}
		}
		var markerCount int
		if err := database.DB().QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM extension_profile_markers WHERE profile_name = 'shared'`,
		).Scan(&markerCount); err != nil {
			t.Fatalf("count independent markers error = %v", err)
		}
		if markerCount != 2 {
			t.Fatalf("shared marker count = %d, want 2", markerCount)
		}
	})

	t.Run("Should preserve a declared profile and its requirements across update and uninstall", func(t *testing.T) {
		t.Parallel()

		manager, database, _ := newTestManager(t)
		ctx := testutil.Context(t)
		if _, err := database.DB().ExecContext(ctx, `
			INSERT INTO extensions (name, version, source, manifest_path, installed_at, checksum)
			VALUES ('kit-a', '1.0.0', 'user', '/tmp/kit-a/extension.toml', ?, 'checksum')`,
			formatTimestamp(time.Now()),
		); err != nil {
			t.Fatalf("insert extension kit-a error = %v", err)
		}

		created, err := manager.CreateDeclared(ctx, DeclaredInput{
			Extension: "kit-a", Name: "retained",
			Seed: DeclaredSeed{
				Color: "#5fbf85", Icon: "chart-line",
				Defaults:       PersonaDefaults{Agent: "analyst"},
				CredentialAsks: []CredentialAsk{{Provider: "openai", Slot: "api_key"}},
			},
		})
		if err != nil {
			t.Fatalf("CreateDeclared(retained) error = %v", err)
		}
		replayed, err := manager.CreateDeclared(ctx, DeclaredInput{
			Extension: "kit-a", Name: "retained",
			Seed: DeclaredSeed{
				Color: "#e0635a", Icon: "flame", Defaults: PersonaDefaults{Agent: "mutated"},
			},
		})
		if err != nil {
			t.Fatalf("CreateDeclared(update replay) error = %v", err)
		}
		if replayed.ID != created.ID || replayed.Color != created.Color || replayed.Icon != created.Icon {
			t.Fatalf("CreateDeclared(update replay) = %#v, want unchanged %#v", replayed, created)
		}
		if _, err := database.DB().ExecContext(ctx, `DELETE FROM extensions WHERE name = 'kit-a'`); err != nil {
			t.Fatalf("uninstall kit-a error = %v", err)
		}
		preserved, err := manager.GetByName(ctx, "retained")
		if err != nil {
			t.Fatalf("GetByName(retained after uninstall) error = %v", err)
		}
		var markers, requirements int
		if err := database.DB().QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM extension_profile_markers WHERE extension_name = 'kit-a'`,
		).Scan(&markers); err != nil {
			t.Fatalf("count removed markers error = %v", err)
		}
		if err := database.DB().QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM profile_credential_requirements WHERE profile_id = ?`,
			created.ID,
		).Scan(&requirements); err != nil {
			t.Fatalf("count preserved requirements error = %v", err)
		}
		if preserved.ID != created.ID || markers != 0 || requirements != 1 {
			t.Fatalf(
				"uninstall state = profile %#v markers %d requirements %d, want profile preserved, 0 markers, 1 requirement",
				preserved,
				markers,
				requirements,
			)
		}
	})

	t.Run("Should enumerate and atomically rewrite only renamed profile vault refs", func(t *testing.T) {
		t.Parallel()

		manager, database, home := newTestManager(t)
		ctx := testutil.Context(t)
		created, err := manager.Create(ctx, CreateInput{Name: "dev"})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		service, err := vault.NewService(
			database,
			vault.NewFileKeyProvider(home.HomeDir, nil),
		)
		if err != nil {
			t.Fatalf("vault.NewService() error = %v", err)
		}
		oldRef := "vault:profiles/dev/providers/openai/api_key"
		foreignRef := "vault:profiles/sales/providers/openai/api_key"
		if _, err := service.PutSecret(ctx, oldRef, "api_key", "dev-secret"); err != nil {
			t.Fatalf("PutSecret(dev) error = %v", err)
		}
		if _, err := service.PutSecret(ctx, foreignRef, "api_key", "sales-secret"); err != nil {
			t.Fatalf("PutSecret(sales) error = %v", err)
		}
		now := formatTimestamp(time.Now())
		if _, err := database.DB().ExecContext(ctx, `
			INSERT INTO extension_env_bindings
			(extension_name, profile_id, workspace_id, env_name, secret_ref, kind, created_at, updated_at)
			VALUES ('growth', ?, '', 'OPENAI_API_KEY', ?, 'extension_env', ?, ?)`,
			created.ID,
			oldRef,
			now,
			now,
		); err != nil {
			t.Fatalf("insert extension profile secret binding error = %v", err)
		}
		bridgeRef := "vault:profiles/dev/bridges/telegram/token"
		if _, err := database.DB().ExecContext(ctx, `INSERT INTO bridge_instances (
			id, profile_id, scope, workspace_id, platform, extension_name, display_name,
			status, routing_policy, created_at, updated_at
		) VALUES (
			'bridge-profile-rename', ?, 'global', NULL, 'telegram', 'telegram.bridge',
			'Profile rename bridge', 'active', '{}', ?, ?
		)`, created.ID, now, now); err != nil {
			t.Fatalf("insert profile rename bridge error = %v", err)
		}
		if _, err := database.DB().ExecContext(ctx, `INSERT INTO bridge_secret_bindings (
			bridge_instance_id, binding_name, secret_ref, kind, created_at, updated_at
		) VALUES ('bridge-profile-rename', 'token', ?, 'bridge_secret', ?, ?)`,
			bridgeRef, now, now,
		); err != nil {
			t.Fatalf("insert bridge profile secret binding error = %v", err)
		}
		automationRef := "vault:profiles/dev/automations/nightly/webhook"
		if _, err := database.DB().ExecContext(ctx, `INSERT INTO automation_triggers (
			id, profile_id, scope, name, agent_name, workspace_id, prompt, event, enabled,
			retry, fire_limit, source, webhook_secret_ref, target_kind, created_at, updated_at
		) VALUES (
			'automation-profile-rename', ?, 'global', 'profile rename', 'default', NULL,
			'run', 'push', 1, '{}', '{}', 'dynamic', ?, 'agent', ?, ?
		)`, created.ID, automationRef, now, now); err != nil {
			t.Fatalf("insert automation profile secret ref error = %v", err)
		}
		mcpOldTarget := mcpauth.Target{
			Scope: mcpauth.ScopeProfile, WorkspaceID: "dev", ServerName: "github",
		}
		if err := database.SaveMCPAuthToken(ctx, mcpauth.TokenRecord{
			Target:                mcpOldTarget,
			DefinitionFingerprint: "sha256:" + strings.Repeat("a", 64),
			ClientID:              "profile-rename-client",
			AccessToken:           "github-profile-token",
		}); err != nil {
			t.Fatalf("SaveMCPAuthToken(profile) error = %v", err)
		}
		mcpRefreshRef := "vault:mcp/profile/dev/github/oauth/refresh-token"
		if _, err := service.PutSecret(
			ctx,
			mcpRefreshRef,
			"mcp_oauth_refresh_token",
			"github-refresh-token",
		); err != nil {
			t.Fatalf("PutSecret(MCP refresh) error = %v", err)
		}
		if _, err := database.DB().ExecContext(ctx, `UPDATE mcp_auth_tokens
			SET refresh_token_ref = ?
			WHERE scope = 'profile' AND workspace_id = 'dev' AND server_name = 'github'`,
			mcpRefreshRef,
		); err != nil {
			t.Fatalf("add MCP refresh ref error = %v", err)
		}
		mcpClientSecretRef := "vault:mcp/profile/dev/github/oauth/dcr-client-secret"
		mcpRegistrationTokenRef := "vault:mcp/profile/dev/github/oauth/registration-access-token"
		if _, err := database.DB().ExecContext(ctx, `INSERT INTO mcp_oauth_registrations (
			scope, workspace_id, server_name, definition_fingerprint, resource_url, issuer,
			client_id, client_secret_ref, registration_access_token_ref, registration_client_uri, redirect_uri,
			scopes_json, updated_at
		) VALUES (
			'profile', 'dev', 'github', 'definition', 'https://resource.example',
			'https://issuer.example', 'profile-rename-client', ?, ?,
			'https://issuer.example/register/profile-rename-client', 'http://127.0.0.1/callback', '[]', ?
		)`, mcpClientSecretRef, mcpRegistrationTokenRef, now); err != nil {
			t.Fatalf("insert MCP registration refs error = %v", err)
		}

		beforeRewrites, err := vault.ListProfileRefRewrites(ctx, database.DB(), "dev", "engineering")
		if err != nil {
			t.Fatalf("ListProfileRefRewrites(before rename) error = %v", err)
		}
		locationCounts := make(map[string]int)
		for _, rewrite := range beforeRewrites {
			locationCounts[rewrite.Location]++
		}
		for _, location := range []string{
			"vault_secrets.ref",
			"extension_env_bindings.secret_ref",
			"bridge_secret_bindings.secret_ref",
			"automation_triggers.webhook_secret_ref",
			"mcp_auth_tokens.access_token_ref",
			"mcp_auth_tokens.refresh_token_ref",
			"mcp_oauth_registrations.client_secret_ref",
			"mcp_oauth_registrations.registration_access_token_ref",
			"mcp_auth_tokens.workspace_id",
			"mcp_oauth_registrations.workspace_id",
		} {
			if locationCounts[location] == 0 {
				t.Fatalf("profile ref rewrite location %q has no fixture", location)
			}
		}

		plan, err := manager.PrepareRename(ctx, "dev", "engineering")
		if err != nil {
			t.Fatalf("PrepareRename() error = %v", err)
		}
		if plan.VaultRefRewrites != len(beforeRewrites) {
			t.Fatalf("PrepareRename().VaultRefRewrites = %d, want %d persisted occurrences",
				plan.VaultRefRewrites, len(beforeRewrites))
		}
		if _, err := manager.Rename(ctx, "dev", RenameOptions{
			NewName: "engineering", Repos: RepoChoice{None: true}, PlanRevision: plan.Revision,
		}); err != nil {
			t.Fatalf("Rename() error = %v", err)
		}

		newRef := "vault:profiles/engineering/providers/openai/api_key"
		value, err := service.ResolveRef(ctx, newRef)
		if err != nil {
			t.Fatalf("ResolveRef(renamed) error = %v", err)
		}
		if value != "dev-secret" {
			t.Fatalf("ResolveRef(renamed) = %q, want dev-secret", value)
		}
		if _, err := service.ResolveRef(ctx, oldRef); !errors.Is(err, vault.ErrSecretNotFound) {
			t.Fatalf("ResolveRef(old) error = %v, want ErrSecretNotFound", err)
		}
		foreignValue, err := service.ResolveRef(ctx, foreignRef)
		if err != nil || foreignValue != "sales-secret" {
			t.Fatalf("ResolveRef(foreign) = %q, %v; want unchanged sales-secret", foreignValue, err)
		}
		var bindingRef string
		if err := database.DB().QueryRowContext(
			ctx,
			`SELECT secret_ref FROM extension_env_bindings WHERE extension_name = 'growth' AND profile_id = ?`,
			created.ID,
		).Scan(&bindingRef); err != nil {
			t.Fatalf("read renamed extension binding error = %v", err)
		}
		if bindingRef != newRef {
			t.Fatalf("extension binding ref = %q, want %q", bindingRef, newRef)
		}
		mcpNewTarget := mcpauth.Target{
			Scope: mcpauth.ScopeProfile, WorkspaceID: "engineering", ServerName: "github",
		}
		mcpToken, err := database.GetMCPAuthToken(ctx, mcpNewTarget)
		if err != nil {
			t.Fatalf("GetMCPAuthToken(renamed profile) error = %v", err)
		}
		if mcpToken.AccessToken != "github-profile-token" {
			t.Fatalf("renamed MCP access token = %q, want github-profile-token", mcpToken.AccessToken)
		}
		if _, err := database.GetMCPAuthToken(ctx, mcpOldTarget); !errors.Is(err, mcpauth.ErrTokenNotFound) {
			t.Fatalf("GetMCPAuthToken(old profile) error = %v, want ErrTokenNotFound", err)
		}
		var mcpAccessRef, renamedRefreshRef string
		if err := database.DB().QueryRowContext(
			ctx,
			`SELECT access_token_ref, refresh_token_ref FROM mcp_auth_tokens
			 WHERE scope = 'profile' AND workspace_id = 'engineering' AND server_name = 'github'`,
		).Scan(&mcpAccessRef, &renamedRefreshRef); err != nil {
			t.Fatalf("read renamed MCP access ref error = %v", err)
		}
		if want := "vault:mcp/profile/engineering/github/oauth/access-token"; mcpAccessRef != want {
			t.Fatalf("renamed MCP access ref = %q, want %q", mcpAccessRef, want)
		}
		if want := "vault:mcp/profile/engineering/github/oauth/refresh-token"; renamedRefreshRef != want {
			t.Fatalf("renamed MCP refresh ref = %q, want %q", renamedRefreshRef, want)
		}
		oldRewrites, err := vault.ListProfileRefRewrites(ctx, database.DB(), "dev", "engineering")
		if err != nil {
			t.Fatalf("ListProfileRefRewrites(old owner after rename) error = %v", err)
		}
		if len(oldRewrites) != 0 {
			t.Fatalf("old profile ref rewrites after rename = %#v, want none", oldRewrites)
		}
		reverseRewrites, err := vault.ListProfileRefRewrites(ctx, database.DB(), "engineering", "dev")
		if err != nil {
			t.Fatalf("ListProfileRefRewrites(new owner after rename) error = %v", err)
		}
		if len(reverseRewrites) != len(beforeRewrites) {
			t.Fatalf("new profile ref occurrences = %d, want %d", len(reverseRewrites), len(beforeRewrites))
		}
	})

	t.Run("Should roll back profile identity and refs when ciphertext cannot be rebound", func(t *testing.T) {
		t.Parallel()

		manager, database, home := newTestManager(t)
		ctx := testutil.Context(t)
		created, err := manager.Create(ctx, CreateInput{Name: "dev"})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		service, err := vault.NewService(database, vault.NewFileKeyProvider(home.HomeDir, nil))
		if err != nil {
			t.Fatalf("vault.NewService() error = %v", err)
		}
		oldRef := "vault:profiles/dev/providers/openai/api_key"
		if _, err := service.PutSecret(ctx, oldRef, "api_key", "dev-secret"); err != nil {
			t.Fatalf("PutSecret(dev) error = %v", err)
		}
		now := formatTimestamp(time.Now())
		if _, err := database.DB().ExecContext(ctx, `INSERT INTO extension_env_bindings (
			extension_name, profile_id, workspace_id, env_name, secret_ref, kind, created_at, updated_at
		) VALUES ('growth', ?, '', 'OPENAI_API_KEY', ?, 'extension_env', ?, ?)`,
			created.ID, oldRef, now, now,
		); err != nil {
			t.Fatalf("insert rollback profile binding error = %v", err)
		}
		if _, err := database.DB().ExecContext(ctx, `UPDATE vault_secrets
			SET encrypted_value = 'invalid-ciphertext' WHERE ref = ?`, oldRef); err != nil {
			t.Fatalf("corrupt rollback ciphertext fixture error = %v", err)
		}
		plan, err := manager.PrepareRename(ctx, "dev", "engineering")
		if err != nil {
			t.Fatalf("PrepareRename() error = %v", err)
		}
		if _, err := manager.Rename(ctx, "dev", RenameOptions{
			NewName: "engineering", Repos: RepoChoice{None: true}, PlanRevision: plan.Revision,
		}); err == nil {
			t.Fatal("Rename(invalid ciphertext) error = nil, want atomic rollback")
		}
		if _, err := manager.GetByName(ctx, "dev"); err != nil {
			t.Fatalf("GetByName(original after rollback) error = %v", err)
		}
		if _, err := manager.GetByName(ctx, "engineering"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("GetByName(new after rollback) error = %v, want ErrNotFound", err)
		}
		var bindingRef string
		if err := database.DB().QueryRowContext(ctx, `SELECT secret_ref FROM extension_env_bindings
			WHERE extension_name = 'growth' AND profile_id = ?`, created.ID).Scan(&bindingRef); err != nil {
			t.Fatalf("read rollback profile binding error = %v", err)
		}
		if bindingRef != oldRef {
			t.Fatalf("binding ref after rollback = %q, want %q", bindingRef, oldRef)
		}
	})

	t.Run("Should preserve identity through update and rename", func(t *testing.T) {
		t.Parallel()

		manager, database, home := newTestManager(t)
		ctx := testutil.Context(t)
		created, err := manager.Create(ctx, CreateInput{
			Name: "marketing", Color: "#FF7F3A", Icon: "megaphone",
			Activate: &Lens{Kind: SelectionLensGlobal},
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if len(created.ID) != 26 || created.State != StateActive || created.Color != "#ff7f3a" {
			t.Fatalf("Create() = %#v, want active persisted identity with ULID", created)
		}
		seedMCPAuthProfileLifecycleRows(ctx, t, database, "marketing")
		if _, err := os.Stat(filepath.Join(home.ProfilesDir, "marketing")); err != nil {
			t.Fatalf("created profile directory error = %v", err)
		}

		emoji := "🚀"
		updated, err := manager.UpdateIdentity(ctx, created.Name, IdentityPatch{Emoji: &emoji})
		if err != nil {
			t.Fatalf("UpdateIdentity() error = %v", err)
		}
		if updated.Emoji != emoji || updated.Icon != "" || updated.ID != created.ID {
			t.Fatalf("UpdateIdentity() = %#v, want emoji replacement and stable id", updated)
		}

		renamePlan, err := manager.PrepareRename(ctx, "marketing", "growth")
		if err != nil {
			t.Fatalf("PrepareRename() error = %v", err)
		}
		if _, err := manager.Rename(ctx, "marketing", RenameOptions{
			NewName: "growth", Repos: RepoChoice{None: true}, PlanRevision: renamePlan.Revision,
		}); err != nil {
			t.Fatalf("Rename() error = %v", err)
		}
		renamed, err := manager.GetByName(ctx, "growth")
		if err != nil {
			t.Fatalf("GetByName(renamed) error = %v", err)
		}
		if renamed.ID != created.ID {
			t.Fatalf("renamed ID = %q, want %q", renamed.ID, created.ID)
		}
		assertMCPAuthProfileLifecycleRows(ctx, t, database, "marketing", 0)
		assertMCPAuthProfileLifecycleRows(ctx, t, database, "growth", 4)
		if _, err := os.Stat(filepath.Join(home.ProfilesDir, "growth")); err != nil {
			t.Fatalf("renamed profile directory error = %v", err)
		}
	})

	t.Run("Should retain desktop partitions through archive and unarchive", func(t *testing.T) {
		t.Parallel()

		desktops := &fakeDesktopPartitions{byProfile: map[string]int{}}
		manager, _, _ := newTestManagerWithDesktops(t, desktops)
		ctx := testutil.Context(t)
		created, err := manager.Create(ctx, CreateInput{
			Name: "growth", Activate: &Lens{Kind: SelectionLensGlobal},
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		// Desks this profile owns before it is archived. Archiving is a pause, not a
		// removal: the arrangement has to survive it untouched (US-026.EC-1).
		desktops.byProfile[created.ID] = 2

		archivePlan, err := manager.PrepareArchive(ctx, "growth")
		if err != nil {
			t.Fatalf("PrepareArchive() error = %v", err)
		}
		if _, err := manager.Archive(ctx, "growth", archivePlan.Revision); err != nil {
			t.Fatalf("Archive() error = %v", err)
		}
		if _, err := manager.Archive(ctx, "growth", archivePlan.Revision); err != nil {
			t.Fatalf("Archive(idempotent) error = %v", err)
		}
		fallback, err := manager.Resolve(ctx, ResolveInput{Lens: Lens{Kind: SelectionLensGlobal}})
		if err != nil {
			t.Fatalf("Resolve(archived remembered) error = %v", err)
		}
		if fallback.Profile.Name != "default" || fallback.Note != ResolutionNoteArchivedRememberedFallback {
			t.Fatalf("Resolve(archived remembered) = %#v", fallback)
		}

		unarchived, err := manager.Unarchive(ctx, "growth")
		if err != nil {
			t.Fatalf("Unarchive() error = %v", err)
		}
		if unarchived.Profile.State != StateActive {
			t.Fatalf("Unarchive().Profile.State = %q, want active", unarchived.Profile.State)
		}
		idempotentUnarchive, err := manager.Unarchive(ctx, "growth")
		if err != nil {
			t.Fatalf("Unarchive(idempotent) error = %v", err)
		}
		if len(idempotentUnarchive.PausedAutomations) != 0 {
			t.Fatalf(
				"Unarchive(idempotent).PausedAutomations = %v, want no historical archive actions",
				idempotentUnarchive.PausedAutomations,
			)
		}
		if len(desktops.purged) != 0 {
			t.Fatalf("purged desktop partitions = %v, want none across archive and unarchive", desktops.purged)
		}
		retained, err := desktops.CountDesktopPartitions(ctx, created.ID)
		if err != nil {
			t.Fatalf("CountDesktopPartitions(after unarchive) error = %v", err)
		}
		if retained != 2 {
			t.Fatalf("desktop partitions after unarchive = %d, want 2 retained", retained)
		}
	})

	t.Run("Should preview and sweep the complete delete catalog", func(t *testing.T) {
		t.Parallel()

		desktops := &fakeDesktopPartitions{byProfile: map[string]int{}}
		manager, database, home := newTestManagerWithDesktops(t, desktops)
		ctx := testutil.Context(t)
		created, err := manager.Create(ctx, CreateInput{
			Name: "growth", Activate: &Lens{Kind: SelectionLensGlobal},
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		desktops.byProfile[created.ID] = 2
		seedMCPAuthProfileLifecycleRows(ctx, t, database, "growth")

		profileDir := filepath.Join(home.ProfilesDir, "growth")
		if err := os.WriteFile(
			filepath.Join(profileDir, compozyconfig.ConfigName),
			[]byte("[persona]\nagent = \"coder\"\nprovider = \"codex\"\n"),
			0o600,
		); err != nil {
			t.Fatalf("write profile config fixture error = %v", err)
		}
		if err := os.WriteFile(
			filepath.Join(profileDir, compozyconfig.MCPJSONName),
			[]byte(`{"mcpServers":{"github":{"command":"github-mcp"},"linear":{"command":"linear-mcp"}}}`),
			0o600,
		); err != nil {
			t.Fatalf("write profile MCP fixture error = %v", err)
		}
		vaultService, err := vault.NewService(database, vault.NewFileKeyProvider(home.HomeDir, nil))
		if err != nil {
			t.Fatalf("vault.NewService() error = %v", err)
		}
		credentialRefs := []string{
			"vault:profiles/growth/providers/openai/api_key",
			"vault:mcp/profile/growth/github/access_token",
		}
		for _, ref := range credentialRefs {
			if _, err := vaultService.PutSecret(ctx, ref, "api_key", "secret"); err != nil {
				t.Fatalf("PutSecret(%q) error = %v", ref, err)
			}
		}
		if _, err := database.DB().ExecContext(ctx, `
			INSERT INTO profile_credential_requirements
				(profile_id, provider, slot, source_extension, declaration_digest, created_at)
			VALUES (?, 'anthropic', 'api_key', 'growth-kit', 'declaration-digest', '2026-08-22T00:00:00Z')`,
			created.ID,
		); err != nil {
			t.Fatalf("seed profile credential requirement error = %v", err)
		}
		if _, err := database.DB().ExecContext(ctx, `
			INSERT INTO event_summaries (id, profile_id, type, timestamp)
			VALUES ('evt-profile-growth', ?, 'profile.updated', '2026-08-22T00:00:00Z')`,
			created.ID,
		); err != nil {
			t.Fatalf("seed profile event summary error = %v", err)
		}

		// Both arrangements are part of the enumerated removal catalog and must be
		// gone after the delete (US-006).
		deletePlan, err := manager.PrepareDelete(ctx, "growth")
		if err != nil {
			t.Fatalf("PrepareDelete() error = %v", err)
		}
		if deletePlan.Removed.ConfigKeys != 2 || deletePlan.Removed.MCPServers != 2 ||
			deletePlan.Removed.CredentialOverrides != 3 || deletePlan.Removed.DesktopPartitions != 2 ||
			deletePlan.Removed.EventSummaries != 1 {
			t.Fatalf(
				"PrepareDelete().Removed = %#v, want config=2 MCP=2 credentials=3 desktops=2 events=1",
				deletePlan.Removed,
			)
		}
		deleted, err := manager.Delete(ctx, "growth", deletePlan.Revision)
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		if deleted.SweptSelections != 1 {
			t.Fatalf("Delete().SweptSelections = %d, want 1", deleted.SweptSelections)
		}
		if deleted.Removed != deletePlan.Removed {
			t.Fatalf("Delete().Removed = %#v, want preview %#v", deleted.Removed, deletePlan.Removed)
		}
		if _, err := os.Stat(profileDir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("deleted profile directory error = %v, want not exist", err)
		}
		for _, ref := range credentialRefs {
			if _, err := vaultService.ResolveRef(ctx, ref); !errors.Is(err, vault.ErrSecretNotFound) {
				t.Fatalf("ResolveRef(%q) error = %v, want ErrSecretNotFound", ref, err)
			}
		}
		if _, err := manager.GetByName(ctx, "growth"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("GetByName(deleted) error = %v, want ErrNotFound", err)
		}
		var eventSummaries int
		if err := database.DB().QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM event_summaries WHERE profile_id = ?`,
			created.ID,
		).Scan(&eventSummaries); err != nil {
			t.Fatalf("count removed profile event summaries error = %v", err)
		}
		if eventSummaries != 0 {
			t.Fatalf("removed profile event summaries = %d, want 0", eventSummaries)
		}
		var selections int
		if err := database.DB().
			QueryRowContext(ctx, `SELECT COUNT(*) FROM profile_selections WHERE profile_id = ?`, created.ID).
			Scan(&selections); err != nil {
			t.Fatalf("count swept selections error = %v", err)
		}
		if selections != 0 {
			t.Fatalf("selection rows = %d, want 0", selections)
		}
		var requirements int
		if err := database.DB().QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM profile_credential_requirements WHERE profile_id = ?`,
			created.ID,
		).Scan(&requirements); err != nil {
			t.Fatalf("count credential requirements error = %v", err)
		}
		if requirements != 0 {
			t.Fatalf("profile credential requirement rows = %d, want 0", requirements)
		}
		assertMCPAuthProfileLifecycleRows(ctx, t, database, "growth", 0)
		if !slices.Contains(desktops.purged, created.ID) {
			t.Fatalf("purged desktop partitions = %v, want profile %q", desktops.purged, created.ID)
		}
		if remaining := desktops.byProfile[created.ID]; remaining != 0 {
			t.Fatalf("desktop partitions after delete = %d, want 0", remaining)
		}
		if _, err := manager.Create(ctx, CreateInput{Name: "growth"}); err != nil {
			t.Fatalf("Create(freed name) error = %v", err)
		}
	})

	t.Run("Should record lifecycle events with snake_case payload keys", func(t *testing.T) {
		t.Parallel()

		recorder := &recordingEventRecorder{}
		manager := newTestManagerWithRecorder(t, recorder)
		ctx := testutil.Context(t)
		created, err := manager.Create(ctx, CreateInput{
			Name: "marketing", Color: "#ff7f3a", Icon: "megaphone",
			Activate: &Lens{Kind: SelectionLensGlobal},
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		names := recorder.names()
		if !slices.Contains(names, "profile.created") || !slices.Contains(names, "profile.selection_changed") {
			t.Fatalf("recorded events = %v, want profile.created and profile.selection_changed", names)
		}

		selection, ok := recorder.find("profile.selection_changed")
		if !ok {
			t.Fatalf("recorded events = %v, want profile.selection_changed", names)
		}
		// The marshaled form is what reaches clients as the `content` field of a
		// log event, so the wire keys are the contract — not the Go field names.
		payload, err := json.Marshal(selection)
		if err != nil {
			t.Fatalf("Marshal(event) error = %v", err)
		}
		var wire map[string]string
		if err := json.Unmarshal(payload, &wire); err != nil {
			t.Fatalf("Unmarshal(event) error = %v", err)
		}
		if wire["name"] != "profile.selection_changed" {
			t.Fatalf("event name = %q, want %q", wire["name"], "profile.selection_changed")
		}
		if wire["profile_id"] != created.ID {
			t.Fatalf("event profile_id = %q, want %q", wire["profile_id"], created.ID)
		}
		if wire["profile_name"] != "marketing" {
			t.Fatalf("event profile_name = %q, want %q", wire["profile_name"], "marketing")
		}
		renamePlan, err := manager.PrepareRename(ctx, "marketing", "growth")
		if err != nil {
			t.Fatalf("PrepareRename() error = %v", err)
		}
		if _, err := manager.Rename(ctx, "marketing", RenameOptions{
			NewName: "growth", Repos: RepoChoice{None: true}, PlanRevision: renamePlan.Revision,
		}); err != nil {
			t.Fatalf("Rename() error = %v", err)
		}
		renamed, ok := recorder.find("profile.renamed")
		if !ok {
			t.Fatalf("recorded events = %v, want profile.renamed", recorder.names())
		}
		if renamed.ProfileName != "growth" || renamed.PreviousProfileName != "marketing" {
			t.Fatalf("rename event = %#v, want growth with previous name marketing", renamed)
		}
		// An empty error must not reach the wire at all — clients branch on presence.
		if _, present := wire["error"]; present {
			t.Fatalf("event payload = %v, want no error key when the event succeeded", wire)
		}
		for key := range wire {
			if strings.ToLower(key) != key {
				t.Fatalf("event payload key %q is not snake_case: %v", key, wire)
			}
		}
	})

	t.Run("Should reject invalid reserved duplicate and conflicting identity inputs", func(t *testing.T) {
		t.Parallel()

		manager, _, _ := newTestManager(t)
		ctx := testutil.Context(t)
		t.Run("Should reject nil contexts at public planning boundaries", func(t *testing.T) {
			t.Parallel()

			if _, err := manager.PrepareRename(
				nil, //nolint:staticcheck // Intentionally verifies the public nil-context guard.
				"dev",
				"engineering",
			); err == nil ||
				!strings.Contains(err.Error(), "context is required") {
				t.Fatalf("PrepareRename(nil) error = %v, want context validation", err)
			}
			if _, err := manager.PrepareArchive(
				nil, //nolint:staticcheck // Intentionally verifies the public nil-context guard.
				"dev",
			); err == nil ||
				!strings.Contains(err.Error(), "context is required") {
				t.Fatalf("PrepareArchive(nil) error = %v, want context validation", err)
			}
			if _, err := manager.PrepareDelete(
				nil, //nolint:staticcheck // Intentionally verifies the public nil-context guard.
				"dev",
			); err == nil ||
				!strings.Contains(err.Error(), "context is required") {
				t.Fatalf("PrepareDelete(nil) error = %v, want context validation", err)
			}
			if _, _, err := manager.selections.Get(
				nil, //nolint:staticcheck // Intentionally verifies the public nil-context guard.
				SelectionLensGlobal,
				"",
			); err == nil ||
				!strings.Contains(err.Error(), "context is required") {
				t.Fatalf("SelectionStore.Get(nil) error = %v, want context validation", err)
			}
			if _, err := manager.Archive(
				nil, //nolint:staticcheck // Intentionally verifies the public nil-context guard.
				"dev",
				"revision",
			); err == nil ||
				!strings.Contains(err.Error(), "context is required") {
				t.Fatalf("Archive(nil) error = %v, want context validation", err)
			}
			if _, err := manager.Delete(
				nil, //nolint:staticcheck // Intentionally verifies the public nil-context guard.
				"dev",
				"revision",
			); err == nil ||
				!strings.Contains(err.Error(), "context is required") {
				t.Fatalf("Delete(nil) error = %v, want context validation", err)
			}
		})
		for _, name := range []string{"Marketing", "mkt space", "-x", strings.Repeat("a", 33)} {
			t.Run("Should reject invalid name "+name, func(t *testing.T) {
				t.Parallel()

				if _, err := manager.Create(ctx, CreateInput{Name: name}); !errors.Is(err, ErrNameInvalid) {
					t.Fatalf("Create(%q) error = %v, want ErrNameInvalid", name, err)
				}
			})
		}
		for _, name := range []string{"default", "all", "global"} {
			t.Run("Should reject reserved name "+name, func(t *testing.T) {
				t.Parallel()

				if _, err := manager.Create(ctx, CreateInput{Name: name}); !errors.Is(err, ErrNameReserved) {
					t.Fatalf("Create(%q) error = %v, want ErrNameReserved", name, err)
				}
			})
		}
		t.Run("Should reject conflicting icon and emoji", func(t *testing.T) {
			if _, err := manager.Create(
				ctx,
				CreateInput{Name: "dev", Icon: "code", Emoji: "🧑‍💻"},
			); !errors.Is(
				err,
				ErrInvalidInput,
			) {
				t.Fatalf("Create(conflicting symbol) error = %v, want ErrInvalidInput", err)
			}
		})
		t.Run("Should reject an icon outside the Lucide catalog", func(t *testing.T) {
			if _, err := manager.Create(
				ctx,
				CreateInput{Name: "ops", Icon: "not-a-lucide-icon"},
			); !errors.Is(
				err,
				ErrInvalidInput,
			) {
				t.Fatalf("Create(unknown icon) error = %v, want ErrInvalidInput", err)
			}
		})
		t.Run("Should reject a malformed emoji", func(t *testing.T) {
			if _, err := manager.Create(
				ctx,
				CreateInput{Name: "fun", Emoji: strings.Repeat("🦊", 20)},
			); !errors.Is(
				err,
				ErrInvalidInput,
			) {
				t.Fatalf("Create(oversized emoji) error = %v, want ErrInvalidInput", err)
			}
		})
		t.Run("Should reject duplicate profile names", func(t *testing.T) {
			if _, err := manager.Create(ctx, CreateInput{Name: "dev"}); err != nil {
				t.Fatalf("Create(dev) error = %v", err)
			}
			if _, err := manager.Create(ctx, CreateInput{Name: "dev"}); !errors.Is(err, ErrNameTaken) {
				t.Fatalf("Create(duplicate) error = %v, want ErrNameTaken", err)
			}
		})
		t.Run("Should reject invalid identity colors", func(t *testing.T) {
			if _, err := manager.Create(ctx, CreateInput{Name: "designer"}); err != nil {
				t.Fatalf("Create(designer) error = %v", err)
			}
			invalidColor := "red"
			if _, err := manager.UpdateIdentity(
				ctx,
				"designer",
				IdentityPatch{Color: &invalidColor},
			); !errors.Is(
				err,
				ErrInvalidInput,
			) {
				t.Fatalf("UpdateIdentity(invalid color) error = %v, want ErrInvalidInput", err)
			}
		})
		t.Run("Should reject permanent profile archive", func(t *testing.T) {
			if _, err := manager.PrepareArchive(ctx, "default"); !errors.Is(err, ErrPermanent) {
				t.Fatalf("PrepareArchive(default) error = %v, want ErrPermanent", err)
			}
		})
	})

	t.Run("Should reject stale plans without committing", func(t *testing.T) {
		t.Parallel()

		manager, _, home := newTestManager(t)
		ctx := testutil.Context(t)
		if _, err := manager.Create(ctx, CreateInput{Name: "dev"}); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		plan, err := manager.PrepareRename(ctx, "dev", "engineering")
		if err != nil {
			t.Fatalf("PrepareRename() error = %v", err)
		}
		changed := filepath.Join(home.ProfilesDir, "dev", "changed.txt")
		if err := os.WriteFile(changed, []byte("revision changed"), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		_, err = manager.Rename(ctx, "dev", RenameOptions{
			NewName: "engineering", Repos: RepoChoice{None: true}, PlanRevision: plan.Revision,
		})
		if !errors.Is(err, ErrPlanStale) {
			t.Fatalf("Rename(stale plan) error = %v, want ErrPlanStale", err)
		}
		if _, err := manager.GetByName(ctx, "dev"); err != nil {
			t.Fatalf("GetByName(dev after stale plan) error = %v", err)
		}
	})

	t.Run("Should refuse archive while a notification delivery permit is held", func(t *testing.T) {
		t.Parallel()

		manager, database, _ := newTestManager(t)
		ctx := testutil.Context(t)
		created, err := manager.Create(ctx, CreateInput{Name: "alerts"})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		plan, err := manager.PrepareArchive(ctx, created.Name)
		if err != nil {
			t.Fatalf("PrepareArchive() error = %v", err)
		}
		now := formatTimestamp(time.Now())
		if _, err := database.DB().ExecContext(ctx, `
			INSERT INTO notification_delivery_permits
			(scope_kind, profile_id, workspace_id, consumer_id, stream_name, subject_id, delivery_id, acquired_at)
			VALUES ('global', ?, '', 'terminal', 'task-events', 'task-1', 'delivery-held', ?)`,
			created.ID, now,
		); err != nil {
			t.Fatalf("insert delivery permit error = %v", err)
		}
		if _, err := manager.Archive(ctx, created.Name, plan.Revision); !errors.Is(err, ErrDeliveriesInFlight) {
			t.Fatalf("Archive(with permit) error = %v, want ErrDeliveriesInFlight", err)
		}
		stored, err := manager.GetByName(ctx, created.Name)
		if err != nil {
			t.Fatalf("GetByName() error = %v", err)
		}
		if stored.State != StateActive {
			t.Fatalf("profile state after refused archive = %q, want active", stored.State)
		}
	})
}

func seedMCPAuthProfileLifecycleRows(
	ctx context.Context,
	t *testing.T,
	database *globaldb.GlobalDB,
	profileName string,
) {
	t.Helper()
	now := "2026-08-22T00:00:00Z"
	for _, target := range []struct {
		scope string
		id    string
	}{
		{scope: "profile", id: profileName},
		{scope: "workspace_profile", id: "ws-lifecycle@pf:" + profileName},
	} {
		if _, err := database.DB().ExecContext(ctx, `
			INSERT INTO mcp_auth_tokens
				(scope, workspace_id, server_name, definition_fingerprint, issuer, client_id,
				 scopes_json, access_token_ref, obtained_at, updated_at)
			VALUES (?, ?, 'github', 'definition', 'https://issuer.example', 'client', '[]',
			        'vault:mcp/token', ?, ?)`, target.scope, target.id, now, now); err != nil {
			t.Fatalf("seed MCP auth token for %s/%s error = %v", target.scope, target.id, err)
		}
		if _, err := database.DB().ExecContext(ctx, `
			INSERT INTO mcp_oauth_registrations
				(scope, workspace_id, server_name, definition_fingerprint, resource_url, issuer,
				 client_id, redirect_uri, scopes_json, updated_at)
			VALUES (?, ?, 'github', 'definition', 'https://resource.example', 'https://issuer.example',
			        'client', 'http://127.0.0.1/callback', '[]', ?)`, target.scope, target.id, now); err != nil {
			t.Fatalf("seed MCP OAuth registration for %s/%s error = %v", target.scope, target.id, err)
		}
	}
}

func assertMCPAuthProfileLifecycleRows(
	ctx context.Context,
	t *testing.T,
	database *globaldb.GlobalDB,
	profileName string,
	want int,
) {
	t.Helper()
	var got int
	if err := database.DB().QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM mcp_auth_tokens
			 WHERE (scope = 'profile' AND workspace_id = ?)
			    OR (scope = 'workspace_profile' AND workspace_id LIKE ?)) +
			(SELECT COUNT(*) FROM mcp_oauth_registrations
			 WHERE (scope = 'profile' AND workspace_id = ?)
			    OR (scope = 'workspace_profile' AND workspace_id LIKE ?))`,
		profileName,
		"%@pf:"+profileName,
		profileName,
		"%@pf:"+profileName,
	).Scan(&got); err != nil {
		t.Fatalf("count MCP auth lifecycle rows for %q error = %v", profileName, err)
	}
	if got != want {
		t.Fatalf("MCP auth lifecycle rows for %q = %d, want %d", profileName, got, want)
	}
}

type placementCatalogFunc func(context.Context, string) ([]PlacementRef, error)

func (f placementCatalogFunc) PlacementsForProfile(
	ctx context.Context,
	profile string,
) ([]PlacementRef, error) {
	return f(ctx, profile)
}

func TestManagerSelectionResolutionAndAvailability(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve flag env remembered default and session in priority order", func(t *testing.T) {
		t.Parallel()

		manager, _, _ := newTestManager(t)
		ctx := testutil.Context(t)
		dev, err := manager.Create(ctx, CreateInput{Name: "dev"})
		if err != nil {
			t.Fatalf("Create(dev) error = %v", err)
		}
		marketing, err := manager.Create(ctx, CreateInput{Name: "marketing"})
		if err != nil {
			t.Fatalf("Create(marketing) error = %v", err)
		}
		lens := Lens{Kind: SelectionLensWorkspace, WorkspaceID: "ws-1"}
		if err := manager.PutSelection(
			ctx,
			Selection{Lens: lens.Kind, WorkspaceID: lens.WorkspaceID, ProfileID: marketing.ID},
		); err != nil {
			t.Fatalf("PutSelection() error = %v", err)
		}

		cases := []struct {
			name string
			in   ResolveInput
			want string
			src  ResolutionSource
		}{
			{
				name: "flag",
				in:   ResolveInput{Flag: "dev", Env: "marketing", Lens: lens},
				want: "dev",
				src:  ResolutionSourceFlag,
			},
			{name: "env", in: ResolveInput{Env: "dev", Lens: lens}, want: "dev", src: ResolutionSourceEnv},
			{name: "remembered", in: ResolveInput{Lens: lens}, want: "marketing", src: ResolutionSourceRemembered},
			{
				name: "default",
				in:   ResolveInput{Lens: Lens{Kind: SelectionLensGlobal}},
				want: "default",
				src:  ResolutionSourceDefault,
			},
			{
				name: "session",
				in:   ResolveInput{Flag: "dev", Env: "dev", SessionProfileID: dev.ID, Lens: lens},
				want: "dev",
				src:  ResolutionSourceSession,
			},
		}
		for _, testCase := range cases {
			t.Run("Should resolve "+testCase.name, func(t *testing.T) {
				t.Parallel()

				got, err := manager.Resolve(ctx, testCase.in)
				if err != nil {
					t.Fatalf("Resolve() error = %v", err)
				}
				if got.Profile.Name != testCase.want || got.Source != testCase.src {
					t.Fatalf("Resolve() = %#v, want name %q source %q", got, testCase.want, testCase.src)
				}
			})
		}
		if _, err := manager.Resolve(
			ctx,
			ResolveInput{Flag: "marketing", SessionProfileID: dev.ID, Lens: lens},
		); !errors.Is(
			err,
			ErrSessionConflict,
		) {
			t.Fatalf("Resolve(session conflict) error = %v, want ErrSessionConflict", err)
		}
		if _, err := manager.Resolve(ctx, ResolveInput{Flag: "missing", Lens: lens}); !errors.Is(err, ErrNotFound) {
			t.Fatalf("Resolve(missing flag) error = %v, want ErrNotFound", err)
		}
		if err := manager.PutSelection(
			ctx,
			Selection{Lens: SelectionLensGlobal, ProfileID: "@all"},
		); !errors.Is(
			err,
			ErrInvalidInput,
		) {
			t.Fatalf("PutSelection(@all) error = %v, want ErrInvalidInput", err)
		}
	})

	t.Run("Should make pending and failed lifecycle owners unavailable until done", func(t *testing.T) {
		t.Parallel()

		manager, database, _ := newTestManager(t)
		ctx := testutil.Context(t)
		dev, err := manager.Create(ctx, CreateInput{Name: "dev"})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		insertLifecycleOperation(ctx, t, database, "op_availability", dev.ID, "rename", "dev", "engineering", "failed")
		if _, err := manager.Resolve(
			ctx,
			ResolveInput{Flag: "dev", Lens: Lens{Kind: SelectionLensGlobal}},
		); !errors.Is(
			err,
			ErrUnavailable,
		) {
			t.Fatalf("Resolve(unavailable) error = %v, want ErrUnavailable", err)
		}
		if err := manager.PutSelection(
			ctx,
			Selection{Lens: SelectionLensGlobal, ProfileID: dev.ID},
		); !errors.Is(
			err,
			ErrUnavailable,
		) {
			t.Fatalf("PutSelection(unavailable) error = %v, want ErrUnavailable", err)
		}
		resolver, err := workspacepkg.NewResolver(
			database,
			workspacepkg.WithHomePaths(manager.home),
			workspacepkg.WithProfileAvailabilityChecker(manager),
		)
		if err != nil {
			t.Fatalf("workspace.NewResolver() error = %v", err)
		}
		if _, err := resolver.ResolveForProfile(ctx, "missing-workspace", dev.Name); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("ResolveForProfile(unavailable) error = %v, want ErrUnavailable", err)
		}
		preStarter := providerpkg.NewPreStarter()
		preStarter.SetProfileAvailabilityChecker(manager)
		report := preStarter.PreStart(ctx, compozyconfig.ProviderConfig{}, &providerpkg.ProbeEnv{
			PreStartScope: providerpkg.PreStartScope{ProfileID: dev.ID},
		})
		if !errors.Is(report.Cause, ErrUnavailable) {
			t.Fatalf("PreStart(unavailable).Cause = %v, want ErrUnavailable", report.Cause)
		}
		if _, err := database.DB().
			ExecContext(ctx, `UPDATE profile_lifecycle_ops SET status = 'done', completed_at = ? WHERE id = ?`, formatTimestamp(time.Now()), "op_availability"); err != nil {
			t.Fatalf("complete lifecycle operation error = %v", err)
		}
		if _, err := manager.Resolve(
			ctx,
			ResolveInput{Flag: "dev", Lens: Lens{Kind: SelectionLensGlobal}},
		); err != nil {
			t.Fatalf("Resolve(done operation) error = %v", err)
		}
	})
}

func TestManagerRecoveryAndReservation(t *testing.T) {
	t.Parallel()

	t.Run("Should resume a delete that still owes its desktop purge", func(t *testing.T) {
		t.Parallel()

		desktops := &fakeDesktopPartitions{byProfile: map[string]int{}}
		manager, database, home := newTestManagerWithDesktops(t, desktops)
		ctx := testutil.Context(t)
		profileID := strings.Repeat("D", 26)
		desktops.byProfile[profileID] = 2
		now := formatTimestamp(time.Now())
		// The catalog row is already gone: this is the crash window between a delete's
		// apply commit and its forward-only finalize, which boot has to close.
		insertLifecycleOperation(ctx, t, database, "op_delete", profileID, "delete", "doomed", "", "applied")
		path := filepath.Join(home.ProfilesDir, "doomed")
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatalf("seed profile directory error = %v", err)
		}
		if _, err := database.DB().
			ExecContext(ctx, `INSERT INTO profile_lifecycle_op_steps (op_id, seq, action, path_old, status, updated_at) VALUES ('op_delete', 0, 'remove_profile', ?, 'pending', ?)`, path, now); err != nil {
			t.Fatalf("insert remove_profile step error = %v", err)
		}
		if _, err := database.DB().
			ExecContext(ctx, `INSERT INTO profile_lifecycle_op_steps (op_id, seq, action, status, updated_at) VALUES ('op_delete', 1, 'remove_desktop_partitions', 'pending', ?)`, now); err != nil {
			t.Fatalf("insert remove_desktop_partitions step error = %v", err)
		}

		if err := manager.Recover(ctx); err != nil {
			t.Fatalf("Recover() error = %v", err)
		}
		if err := manager.Recover(ctx); err != nil {
			t.Fatalf("Recover(idempotent) error = %v", err)
		}

		if !slices.Contains(desktops.purged, profileID) {
			t.Fatalf("purged desktop partitions = %v, want profile %q", desktops.purged, profileID)
		}
		if remaining := desktops.byProfile[profileID]; remaining != 0 {
			t.Fatalf("desktop partitions after recovery = %d, want 0", remaining)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("recovered profile directory error = %v, want not exist", err)
		}
		var status string
		if err := database.DB().
			QueryRowContext(ctx, `SELECT status FROM profile_lifecycle_ops WHERE id = 'op_delete'`).
			Scan(&status); err != nil {
			t.Fatalf("read delete recovery status error = %v", err)
		}
		if status != opStatusDone {
			t.Fatalf("delete recovery status = %q, want %q", status, opStatusDone)
		}
	})

	t.Run("Should converge an applied filesystem step exactly once", func(t *testing.T) {
		t.Parallel()

		manager, database, home := newTestManager(t)
		ctx := testutil.Context(t)
		profileID := strings.Repeat("R", 26)
		now := formatTimestamp(time.Now())
		if _, err := database.DB().
			ExecContext(ctx, `INSERT INTO profiles (id, name, color, icon, state, created_at) VALUES (?, 'recovered', '#8e8eb5', 'circle', 'active', ?)`, profileID, now); err != nil {
			t.Fatalf("insert recovery profile error = %v", err)
		}
		insertLifecycleOperation(ctx, t, database, "op_recovery", profileID, "create", "", "recovered", "applied")
		path := filepath.Join(home.ProfilesDir, "recovered")
		if _, err := database.DB().
			ExecContext(ctx, `INSERT INTO profile_lifecycle_op_steps (op_id, seq, action, path_new, status, updated_at) VALUES ('op_recovery', 0, 'mkdir_profile', ?, 'pending', ?)`, path, now); err != nil {
			t.Fatalf("insert recovery step error = %v", err)
		}
		if err := manager.Recover(ctx); err != nil {
			t.Fatalf("Recover() error = %v", err)
		}
		if err := manager.Recover(ctx); err != nil {
			t.Fatalf("Recover(idempotent) error = %v", err)
		}
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			t.Fatalf("recovered path stat = %#v, %v", info, err)
		}
		var status string
		if err := database.DB().
			QueryRowContext(ctx, `SELECT status FROM profile_lifecycle_ops WHERE id = 'op_recovery'`).
			Scan(&status); err != nil {
			t.Fatalf("read recovery status error = %v", err)
		}
		if status != opStatusDone {
			t.Fatalf("recovery status = %q, want %q", status, opStatusDone)
		}
	})

	t.Run(
		"Should recover declared defaults from the journal and retain setup requirements [IT-085]",
		func(t *testing.T) {
			t.Parallel()

			manager, database, home := newTestManager(t)
			ctx := testutil.Context(t)
			const (
				profileID = "01JPROFILEGROWTH0000000000"
				opID      = "op_declared_seed"
			)
			now := formatTimestamp(time.Now())
			if _, err := database.DB().ExecContext(ctx, `INSERT INTO profiles (
			id, name, color, icon, state, created_at
		) VALUES (?, 'growth', '#5fbf85', 'chart-line', 'active', ?)`, profileID, now); err != nil {
				t.Fatalf("insert declared recovery profile error = %v", err)
			}
			insertLifecycleOperation(ctx, t, database, opID, profileID, "create", "", "growth", opStatusApplied)
			accepted := DeclaredSeed{
				Color: "#5fbf85", Icon: "chart-line",
				Defaults:       PersonaDefaults{Agent: "growth-analyst"},
				CredentialAsks: []CredentialAsk{{Provider: "openai", Slot: "api_key"}},
			}
			encoded, err := json.Marshal(accepted)
			if err != nil {
				t.Fatalf("json.Marshal(accepted seed) error = %v", err)
			}
			digest, err := fingerprint(json.RawMessage(encoded))
			if err != nil {
				t.Fatalf("fingerprint(accepted seed) error = %v", err)
			}
			if _, err := database.DB().ExecContext(ctx, `INSERT INTO profile_lifecycle_op_seed (
			op_id, color, icon, default_agent, declaration_digest
		) VALUES (?, ?, ?, ?, ?)`, opID, accepted.Color, accepted.Icon, accepted.Defaults.Agent, digest); err != nil {
				t.Fatalf("insert declared seed snapshot error = %v", err)
			}
			if _, err := database.DB().ExecContext(ctx, `INSERT INTO profile_lifecycle_op_credential_asks (
			op_id, provider, slot
		) VALUES (?, 'openai', 'api_key')`, opID); err != nil {
				t.Fatalf("insert declared credential ask error = %v", err)
			}
			if _, err := database.DB().ExecContext(ctx, `INSERT INTO profile_credential_requirements (
			profile_id, provider, slot, source_extension, declaration_digest, created_at
		) VALUES (?, 'openai', 'api_key', 'growth-kit', ?, ?)`, profileID, digest, now); err != nil {
				t.Fatalf("insert durable credential requirement error = %v", err)
			}
			profilePath := filepath.Join(home.ProfilesDir, "growth")
			if err := os.MkdirAll(profilePath, 0o700); err != nil {
				t.Fatalf("MkdirAll(profile path) error = %v", err)
			}
			if _, err := database.DB().ExecContext(ctx, `INSERT INTO profile_lifecycle_op_steps
			(op_id, seq, action, path_new, status, updated_at) VALUES
			(?, 0, 'mkdir_profile', ?, 'done', ?),
			(?, 1, 'write_declared_seed', ?, 'pending', ?)`,
				opID, profilePath, now, opID, profilePath, now,
			); err != nil {
				t.Fatalf("insert declared recovery steps error = %v", err)
			}

			accepted.Defaults.Agent = "mutated-after-crash"
			if err := manager.Recover(ctx); err != nil {
				t.Fatalf("Recover(declared seed) error = %v", err)
			}
			effective, err := compozyconfig.LoadForHome(home, compozyconfig.WithProfile("growth"))
			if err != nil {
				t.Fatalf("LoadForHome(growth) error = %v", err)
			}
			if effective.Defaults.Agent != "growth-analyst" {
				t.Fatalf("recovered defaults.agent = %q, want journaled value", effective.Defaults.Agent)
			}
			var storedDigest string
			if err := database.DB().QueryRowContext(
				ctx, `SELECT declaration_digest FROM profile_lifecycle_op_seed WHERE op_id = ?`, opID,
			).Scan(&storedDigest); err != nil {
				t.Fatalf("read declaration digest error = %v", err)
			}
			if storedDigest != digest {
				t.Fatalf("declaration digest = %q, want %q", storedDigest, digest)
			}

			insertLifecycleOperation(
				ctx,
				t,
				database,
				"op_z_retained",
				profileID,
				"archive",
				"growth",
				"growth",
				opStatusDone,
			)
			if err := manager.pruneDoneOperations(ctx, 1); err != nil {
				t.Fatalf("pruneDoneOperations() error = %v", err)
			}
			var opCount, requirementCount int
			if err := database.DB().QueryRowContext(
				ctx, `SELECT COUNT(*) FROM profile_lifecycle_ops WHERE id = ?`, opID,
			).Scan(&opCount); err != nil {
				t.Fatalf("count pruned operation error = %v", err)
			}
			if err := database.DB().QueryRowContext(
				ctx, `SELECT COUNT(*) FROM profile_credential_requirements WHERE profile_id = ?`, profileID,
			).Scan(&requirementCount); err != nil {
				t.Fatalf("count retained requirements error = %v", err)
			}
			if opCount != 0 || requirementCount != 1 {
				t.Fatalf("retention counts operation=%d requirement=%d, want 0/1", opCount, requirementCount)
			}
		},
	)

	t.Run("Should allow exactly one concurrent creator for a name", func(t *testing.T) {
		t.Parallel()

		manager, _, _ := newTestManager(t)
		ctx := testutil.Context(t)
		start := make(chan struct{})
		errs := make([]error, 2)
		var wait sync.WaitGroup
		wait.Add(2)
		for index := range errs {
			go func() {
				defer wait.Done()
				<-start
				_, errs[index] = manager.Create(ctx, CreateInput{Name: "racing"})
			}()
		}
		close(start)
		wait.Wait()
		successes, conflicts := 0, 0
		for _, err := range errs {
			switch {
			case err == nil:
				successes++
			case errors.Is(err, ErrNameTaken):
				conflicts++
			default:
				t.Fatalf("Create(racing) error = %v", err)
			}
		}
		if successes != 1 || conflicts != 1 {
			t.Fatalf("concurrent creates successes=%d conflicts=%d, want 1/1", successes, conflicts)
		}
	})
}

func newTestManager(t *testing.T) (*Manager, *globaldb.GlobalDB, compozyconfig.HomePaths) {
	return newTestManagerWithOptions(t)
}

func newTestManagerWithDesktops(
	t *testing.T,
	desktops DesktopPartitionCatalog,
) (*Manager, *globaldb.GlobalDB, compozyconfig.HomePaths) {
	return newTestManagerWithOptions(t, WithDesktopPartitionCatalog(desktops))
}

func newTestManagerWithOptions(
	t *testing.T,
	options ...Option,
) (*Manager, *globaldb.GlobalDB, compozyconfig.HomePaths) {
	t.Helper()

	home, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	database, err := globaldb.OpenGlobalDB(testutil.Context(t), home.DatabaseFile)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(context.Background()); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	managerOptions := []Option{WithStore(database), WithHomePaths(home)}
	managerOptions = append(managerOptions, options...)
	manager, err := NewManager(managerOptions...)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	return manager, database, home
}

// fakeDesktopPartitions stands in for the daemon's per-profile window managers.
// Desktops live in the client-state store rather than the catalog database, so the
// domain reaches them through this seam: the count feeds the delete preview and the
// purge is the journaled finalize step.
type fakeDesktopPartitions struct {
	mu        sync.Mutex
	byProfile map[string]int
	purged    []string
}

func (f *fakeDesktopPartitions) CountDesktopPartitions(_ context.Context, profileID string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.byProfile[profileID], nil
}

func (f *fakeDesktopPartitions) PurgeDesktopPartitions(_ context.Context, profileID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.byProfile, profileID)
	f.purged = append(f.purged, profileID)
	return nil
}

// recordingEventRecorder captures emitted lifecycle events so tests can assert
// the event payload handed to the recorder seam.
type recordingEventRecorder struct {
	mu     sync.Mutex
	events []Event
}

func (r *recordingEventRecorder) RecordProfileEvent(event Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *recordingEventRecorder) names() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	names := make([]string, 0, len(r.events))
	for _, event := range r.events {
		names = append(names, event.Name)
	}
	return names
}

func (r *recordingEventRecorder) find(name string) (Event, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, event := range r.events {
		if event.Name == name {
			return event, true
		}
	}
	return Event{}, false
}

func newTestManagerWithRecorder(t *testing.T, recorder EventRecorder) *Manager {
	manager, _, _ := newTestManagerWithOptions(t, WithEventRecorder(recorder))
	return manager
}

func insertLifecycleOperation(
	ctx context.Context,
	t *testing.T,
	database *globaldb.GlobalDB,
	opID, profileID, kind, oldName, newName, status string,
) {
	t.Helper()

	now := formatTimestamp(time.Now())
	completedAt := any(nil)
	if status == opStatusDone {
		completedAt = now
	}
	if _, err := database.DB().ExecContext(ctx, `
		INSERT INTO profile_lifecycle_ops
		(id, kind, profile_id, old_name, new_name, plan_revision, status, created_at, updated_at, completed_at)
		VALUES (?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), 'test-revision', ?, ?, ?, ?)`,
		opID, kind, profileID, oldName, newName, status, now, now, completedAt,
	); err != nil {
		t.Fatalf("insert lifecycle operation %q error = %v", opID, err)
	}
}
