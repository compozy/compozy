package globaldb

import (
	"context"
	"database/sql"
	"encoding/json/v2"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/account"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ account.Store = (*AccountRepo)(nil)

func (r *AccountRepo) ReadInstallationAccount(ctx context.Context) (account.Installation, error) {
	if err := r.checkReady(ctx, "read installation account"); err != nil {
		return account.Installation{}, err
	}
	row, err := r.readQueries.ReadInstallationAccount(ctx)
	if err != nil {
		return account.Installation{}, fmt.Errorf("read installation account: %w", err)
	}
	var plan remoteproto.AccountPlan
	if row.PlanJson.Valid {
		if err := json.Unmarshal([]byte(row.PlanJson.String), &plan); err != nil {
			return account.Installation{}, err
		}
	}
	result := account.Installation{
		InstallationID: row.InstallationID, PreAccountInstall: row.PreAccountInstall == 1,
		State: account.State(row.State), Source: row.Source.String, AccountID: row.AccountID.String,
		SignedOutReason: row.SignedOutReason.String, AuthGeneration: row.AuthGeneration,
		GatePolicyJWS: row.GatePolicyJws.String, GatePolicyKeys: row.GatePolicyKeys.String,
		TokenName: row.TokenName.String,
		Profile: remoteproto.AccountSummary{
			AccountPlan: plan, Email: row.Email.String, Name: row.Name.String, Provider: row.Provider.String,
			MarketingOptIn: row.MarketingOptIn == 1, Device: remoteproto.AccountDevice{
				ID: remoteproto.DeviceID(
					row.DeviceID.String,
				),
				Name:  row.DeviceName.String,
				Admin: row.DeviceAdmin.Int64 == 1,
			},
		},
	}
	for _, field := range []struct {
		value sql.NullString
		dst   *time.Time
	}{
		{row.LastValidatedAt, &result.LastValidatedAt},
		{row.GatePolicyFetchedAt, &result.GatePolicyFetchedAt},
		{row.OfflineGraceStartedAt, &result.OfflineGraceStartedAt},
		{row.SignInNoticesDismissedAt, &result.SignInNoticesDismissedAt},
	} {
		if field.value.Valid {
			if *field.dst, err = store.ParseTimestamp(field.value.String); err != nil {
				return account.Installation{}, fmt.Errorf("read installation account timestamp: %w", err)
			}
		}
	}
	return result, nil
}

func (r *AccountRepo) SaveInstallationAccount(
	ctx context.Context,
	previous int64,
	installation account.Installation,
) error {
	if err := r.checkReady(ctx, "save installation account"); err != nil {
		return err
	}
	profile := installation.Profile
	params := sqlcgen.SaveInstallationAccountParams{
		PreviousGeneration: previous,
		AuthGeneration:     installation.AuthGeneration,
		State:              string(installation.State),
		Source:             accountText(installation.Source),
		AccountID:          accountText(installation.AccountID),
		Email:              accountText(profile.Email),
		Name:               accountText(profile.Name),
		Provider:           accountText(profile.Provider),
		DeviceID:           accountText(string(profile.Device.ID)),
		DeviceName:         accountText(profile.Device.Name),
		DeviceAdmin:        sql.NullInt64{Int64: accountBit(profile.Device.Admin), Valid: profile.Device.ID != ""},
		MarketingOptIn:     accountBit(profile.MarketingOptIn),
		SignedOutReason:    accountText(installation.SignedOutReason),
		LastValidatedAt:    accountText(store.FormatNullableTimestamp(installation.LastValidatedAt)),
		GatePolicyJws: accountText(
			installation.GatePolicyJWS,
		),
		GatePolicyKeys:           accountText(installation.GatePolicyKeys),
		GatePolicyFetchedAt:      accountText(store.FormatNullableTimestamp(installation.GatePolicyFetchedAt)),
		OfflineGraceStartedAt:    accountText(store.FormatNullableTimestamp(installation.OfflineGraceStartedAt)),
		TokenName:                accountText(installation.TokenName),
		SignInNoticesDismissedAt: accountText(store.FormatNullableTimestamp(installation.SignInNoticesDismissedAt)),
	}
	if profile.Plan != "" {
		plan, err := json.Marshal(profile.AccountPlan)
		if err != nil {
			return err
		}
		params.PlanJson = accountText(string(plan))
	}
	return r.executeWriteQuery(
		ctx,
		"globaldb.SaveInstallationAccount",
		func(ctx context.Context, queries *sqlcgen.Queries) error {
			rows, err := queries.SaveInstallationAccount(ctx, params)
			if err != nil {
				return err
			}
			if rows != 1 {
				return errors.New("installation account generation changed")
			}
			return nil
		},
	)
}

func accountText(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func accountBit(value bool) int64 {
	if value {
		return 1
	}
	return 0
}
