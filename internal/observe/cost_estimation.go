package observe

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/modelcatalog"
	"github.com/compozy/agh/internal/store"
)

// CostCatalog is the merged model projection consumed by usage estimation.
type CostCatalog interface {
	ListModels(context.Context, modelcatalog.ListOptions) ([]modelcatalog.Model, error)
}

// ProviderAuthModeResolver resolves the effective auth owner for one session runtime.
type ProviderAuthModeResolver func(
	ctx context.Context,
	agentName string,
	provider string,
	model string,
	workspaceID string,
) (aghconfig.ProviderAuthMode, error)

// WithCostCatalog injects the merged model catalog used for silent-agent estimation.
func WithCostCatalog(catalog CostCatalog) Option {
	return func(observer *Observer) {
		observer.costCatalog = catalog
	}
}

// WithProviderAuthModeResolver overrides provider auth resolution for session snapshots.
func WithProviderAuthModeResolver(resolver ProviderAuthModeResolver) Option {
	return func(observer *Observer) {
		observer.resolveProviderAuth = resolver
	}
}

func (o *Observer) observedCostFields(
	ctx context.Context,
	snapshot observedSession,
	usage *acp.TokenUsage,
) (*float64, *string, string, string) {
	if usage == nil {
		return nil, nil, string(modelcatalog.CostStatusUnknown), string(modelcatalog.CostSourceNone)
	}
	if usage.CostAmount != nil {
		if usage.CostCurrency == nil || strings.TrimSpace(*usage.CostCurrency) == "" ||
			*usage.CostAmount < 0 || math.IsNaN(*usage.CostAmount) || math.IsInf(*usage.CostAmount, 0) {
			return nil, nil, string(modelcatalog.CostStatusUnknown), string(modelcatalog.CostSourceNone)
		}
		currency := strings.TrimSpace(*usage.CostCurrency)
		return usage.CostAmount,
			&currency,
			string(modelcatalog.CostStatusActual),
			string(modelcatalog.CostSourceAgentReported)
	}
	if snapshot.authMode == aghconfig.ProviderAuthModeNativeCLI {
		return nil, nil, string(modelcatalog.CostStatusIncluded), string(modelcatalog.CostSourceNone)
	}
	if o.costCatalog == nil || snapshot.provider == "" || snapshot.model == "" {
		return nil, nil, string(modelcatalog.CostStatusUnknown), string(modelcatalog.CostSourceNone)
	}

	models, err := o.costCatalog.ListModels(ctx, modelcatalog.ListOptions{
		ProviderID:         snapshot.provider,
		View:               modelcatalog.CatalogViewAll,
		SkipRefreshIfEmpty: true,
		IncludeAll:         true,
		IncludeStale:       true,
	})
	if err != nil {
		o.logger.Warn(
			"observe: estimate usage cost failed",
			"agent_name", snapshot.agentName,
			"provider", snapshot.provider,
			"model", snapshot.model,
			"workspace_id", snapshot.workspaceID,
			"error", err,
		)
		return nil, nil, string(modelcatalog.CostStatusUnknown), string(modelcatalog.CostSourceNone)
	}
	model := findObservedModel(models, snapshot.provider, snapshot.model)
	result, ok := modelcatalog.EstimateCost(model, modelcatalog.TokenBuckets{
		Input:      usage.InputTokens,
		Output:     usage.OutputTokens,
		CacheRead:  usage.CacheReadTokens,
		CacheWrite: usage.CacheWriteTokens,
		Reasoning:  usage.ThoughtTokens,
	})
	if !ok {
		return nil, nil, string(modelcatalog.CostStatusUnknown), string(modelcatalog.CostSourceNone)
	}
	amount := result.Amount
	currency := result.Currency
	return &amount, &currency, string(result.Status), string(result.Source)
}

func (o *Observer) aggregateObservedUsage(
	ctx context.Context,
	sessionID string,
	snapshot observedSession,
	event acp.AgentEvent,
	timestamp time.Time,
) error {
	if !shouldAggregateUsage(event) {
		return nil
	}

	usageTimestamp := timestamp
	if !event.Usage.Timestamp.IsZero() {
		usageTimestamp = event.Usage.Timestamp
	}

	costAmount, costCurrency, costStatus, costSource := o.observedCostFields(ctx, snapshot, event.Usage)
	// Commit the session aggregate and the daily rollup atomically so a rollup
	// failure cannot leave overview usage undercounting committed token_stats.
	if err := o.registry.RecordTokenUsage(ctx,
		store.TokenStatsUpdate{
			SessionID:    sessionID,
			AgentName:    snapshot.agentName,
			InputTokens:  event.Usage.InputTokens,
			OutputTokens: event.Usage.OutputTokens,
			TotalTokens:  event.Usage.TotalTokens,
			CostAmount:   costAmount,
			CostCurrency: costCurrency,
			CostStatus:   costStatus,
			CostSource:   costSource,
			Turns:        1,
			UpdatedAt:    usageTimestamp,
		},
		store.TokenUsageDailyUpdate{
			Day:          store.LocalDay(usageTimestamp),
			WorkspaceID:  snapshot.workspaceID,
			AgentName:    snapshot.agentName,
			InputTokens:  event.Usage.InputTokens,
			OutputTokens: event.Usage.OutputTokens,
			TotalTokens:  event.Usage.TotalTokens,
			CostAmount:   costAmount,
			CostCurrency: costCurrency,
			CostStatus:   costStatus,
			CostSource:   costSource,
			Turns:        1,
			UpdatedAt:    usageTimestamp,
		},
	); err != nil {
		return fmt.Errorf("observe: record token usage: %w", err)
	}
	return nil
}

func findObservedModel(models []modelcatalog.Model, provider string, model string) *modelcatalog.Model {
	provider = strings.TrimSpace(provider)
	model = strings.TrimSpace(model)
	for index := range models {
		if strings.TrimSpace(models[index].ProviderID) == provider &&
			strings.TrimSpace(models[index].ModelID) == model {
			return &models[index]
		}
	}
	return nil
}
