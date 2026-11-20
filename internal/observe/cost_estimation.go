package observe

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/store"
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
) (compozyconfig.ProviderAuthMode, error)

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

// usageSnapshotForDone keeps live delivery and ledger replay on the same
// immutable provider/model recorded when that particular prompt began. A
// session's current runtime can change before a delayed Done is observed.
func (o *Observer) usageSnapshotForDone(
	ctx context.Context, sessionID string, snapshot observedSession, event acp.AgentEvent,
) observedSession {
	runtime := event.PromptRuntimeSnapshot()
	if runtime == nil {
		return snapshot
	}
	oldProvider, oldModel := snapshot.provider, snapshot.model
	snapshot.provider, snapshot.model = strings.TrimSpace(runtime.Provider), strings.TrimSpace(runtime.Model)
	if mode := compozyconfig.ProviderAuthMode(runtime.MeteringAuthMode); knownMeteringAuthMode(mode) {
		snapshot.authMode = mode
		snapshot.suppressCostEstimate = false
		return snapshot
	}
	if o.resolveProviderAuth == nil {
		if snapshot.provider != oldProvider || snapshot.model != oldModel {
			// A stale auth mode from another runtime cannot prove a metered
			// price (nor that the turn was covered by an included CLI plan).
			snapshot.authMode = ""
			snapshot.suppressCostEstimate = true
		}
		return snapshot
	}
	authMode, err := o.resolveProviderAuth(ctx, snapshot.agentName,
		snapshot.provider, snapshot.model, snapshot.workspaceID)
	if err != nil {
		o.logger.WarnContext(ctx, "observe: completed turn provider auth unavailable",
			"session_id", sessionID, "turn_id", event.TurnID, "error", err)
		snapshot.authMode = ""
		snapshot.suppressCostEstimate = true
	} else {
		snapshot.authMode = authMode
		snapshot.suppressCostEstimate = false
	}
	return snapshot
}

func knownMeteringAuthMode(mode compozyconfig.ProviderAuthMode) bool {
	switch mode {
	case compozyconfig.ProviderAuthModeNone,
		compozyconfig.ProviderAuthModeBoundSecret,
		compozyconfig.ProviderAuthModeNativeCLI:
		return true
	default:
		return false
	}
}

func (o *Observer) observedCostFields(
	ctx context.Context,
	snapshot observedSession,
	usage *acp.TokenUsage,
) (*float64, *string, string, string) {
	if usage == nil {
		if !snapshot.suppressCostEstimate && snapshot.authMode == compozyconfig.ProviderAuthModeNativeCLI {
			return nil, nil, string(modelcatalog.CostStatusIncluded), string(modelcatalog.CostSourceNone)
		}
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
	// A historical runtime whose provider-auth mode can no longer be resolved
	// may have used an included native-CLI subscription. The current model
	// catalog cannot prove that turn incurred metered USD cost.
	if snapshot.suppressCostEstimate {
		return nil, nil, string(modelcatalog.CostStatusUnknown), string(modelcatalog.CostSourceNone)
	}
	if snapshot.authMode == compozyconfig.ProviderAuthModeNativeCLI {
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
	if !hasBudgetUsageEvidence(event) {
		return nil
	}
	snapshot = o.usageSnapshotForDone(ctx, sessionID, snapshot, event)
	stats, daily := o.observedUsageUpdates(ctx, sessionID, snapshot, event, timestamp)
	if !shouldAggregateUsage(event) {
		// A real provider call without telemetry can still incur USD. Its
		// budget receipt must never invent a turn in the legacy aggregates.
		if receipts, ok := o.registry.(dailyBudgetReceiptRecovery); ok {
			return receipts.BackfillDailyBudgetReceipt(ctx, stats, daily, false)
		}
		return nil
	}
	var accepted bool
	var err error
	if receipts, ok := o.registry.(interface {
		RecordTokenUsageOnce(context.Context, store.TokenStatsUpdate, store.TokenUsageDailyUpdate) (bool, error)
	}); ok {
		// The normal prompt pump guarantees canonical turn IDs. Never fall
		// back to additive writes when an attributable receipt lacks one.
		accepted, err = receipts.RecordTokenUsageOnce(ctx, stats, daily)
	} else {
		err = o.registry.RecordTokenUsage(ctx, stats, daily)
		accepted = err == nil
	}
	if err != nil {
		return fmt.Errorf("observe: record token usage: %w", err)
	}
	if !accepted {
		return nil // replay: neither additive totals nor usage metrics change
	}
	o.eventMetrics.RecordUsage(ctx, AcceptedUsage{
		Provider:         snapshot.provider,
		Model:            snapshot.model,
		Agent:            snapshot.agentName,
		InputTokens:      derefOrZero(event.Usage.InputTokens),
		OutputTokens:     derefOrZero(event.Usage.OutputTokens),
		CacheReadTokens:  derefOrZero(event.Usage.CacheReadTokens),
		CacheWriteTokens: derefOrZero(event.Usage.CacheWriteTokens),
		CostAmount:       derefOrZero(stats.CostAmount),
		CostCurrency:     derefOrZero(stats.CostCurrency),
		CostStatus:       modelcatalog.CostStatus(stats.CostStatus),
	})
	return nil
}

func (o *Observer) observedUsageUpdates(
	ctx context.Context,
	sessionID string,
	snapshot observedSession,
	event acp.AgentEvent,
	timestamp time.Time,
) (store.TokenStatsUpdate, store.TokenUsageDailyUpdate) {
	usageTimestamp := timestamp
	costAmount, costCurrency, costStatus, costSource := o.observedCostFields(ctx, snapshot, event.Usage)
	usage := event.Usage
	if usage == nil {
		usage = &acp.TokenUsage{}
	}
	if !usage.Timestamp.IsZero() {
		usageTimestamp = usage.Timestamp
	}
	// Commit the session aggregate and the daily rollup atomically so a rollup
	// failure cannot leave overview usage undercounting committed token_stats.
	stats := store.TokenStatsUpdate{
		CacheReadTokens:  usage.CacheReadTokens,
		CacheWriteTokens: usage.CacheWriteTokens,
		SessionID:        sessionID,
		TurnID:           event.TurnID,
		AgentName:        snapshot.agentName,
		InputTokens:      usage.InputTokens,
		OutputTokens:     usage.OutputTokens,
		TotalTokens:      usage.TotalTokens,
		CostAmount:       costAmount,
		CostCurrency:     costCurrency,
		CostStatus:       costStatus,
		CostSource:       costSource,
		Turns:            1,
		UpdatedAt:        usageTimestamp,
	}
	daily := store.TokenUsageDailyUpdate{
		ProfileID:    snapshot.profileID,
		Day:          store.LocalDay(usageTimestamp),
		WorkspaceID:  snapshot.workspaceID,
		AgentName:    snapshot.agentName,
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		TotalTokens:  usage.TotalTokens,
		CostAmount:   costAmount,
		CostCurrency: costCurrency,
		CostStatus:   costStatus,
		CostSource:   costSource,
		Turns:        1,
		UpdatedAt:    usageTimestamp,
	}
	return stats, daily
}

func derefOrZero[T any](value *T) T {
	if value == nil {
		var zero T
		return zero
	}
	return *value
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
