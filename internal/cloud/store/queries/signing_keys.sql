-- SPDX-License-Identifier: FSL-1.1-MIT
-- name: RetiredSigningKeys :many
SELECT kid FROM cloud_retired_signing_keys ORDER BY kid;

-- name: RetireSigningKey :execrows
INSERT INTO cloud_retired_signing_keys (kid, retired_at) VALUES ($1, $2) ON CONFLICT DO NOTHING;
