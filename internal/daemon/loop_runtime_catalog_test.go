package daemon

import (
	"context"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/modelcatalog"
)

func TestLoopRuntimeCatalogShouldRespectProviderAndModelAuthority(t *testing.T) {
	t.Parallel()

	t.Run("Should canonicalize provider aliases through config authority", func(t *testing.T) {
		t.Parallel()

		catalog := loopRuntimeCatalog{}
		if got := catalog.CanonicalProvider("Z-AI"); got != "zai" {
			t.Fatalf("CanonicalProvider(Z-AI) = %q, want zai", got)
		}
	})

	t.Run("Should reject providers absent from the effective config", func(t *testing.T) {
		t.Parallel()

		catalog := loopRuntimeCatalog{config: &compozyconfig.Config{}}
		err := catalog.ValidateRuntime(t.Context(), looppkg.RuntimeSpec{Provider: "flarp"})
		assertLoopRuntimeCatalogValidation(t, err, "provider", "flarp", "unknown_provider")
	})

	t.Run("Should accept slash models for non-authoritative providers without a catalog read", func(t *testing.T) {
		t.Parallel()

		catalog := loopRuntimeCatalog{config: &compozyconfig.Config{}}
		err := catalog.ValidateRuntime(t.Context(), looppkg.RuntimeSpec{
			Provider: "openrouter",
			Model:    "anthropic/claude-opus-4-7",
		})
		if err != nil {
			t.Fatalf("ValidateRuntime(non-authoritative slash model) error = %v", err)
		}
	})

	t.Run("Should validate authoritative models through the offline-safe catalog projection", func(t *testing.T) {
		t.Parallel()

		models := &runtimeCatalogModelService{models: []modelcatalog.Model{{
			ProviderID: "cursor",
			ModelID:    "grok-4.5[effort=high,fast=true]",
		}}}
		catalog := loopRuntimeCatalog{config: &compozyconfig.Config{}, models: models}
		err := catalog.ValidateRuntime(t.Context(), looppkg.RuntimeSpec{
			Provider: "cursor",
			Model:    "grok-4.5[effort=high,fast=true]",
		})
		if err != nil {
			t.Fatalf("ValidateRuntime(authoritative model) error = %v", err)
		}
		if models.listCalls != 1 || models.lastList.ProviderID != "cursor" ||
			models.lastList.Refresh || !models.lastList.SkipRefreshIfEmpty ||
			!models.lastList.IncludeAll || !models.lastList.IncludeStale ||
			models.lastList.View != modelcatalog.CatalogViewAll {
			t.Fatalf("ListModels options = %#v, want offline-safe authoritative lookup", models.lastList)
		}
	})

	t.Run("Should reject a model absent from an authoritative catalog", func(t *testing.T) {
		t.Parallel()

		catalog := loopRuntimeCatalog{
			config: &compozyconfig.Config{},
			models: &runtimeCatalogModelService{},
		}
		err := catalog.ValidateRuntime(t.Context(), looppkg.RuntimeSpec{
			Provider: "cursor",
			Model:    "unknown",
		})
		assertLoopRuntimeCatalogValidation(t, err, "model", "unknown", "unknown_model")
	})
}

func assertLoopRuntimeCatalogValidation(
	t *testing.T,
	err error,
	field string,
	value string,
	reason string,
) {
	t.Helper()
	validation, ok := looppkg.AsRuntimeValidationError(err)
	if !ok || len(validation.Items) != 1 {
		t.Fatalf("runtime validation error = %v, want one item", err)
	}
	item := validation.Items[0]
	if item.Field != field || item.Value != value || item.Reason != reason {
		t.Fatalf("runtime validation item = %#v, want %s=%q/%s", item, field, value, reason)
	}
}

type runtimeCatalogModelService struct {
	models    []modelcatalog.Model
	listCalls int
	lastList  modelcatalog.ListOptions
}

func (s *runtimeCatalogModelService) ListModels(
	_ context.Context,
	opts modelcatalog.ListOptions,
) ([]modelcatalog.Model, error) {
	s.listCalls++
	s.lastList = opts
	return append([]modelcatalog.Model(nil), s.models...), nil
}

func (*runtimeCatalogModelService) Refresh(
	context.Context,
	modelcatalog.RefreshOptions,
) ([]modelcatalog.SourceStatus, error) {
	return nil, nil
}

func (*runtimeCatalogModelService) ListSourceStatus(
	context.Context,
	string,
) ([]modelcatalog.SourceStatus, error) {
	return nil, nil
}
