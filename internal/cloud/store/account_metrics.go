package store

import (
	"context"
	"time"
)

type AccountTotal struct {
	Provider string
	Total    int64
}

type AccountActivity struct {
	Accounts, Sessions, Links, Tokens int64
}

func (q *Queries) CloudAccountTotals(ctx context.Context) ([]AccountTotal, error) {
	rows, err := q.generated.CloudAccountTotals(ctx)
	if err != nil {
		return nil, err
	}
	totals := make([]AccountTotal, 0, len(rows))
	for _, row := range rows {
		totals = append(totals, AccountTotal(row))
	}
	return totals, nil
}

func (q *Queries) CloudAccountActivity(ctx context.Context, since time.Time) (AccountActivity, error) {
	row, err := q.generated.CloudActiveCounts(ctx, timestamp(since))
	return AccountActivity(row), err
}
