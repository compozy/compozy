package globaldb

import (
	"context"
	"database/sql"
	"errors"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ store.ComputerSSHSetupStore = (*ComputersRepo)(nil)

func (r *ComputersRepo) GetComputerSSHSetup(ctx context.Context, account, id string) (store.ComputerSSHSetup, error) {
	return computerSSHSetupFromRow(r.readQueries.GetComputerSSHSetup(ctx, sqlcgen.GetComputerSSHSetupParams{
		ID: id, AccountID: account,
	}))
}

func (r *ComputersRepo) FindPendingComputerSSHSetup(
	ctx context.Context, account, host string,
) (store.ComputerSSHSetup, error) {
	return computerSSHSetupFromRow(r.readQueries.FindPendingComputerSSHSetup(
		ctx, sqlcgen.FindPendingComputerSSHSetupParams{AccountID: account, HostKey: host},
	))
}

func (r *ComputersRepo) CreateComputerSSHSetup(ctx context.Context, item store.ComputerSSHSetup) error {
	return r.executeWriteQuery(
		ctx,
		"globaldb.CreateComputerSSHSetup",
		func(ctx context.Context, q *sqlcgen.Queries) error {
			return q.CreateComputerSSHSetup(ctx, sqlcgen.CreateComputerSSHSetupParams{
				ID:             item.ID,
				AccountID:      item.AccountID,
				HostKey:        item.HostKey,
				TargetAlias:    item.TargetAlias,
				Name:           item.Name,
				Kind:           item.Kind,
				Step:           item.Step,
				ReleaseVersion: item.ReleaseVersion,
				ComputerID:     item.ComputerID,
				UpdatedAt:      item.UpdatedAt,
			})
		},
	)
}

func (r *ComputersRepo) WriteComputerSSHSetup(ctx context.Context, item store.ComputerSSHSetup) error {
	return r.executeWriteQuery(
		ctx,
		"globaldb.WriteComputerSSHSetup",
		func(ctx context.Context, q *sqlcgen.Queries) error {
			count, err := q.WriteComputerSSHSetup(ctx, sqlcgen.WriteComputerSSHSetupParams{
				ID:                 item.ID,
				AccountID:          item.AccountID,
				Step:               item.Step,
				Platform:           item.Platform,
				BinaryPath:         item.BinaryPath,
				ServiceWarning:     item.ServiceWarning,
				ReservationID:      item.ReservationID,
				LinkTokenRef:       item.LinkTokenRef,
				LinkTokenExpiresAt: item.LinkTokenExpiresAt,
				ComputerID:         item.ComputerID,
				ErrorCode:          item.ErrorCode,
				ErrorDetail:        item.ErrorDetail,
				UpdatedAt:          item.UpdatedAt,
			})
			return computerRowsChanged(count, err, store.ErrComputerSSHSetupNotFound)
		},
	)
}

func computerSSHSetupFromRow(row sqlcgen.ComputerSshSetup, err error) (store.ComputerSSHSetup, error) {
	if errors.Is(err, sql.ErrNoRows) {
		return store.ComputerSSHSetup{}, store.ErrComputerSSHSetupNotFound
	}
	return store.ComputerSSHSetup{
		ID:                 row.ID,
		AccountID:          row.AccountID,
		HostKey:            row.HostKey,
		TargetAlias:        row.TargetAlias,
		Name:               row.Name,
		Kind:               row.Kind,
		Step:               row.Step,
		ReleaseVersion:     row.ReleaseVersion,
		Platform:           row.Platform,
		BinaryPath:         row.BinaryPath,
		ServiceWarning:     row.ServiceWarning,
		ReservationID:      row.ReservationID,
		LinkTokenRef:       row.LinkTokenRef,
		LinkTokenExpiresAt: row.LinkTokenExpiresAt,
		ComputerID:         row.ComputerID,
		ErrorCode:          row.ErrorCode,
		ErrorDetail:        row.ErrorDetail,
		UpdatedAt:          row.UpdatedAt,
	}, err
}
