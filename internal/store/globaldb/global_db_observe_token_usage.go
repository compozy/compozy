package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// RecordTokenUsage writes the session aggregate and the daily rollup in one
// transaction so a rollup failure cannot leave overview usage undercounting the
// tokens already committed to token_stats.
func (g *ObserveRepo) RecordTokenUsage(
	ctx context.Context,
	stats store.TokenStatsUpdate,
	daily store.TokenUsageDailyUpdate,
) (err error) {
	_, err = g.recordTokenUsage(ctx, stats, daily, false)
	return err
}

// RecordTokenUsageOnce reports whether a canonical receipt was newly accepted.
// A replay is successful but must not re-emit accepted-usage metrics.
func (g *ObserveRepo) RecordTokenUsageOnce(
	ctx context.Context,
	stats store.TokenStatsUpdate,
	daily store.TokenUsageDailyUpdate,
) (accepted bool, err error) {
	if strings.TrimSpace(stats.TurnID) == "" || strings.TrimSpace(stats.TurnID) != stats.TurnID {
		return false, errors.New("store: canonical token receipt requires a nonblank, unpadded turn id")
	}
	if err := validateTokenReceiptPair(stats, daily); err != nil {
		return false, err
	}
	return g.recordTokenUsage(ctx, stats, daily, true)
}

// BackfillDailyBudgetReceipt recovers one durable Done receipt. Only events
// carrying the canonical writer marker can repair both additive projections.
func (g *ObserveRepo) BackfillDailyBudgetReceipt(
	ctx context.Context,
	stats store.TokenStatsUpdate,
	daily store.TokenUsageDailyUpdate,
	canonicalWriter bool,
) error {
	if err := g.checkReady(ctx, "backfill daily budget receipt"); err != nil {
		return err
	}
	if err := stats.Validate(); err != nil {
		return err
	}
	if err := daily.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(stats.TurnID) == "" || stats.TurnID != strings.TrimSpace(stats.TurnID) {
		return errors.New("store: backfill requires canonical turn id")
	}
	if err := validateTokenReceiptPair(stats, daily); err != nil {
		return err
	}
	return store.ExecuteWriteOperation(ctx, g.db, "backfill daily budget receipt", func(
		ctx context.Context, tx *store.WriteTx,
	) error {
		inserted, err := g.insertTokenUsageReceipt(ctx, tx, stats, daily)
		if err != nil || !inserted {
			return err
		}
		queries := sqlcgen.New(tx)
		if canonicalWriter {
			statsParams, err := g.tokenStatsParams(stats)
			if err != nil {
				return err
			}
			if err := queries.UpsertTokenStats(ctx, statsParams); err != nil {
				return fmt.Errorf("store: restore token stats after lost observer write: %w", err)
			}
			if err := queries.UpsertTokenUsageDaily(ctx, g.tokenUsageDailyParams(daily)); err != nil {
				return fmt.Errorf("store: restore daily usage after lost observer write: %w", err)
			}
		}
		if daily.Day == store.LocalDay(g.now()) {
			return warnForAcceptedBudgetReceipt(ctx, queries, stats.SessionID, daily.UpdatedAt)
		}
		return nil
	})
}

func validateTokenReceiptPair(stats store.TokenStatsUpdate, daily store.TokenUsageDailyUpdate) error {
	if stats.Turns > 1 || daily.Turns > 1 || stats.Turns < 0 || daily.Turns < 0 {
		return errors.New("store: a canonical token receipt must represent exactly one turn")
	}
	if stats.AgentName != daily.AgentName ||
		!equalReceiptToken(stats.InputTokens, daily.InputTokens) ||
		!equalReceiptToken(stats.OutputTokens, daily.OutputTokens) ||
		!equalReceiptToken(stats.TotalTokens, daily.TotalTokens) ||
		stats.CostStatus != daily.CostStatus || stats.CostSource != daily.CostSource ||
		!equalReceiptCost(stats.CostAmount, daily.CostAmount) ||
		!equalReceiptCurrency(stats.CostCurrency, daily.CostCurrency) {
		return errors.New("store: canonical token receipt session and daily updates disagree")
	}
	return nil
}

func equalReceiptToken(left, right *int64) bool {
	return (left == nil && right == nil) ||
		(left != nil && right != nil && *left == *right)
}

func equalReceiptCost(left, right *float64) bool {
	return (left == nil && right == nil) ||
		(left != nil && right != nil && *left == *right)
}

func equalReceiptCurrency(left, right *string) bool {
	return (left == nil && right == nil) ||
		(left != nil && right != nil && strings.TrimSpace(*left) == strings.TrimSpace(*right))
}

