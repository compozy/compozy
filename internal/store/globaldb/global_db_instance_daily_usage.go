package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

const dailyBudgetReceiptPageSize = 128

var errDailyBudgetTotalsInvalid = agentinstance.ErrDailyBudgetUsageUnavailable

// ReadDailyBudgetUsage reads a stable snapshot of canonical receipts for one
// instance's local day. Historical rollups are deliberately not added because
// they predate the per-turn receipt fence and cannot be safely deduplicated.
func (r *InstanceRepo) ReadDailyBudgetUsage(
	ctx context.Context,
	instanceID string,
	at time.Time,
) (usage agentinstance.DailyBudgetUsage, err error) {
	if err := r.checkReady(ctx, "read instance daily budget usage"); err != nil {
		return usage, err
	}
	if instanceID == "" || strings.TrimSpace(instanceID) != instanceID || at.IsZero() {
		return usage, errors.New("agentinstance: daily usage requires canonical instance id and time")
	}
	tx, err := r.readDB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return usage, fmt.Errorf("store: begin daily budget usage snapshot: %w", err)
	}
	defer func() {
		if closeErr := tx.Rollback(); closeErr != nil && !errors.Is(closeErr, sql.ErrTxDone) {
			err = errors.Join(err, fmt.Errorf("store: close daily budget usage snapshot: %w", closeErr))
		}
	}()
	q := sqlcgen.New(tx)
	instance, err := q.GetAgentInstance(ctx, instanceID)
	if errors.Is(err, sql.ErrNoRows) {
		return usage, agentinstance.ErrNotFound
	}
	if err != nil {
		return usage, fmt.Errorf("store: resolve daily budget owner: %w", err)
	}
	if instance.ArchivedAt.Valid {
		return usage, agentinstance.ErrNotFound
	}
	uncertain, err := countUncertainBudgetDay(ctx, q, instanceID, at)
	if err != nil {
		return usage, err
	}
	if uncertain {
		return usage, agentinstance.ErrDailyBudgetUsageUnavailable
	}
	return readDailyBudgetUsage(ctx, q, instance.ProfileID, instanceID, store.LocalDay(at))
}

// readDailyBudgetUsage also supports the caller's BEGIN IMMEDIATE transaction
// during turn admission, so a usage snapshot and the admission fence can agree.
func readDailyBudgetUsage(
	ctx context.Context,
	q *sqlcgen.Queries,
	profileID, instanceID, day string,
) (agentinstance.DailyBudgetUsage, error) {
	usage, found, err := loadDailyBudgetTotal(ctx, q, profileID, instanceID, day)
	if found || err != nil {
		return usage, err
	}
	return scanDailyBudgetReceipts(ctx, q, profileID, instanceID, day)
}

func scanDailyBudgetReceipts(
	ctx context.Context, q *sqlcgen.Queries, profileID, instanceID, day string,
) (agentinstance.DailyBudgetUsage, error) {
	var usage agentinstance.DailyBudgetUsage
	params := sqlcgen.ListInstanceDailyBudgetReceiptPageParams{
		ProfileID: profileID, InstanceID: instanceID, Day: day, PageSize: dailyBudgetReceiptPageSize,
	}
	for {
		rows, err := q.ListInstanceDailyBudgetReceiptPage(ctx, params)
		if err != nil {
			return agentinstance.DailyBudgetUsage{}, fmt.Errorf("store: read daily token receipts: %w", err)
		}
		for _, row := range rows {
			if err := addDailyBudgetReceipt(&usage, row.TotalTokens, row.UsdAmount, row.CostStatus); err != nil {
				return agentinstance.DailyBudgetUsage{}, err
			}
		}
		if len(rows) < dailyBudgetReceiptPageSize {
			return usage, nil
		}
		last := rows[len(rows)-1]
		params.AfterSessionID, params.AfterTurnID = last.SessionID, last.TurnID
	}
}

