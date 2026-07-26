package cli

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/api/contract"
)

func TestProviderModelsCommands(t *testing.T) {
	t.Parallel()

	t.Run("Should list structured JSON with model source status fields", func(t *testing.T) {
		t.Parallel()

		available := true
		client := &stubClient{
			listProviderModelsFn: func(_ context.Context, query ProviderModelListQuery) (ProviderModelListRecord, error) {
				if got, want := query.ProviderID, "codex"; got != want {
					t.Fatalf("ProviderID = %q, want %q", got, want)
				}
				if got, want := query.SourceID, "config"; got != want {
					t.Fatalf("SourceID = %q, want %q", got, want)
				}
				if got, want := query.View, providerModelViewCurated; got != want {
					t.Fatalf("View = %q, want %q", got, want)
				}
				if !query.Refresh || !query.IncludeStale {
					t.Fatalf("query = %#v, want refresh and include-stale", query)
				}
				return ProviderModelListRecord{
					Models: []ProviderModelRecord{
						{
							ProviderID:        "codex",
							ModelID:           "gpt-5.4",
							Available:         &available,
							AvailabilityState: "available_live",
							Stale:             true,
							Sources: []contract.ModelCatalogSourceRefPayload{
								{SourceID: "config", SourceKind: "config", Stale: true},
							},
						},
					},
				}, nil
			},
		}

		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"provider",
			"models",
			"list",
			"codex",
			"--source",
			"config",
			"--refresh",
			"--include-stale",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("provider models list error = %v", err)
		}
		var record ProviderModelListRecord
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatalf("json.Unmarshal(list) error = %v", err)
		}
		if len(record.Models) != 1 || record.Models[0].Sources[0].SourceID != "config" || !record.Models[0].Stale {
			t.Fatalf("record = %#v, want model with source and stale fields", record)
		}
	})

	t.Run("Should request the complete view with list all", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			listProviderModelsFn: func(
				_ context.Context,
				query ProviderModelListQuery,
			) (ProviderModelListRecord, error) {
				if got, want := query.View, providerModelViewAll; got != want {
					t.Fatalf("View = %q, want %q", got, want)
				}
				return ProviderModelListRecord{Models: []ProviderModelRecord{
					{ProviderID: "codex", ModelID: "gpt-5.6-sol", Curated: true},
					{ProviderID: "codex", ModelID: "gpt-5.6-legacy", Deprecated: true},
				}}, nil
			},
		}

		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"provider",
			"models",
			"list",
			"codex",
			"--all",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("provider models list --all error = %v", err)
		}
		var record ProviderModelListRecord
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatalf("json.Unmarshal(list all) error = %v", err)
		}
		if got, want := len(record.Models), 2; got != want {
			t.Fatalf("model count = %d, want %d", got, want)
		}
	})

	t.Run("Should curate a model with explicit true and false flags", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			curateProviderModelFn: func(
				_ context.Context,
				providerID string,
				request ProviderModelCurationRequest,
			) (ProviderModelCurationRecord, error) {
				if providerID != "codex" || request.ModelID != "gpt-5.6-sol" {
					t.Fatalf("curation identity = %q/%q", providerID, request.ModelID)
				}
				if request.Hidden == nil || *request.Hidden || request.Featured == nil || !*request.Featured ||
					request.Deprecated == nil || *request.Deprecated || request.DefaultReasoningEffort == nil ||
					*request.DefaultReasoningEffort != "max" {
					t.Fatalf("curation request = %#v, want hidden=false featured=true deprecated=false max", request)
				}
				maxEffort := contract.ReasoningEffort("max")
				return ProviderModelCurationRecord{
					Model: contract.ProviderModelPayload{
						ProviderID:             "codex",
						ModelID:                "gpt-5.6-sol",
						Curated:                true,
						Featured:               true,
						DefaultReasoningEffort: &maxEffort,
					},
					Apply: contract.SettingsApplyResponse{
						Applied:          true,
						Lifecycle:        contract.SettingsApplyLifecycle("live"),
						ActiveGeneration: 2,
					},
				}, nil
			},
		}

		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"provider",
			"models",
			"set",
			"codex",
			"gpt-5.6-sol",
			"--hidden=false",
			"--featured=true",
			"--deprecated=false",
			"--default-effort=max",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("provider models set error = %v", err)
		}
		var record ProviderModelCurationRecord
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatalf("json.Unmarshal(curation) error = %v", err)
		}
		if !record.Model.Curated || !record.Model.Featured || !record.Apply.Applied {
			t.Fatalf("curation record = %#v, want featured curated applied model", record)
		}
	})

	t.Run("Should render active_generation in curation toon output", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			curateProviderModelFn: func(
				_ context.Context,
				_ string,
				_ ProviderModelCurationRequest,
			) (ProviderModelCurationRecord, error) {
				return ProviderModelCurationRecord{
					Model: contract.ProviderModelPayload{
						ProviderID: "codex",
						ModelID:    "gpt-5.6-sol",
						Curated:    true,
					},
					Apply: contract.SettingsApplyResponse{
						Applied:          true,
						Lifecycle:        contract.SettingsApplyLifecycle("live"),
						ActiveGeneration: 2,
					},
				}, nil
			},
		}

		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"provider",
			"models",
			"set",
			"codex",
			"gpt-5.6-sol",
			"--featured=true",
			"-o",
			"toon",
		)
		if err != nil {
			t.Fatalf("provider models set toon error = %v", err)
		}
		if !strings.Contains(stdout, "active_generation") || !strings.Contains(stdout, "2") {
			t.Fatalf("curation toon = %q, want active_generation field with value 2", stdout)
		}
	})

	t.Run("Should refresh and print source statuses", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			refreshProviderModelsFn: func(
				_ context.Context,
				providerID string,
				request ProviderModelRefreshRequest,
			) (ProviderModelRefreshRecord, error) {
				if got, want := providerID, "codex"; got != want {
					t.Fatalf("providerID = %q, want %q", got, want)
				}
				if got, want := request.SourceID, "models_dev"; got != want {
					t.Fatalf("SourceID = %q, want %q", got, want)
				}
				if !request.Force || request.RequestID != "req-1" {
					t.Fatalf("request = %#v, want force request req-1", request)
				}
				return ProviderModelRefreshRecord{
					Sources: []ProviderModelSourceStatusRecord{
						{
							ProviderID:   "codex",
							SourceID:     "models_dev",
							SourceKind:   "models_dev",
							RefreshState: "succeeded",
							RowCount:     2,
						},
					},
				}, nil
			},
		}

		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"provider",
			"models",
			"refresh",
			"codex",
			"--source",
			"models_dev",
			"--force",
			"--request-id",
			"req-1",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("provider models refresh error = %v", err)
		}
		var record ProviderModelRefreshRecord
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatalf("json.Unmarshal(refresh) error = %v", err)
		}
		if len(record.Sources) != 1 || record.Sources[0].RefreshState != "succeeded" {
			t.Fatalf("record = %#v, want source status", record)
		}
	})

	t.Run("Should show status for provider", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			providerModelStatusFn: func(_ context.Context, providerID string) (ProviderModelStatusRecord, error) {
				if got, want := providerID, "codex"; got != want {
					t.Fatalf("providerID = %q, want %q", got, want)
				}
				return ProviderModelStatusRecord{
					Sources: []ProviderModelSourceStatusRecord{
						{ProviderID: "codex", SourceID: "config", RefreshState: "succeeded", RowCount: 1},
					},
				}, nil
			},
		}

		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"provider",
			"models",
			"status",
			"codex",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("provider models status error = %v", err)
		}
		var record ProviderModelStatusRecord
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatalf("json.Unmarshal(status) error = %v", err)
		}
		if len(record.Sources) != 1 || record.Sources[0].SourceID != "config" {
			t.Fatalf("record = %#v, want config status", record)
		}
	})

	t.Run("Should surface daemon service unavailable errors", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			listProviderModelsFn: func(context.Context, ProviderModelListQuery) (ProviderModelListRecord, error) {
				return ProviderModelListRecord{}, errors.New("model catalog service unavailable")
			},
		}

		_, _, err := executeRootCommand(t, newTestDeps(t, client), "provider", "models", "list", "-o", "json")
		if err == nil {
			t.Fatal("provider models list error = nil, want service unavailable")
		}
		if !strings.Contains(err.Error(), "model catalog service unavailable") {
			t.Fatalf("provider models list error = %v, want service unavailable", err)
		}
	})
}
