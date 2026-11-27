// SPDX-License-Identifier: FSL-1.1-MIT

package store

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/cloud/store/sqlcgen"
)

func (q *Queries) ChargeAuthIPWindow(
	ctx context.Context, scope string, clientIPHash []byte, endsAt time.Time, limit int32,
) (bool, error) {
	rows, err := q.generated.ChargeAuthIPWindow(ctx, sqlcgen.ChargeAuthIPWindowParams{
		Scope: scope, ClientIpSha256: clientIPHash, ExpiresAt: timestamp(endsAt), RequestLimit: limit,
	})
	return rows != 0, err
}
