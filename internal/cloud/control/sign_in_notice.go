// SPDX-License-Identifier: FSL-1.1-MIT

package control

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/cloud/auth"
	"github.com/compozy/compozy/internal/cloud/store"
)

func (s *Service) RevokeSignInNotice(
	ctx context.Context, authService *auth.Service, token, accountID string, eventID int64,
) error {
	return s.accountTransaction(ctx, accountID, func(q *store.Queries, account store.ControlAccount) error {
		if err := activeAccount(account); err != nil {
			return err
		}
		notice, err := authService.CheckNoticeSession(ctx, q, token, accountID, eventID)
		if err != nil {
			return err
		}
		device, err := findDevice(ctx, q, account.ID, notice.DeviceID)
		if errors.Is(err, errDeviceNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		return s.revokeDevice(ctx, q, account.ID, device.ID, auditSystem)
	})
}