func (g *ObserveRepo) recordTokenUsage(
	ctx context.Context,
	stats store.TokenStatsUpdate,
	daily store.TokenUsageDailyUpdate,
	withReceipt bool,
) (accepted bool, err error) {
	if err := g.checkReady(ctx, "record token usage"); err != nil {
		return false, err
	}
	if err := stats.Validate(); err != nil {
		return false, err
	}
	if err := daily.Validate(); err != nil {
		return false, err
	}
	statsParams, err := g.tokenStatsParams(stats)
	if err != nil {
		return false, err
	}
	daily.ProfileID = strings.TrimSpace(daily.ProfileID)
	daily.Day = strings.TrimSpace(daily.Day)
	if daily.UpdatedAt.IsZero() && !withReceipt {
		daily.UpdatedAt = g.now()
	}
	dailyParams := g.tokenUsageDailyParams(daily)

	err = store.ExecuteWriteOperation(
		ctx,
		g.db,
		"record token usage",
		func(ctx context.Context, tx *store.WriteTx) error {
			// ExecuteWriteOperation may retry this closure after a busy commit.
			// Never leak acceptance from an attempt that was rolled back.
			accepted = false
			if withReceipt {
				accepted, receiptErr := g.insertTokenUsageReceipt(ctx, tx, stats, daily)
				if receiptErr != nil {
					return receiptErr
				}
				if !accepted {
					return nil
				}
			}
			queries := sqlcgen.New(tx)
			if statsErr := queries.UpsertTokenStats(ctx, statsParams); statsErr != nil {
				return fmt.Errorf("store: upsert token stats for session %q: %w", stats.SessionID, statsErr)
			}
			if dailyErr := queries.UpsertTokenUsageDaily(ctx, dailyParams); dailyErr != nil {
				return fmt.Errorf("store: upsert token usage daily for day %q: %w", daily.Day, dailyErr)
			}
			if withReceipt {
				if warningErr := warnForAcceptedBudgetReceipt(
					ctx,
					queries,
					stats.SessionID,
					daily.UpdatedAt,
				); warningErr != nil {
					return fmt.Errorf("store: assess accepted daily budget receipt warning: %w", warningErr)
				}
			}
			accepted = true
			return nil
		},
	)
	if err != nil {
		return false, err
	}
	return accepted, nil
}

// insertTokenUsageReceipt is the idempotency fence for canonical turn usage.
// The session owner and instance binding are sampled inside the same write
// transaction as both additive projections. A deleted triage session leaves
// the immutable receipt behind, so replay does not credit the budget twice.
func (g *ObserveRepo) insertTokenUsageReceipt(
	ctx context.Context,
	tx *store.WriteTx,
	stats store.TokenStatsUpdate,
	daily store.TokenUsageDailyUpdate,
) (bool, error) {
	queries := sqlcgen.New(tx)
	var receiptOwner string
	receiptOwner, err := queries.GetTokenUsageReceiptProfile(ctx, sqlcgen.GetTokenUsageReceiptProfileParams{
		SessionID: stats.SessionID, TurnID: stats.TurnID,
	})
	if err == nil {
		if receiptOwner != daily.ProfileID {
			return false, fmt.Errorf(
				"store: replayed token receipt for session %q has a different profile",
				stats.SessionID,
			)
		}
		return false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("store: check token receipt for session %q: %w", stats.SessionID, err)
	}
	// A duplicate keeps its original day even if the daemon's local timezone
	// changed. New receipts must agree with the writer's local-day boundary.
	if daily.UpdatedAt.IsZero() || daily.Day != store.LocalDay(daily.UpdatedAt) {
		return false, errors.New("store: canonical token receipt day does not match its timestamp")
	}
	sessionOwner, err := queries.GetTokenUsageReceiptSessionOwner(ctx, stats.SessionID)
	if err != nil {
		return false, fmt.Errorf("store: resolve token receipt session %q: %w", stats.SessionID, err)
	}
	if sessionOwner.ProfileID != daily.ProfileID {
		return false, fmt.Errorf("store: token receipt session %q profile does not match usage owner", stats.SessionID)
	}
	// Adoption stamps an existing standalone session with a new instance ID.
	// Receipts from before that instance existed remain standalone, including
	// when the observer first sees them during a later crash recovery. The
	// immutable receipt fence makes the same decision for live and replay.
	if sessionOwner.InstanceID != "" {
		instance, err := queries.GetAgentInstance(ctx, sessionOwner.InstanceID)
		if err != nil {
			return false, fmt.Errorf("store: read receipt instance creation: %w", err)
		}
		createdAt, err := store.ParseTimestamp(instance.CreatedAt)
		if err != nil {
			return false, err
		}
		if daily.UpdatedAt.Before(createdAt) {
			sessionOwner.InstanceID = ""
		}
	}
	tokens, err := receiptTokenCount(stats)
	if err != nil {
		return false, err
	}
	var amount sql.NullString
	if stats.CostAmount != nil && stats.CostCurrency != nil &&
		strings.EqualFold(strings.TrimSpace(*stats.CostCurrency), "USD") {
		amount = sql.NullString{String: strconv.FormatFloat(*stats.CostAmount, 'f', -1, 64), Valid: true}
	}
	receipt := sqlcgen.InsertTokenUsageReceiptParams{
		ProfileID: sessionOwner.ProfileID, SessionID: stats.SessionID, TurnID: stats.TurnID,
		InstanceID: sessionOwner.InstanceID, Day: daily.Day, TotalTokens: tokens,
		UsdAmount: amount, CostStatus: stats.CostStatus,
		CreatedAt: store.FormatTimestamp(daily.UpdatedAt),
	}
	rows, err := queries.InsertTokenUsageReceipt(ctx, receipt)
	if err != nil {
		return false, fmt.Errorf("store: insert token receipt for session %q: %w", stats.SessionID, err)
	}
	if rows == 1 && receipt.InstanceID != "" {
		if err := projectDailyBudgetReceipt(ctx, queries, receipt); err != nil {
			return false, fmt.Errorf("store: project daily budget receipt for session %q: %w", stats.SessionID, err)
		}
	}
	return rows == 1, nil
}

func receiptTokenCount(stats store.TokenStatsUpdate) (int64, error) {
	if stats.TotalTokens != nil {
		return *stats.TotalTokens, nil
	}
	var sum int64
	for _, count := range []*int64{stats.InputTokens, stats.OutputTokens} {
		if count == nil {
			continue
		}
		if *count < 0 || sum > math.MaxInt64-*count {
			return 0, errors.New("store: token receipt count is negative or overflows")
		}
		sum += *count
	}
	return sum, nil
}
