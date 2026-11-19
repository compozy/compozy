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
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

const dailyBudgetReceiptPageSize = 128

// ReadDailyBudgetUsage reads a stable snapshot of canonical receipts for one
// instance's local day. Historical rollups are deliberately not added because
// they predate the per-turn receipt fence and cannot be safely deduplicated.
func (r *InstanceRepo) ReadDailyBudgetUsage(
	ctx context.Context,
	instanceID string,
	at time.Time,
) (usage agentinstance.DailyBudgetUsage, err error) {
	if err = r.checkReady(ctx, "read instance daily budget usage"); err != nil {
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
	return readDailyBudgetUsage(ctx, q, instance.ProfileID, instanceID, store.LocalDay(at))
}

// readDailyBudgetUsage also supports the caller's BEGIN IMMEDIATE transaction
// during turn admission, so a usage snapshot and the admission fence can agree.
func readDailyBudgetUsage(
	ctx context.Context,
	q *sqlcgen.Queries,
	profileID, instanceID, day string,
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
			if row.TotalTokens < 0 || usage.Tokens > math.MaxInt64-row.TotalTokens {
				return agentinstance.DailyBudgetUsage{}, errors.New("store: daily token receipt total overflows")
			}
			usage.Tokens += row.TotalTokens
			if !row.UsdAmount.Valid {
				usage.HasUnpricedUsage = true
				continue
			}
			if row.UsdAmount.String != "-0" && !isReceiptDecimalUSD(row.UsdAmount.String) {
				return agentinstance.DailyBudgetUsage{}, errors.New("store: invalid daily receipt USD decimal")
			}
			amount, ok := new(big.Rat).SetString(row.UsdAmount.String)
			if !ok || amount.Sign() < 0 {
				return agentinstance.DailyBudgetUsage{}, errors.New("store: invalid daily receipt USD amount")
			}
			if usage.USD == nil {
				usage.USD = amount
			} else {
				usage.USD.Add(usage.USD, amount)
			}
		}
		if len(rows) < dailyBudgetReceiptPageSize {
			return usage, nil
		}
		last := rows[len(rows)-1]
		params.AfterSessionID, params.AfterTurnID = last.SessionID, last.TurnID
	}
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