func addDailyBudgetReceipt(
	usage *agentinstance.DailyBudgetUsage, tokens int64, usd sql.NullString, costStatus string,
) error {
	if tokens < 0 || usage.Tokens > math.MaxInt64-tokens {
		return fmt.Errorf("%w: token total overflows", errDailyBudgetTotalsInvalid)
	}
	usage.Tokens += tokens
	if !usd.Valid {
		if costStatus != string(modelcatalog.CostStatusIncluded) {
			usage.HasUnpricedUsage = true
		}
		return nil
	}
	if usd.String != "-0" && !isReceiptDecimalUSD(usd.String) {
		return fmt.Errorf("%w: invalid receipt USD decimal", errDailyBudgetTotalsInvalid)
	}
	amount, ok := new(big.Rat).SetString(usd.String)
	if !ok || amount.Sign() < 0 {
		return fmt.Errorf("%w: invalid receipt USD amount", errDailyBudgetTotalsInvalid)
	}
	if usage.USD == nil {
		usage.USD = amount
	} else {
		usage.USD.Add(usage.USD, amount)
	}
	return nil
}

func loadDailyBudgetTotal(
	ctx context.Context, q *sqlcgen.Queries, profileID, instanceID, day string,
) (agentinstance.DailyBudgetUsage, bool, error) {
	row, err := q.ReadInstanceDailyBudgetTotal(ctx, sqlcgen.ReadInstanceDailyBudgetTotalParams{
		ProfileID: profileID, InstanceID: instanceID, Day: day,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return agentinstance.DailyBudgetUsage{}, false, nil
	}
	if err != nil {
		return agentinstance.DailyBudgetUsage{}, false, err
	}
	if row.Invalid != 0 {
		return agentinstance.DailyBudgetUsage{}, true, errDailyBudgetTotalsInvalid
	}
	usage := agentinstance.DailyBudgetUsage{Tokens: row.TotalTokens, HasUnpricedUsage: row.HasUnpricedUsage != 0}
	if row.UsdTotal.Valid {
		var ok bool
		usage.USD, ok = new(big.Rat).SetString(row.UsdTotal.String)
		if !ok || usage.USD.RatString() != row.UsdTotal.String {
			return agentinstance.DailyBudgetUsage{}, true, errDailyBudgetTotalsInvalid
		}
	}
	if err := usage.Validate(); err != nil {
		return agentinstance.DailyBudgetUsage{}, true, fmt.Errorf("%w: %w", errDailyBudgetTotalsInvalid, err)
	}
	return usage, true, nil
}

func projectDailyBudgetReceipt(
	ctx context.Context, q *sqlcgen.Queries, receipt sqlcgen.InsertTokenUsageReceiptParams,
) error {
	usage, found, err := loadDailyBudgetTotal(ctx, q, receipt.ProfileID, receipt.InstanceID, receipt.Day)
	if err == nil {
		if found {
			err = addDailyBudgetReceipt(&usage, receipt.TotalTokens, receipt.UsdAmount, receipt.CostStatus)
		} else {
			// Include older receipts when a day gets its first projection.
			usage, err = scanDailyBudgetReceipts(ctx, q, receipt.ProfileID, receipt.InstanceID, receipt.Day)
		}
	}
	params := sqlcgen.WriteInstanceDailyBudgetTotalParams{
		InstanceID: receipt.InstanceID, ProfileID: receipt.ProfileID, Day: receipt.Day,
	}
	switch {
	case errors.Is(err, errDailyBudgetTotalsInvalid):
		// Preserve accepted receipts while failing closed on budget reads for this day.
		params.Invalid = 1
	case err != nil:
		return err
	default:
		params.TotalTokens = usage.Tokens
		if usage.USD != nil {
			params.UsdTotal = sql.NullString{String: usage.USD.RatString(), Valid: true}
		}
		if usage.HasUnpricedUsage {
			params.HasUnpricedUsage = 1
		}
	}
	return q.WriteInstanceDailyBudgetTotal(ctx, params)
}

// Receipt costs are stored as base-ten non-negative decimals, never ratios,
// signed amounts, hexadecimal floats, exponent strings or special values.
func isReceiptDecimalUSD(value string) bool {
	if value == "" || strings.Count(value, ".") > 1 || value[0] == '.' || value[len(value)-1] == '.' {
		return false
	}
	for _, char := range value {
		if char != '.' && (char < '0' || char > '9') {
			return false
		}
	}
	return true
}
