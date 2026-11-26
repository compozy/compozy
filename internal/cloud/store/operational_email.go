// SPDX-License-Identifier: FSL-1.1-MIT

package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/compozy/compozy/internal/cloud/store/sqlcgen"
)

type OperationalMail struct {
	AccountID, Kind, Email, ComputerName, Key string
	DeleteAt                                  time.Time
}

func (q *Queries) DueOperationalMail(ctx context.Context, asOf time.Time, account string) ([]OperationalMail, error) {
	rows, err := q.generated.DueOperationalMail(ctx, sqlcgen.DueOperationalMailParams{
		AsOf: timestamp(asOf), ForAccount: account,
	})
	if err != nil {
		return nil, err
	}
	result := make([]OperationalMail, len(rows))
	for i, row := range rows {
		result[i] = OperationalMail{
			AccountID: row.AccountID, Kind: row.Kind, Email: row.Email, ComputerName: row.ComputerName,
			Key: row.DeliveryKey, DeleteAt: row.DeleteAt.Time,
		}
	}
	return result, nil
}

func (s *Store) DeliverOperationalMail(
	ctx context.Context, asOf time.Time, candidate OperationalMail, send func(OperationalMail) error,
) error {
	return s.Transaction(ctx, func(q *Queries) error {
		_, err := q.generated.LockOperationalMailAccount(ctx, candidate.AccountID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		// Lock the same account billing locks so a renewal wins if it committed first.
		due, err := q.DueOperationalMail(ctx, asOf, candidate.AccountID)
		if err != nil {
			return err
		}
		for _, message := range due {
			if message.Key != candidate.Key {
				continue
			}
			if err := send(message); err != nil {
				return err
			}
			_, err = q.generated.CompleteOperationalMail(ctx, sqlcgen.CompleteOperationalMailParams{
				AccountID: message.AccountID, DeliveryKey: message.Key,
			})
			return err
		}
		return nil
	})
}
