# Remote Computers

## Cloud service

`https://cloud.compozy.com` serves the Remote Computers control plane. Agent sessions and their data stay on their computers.

| Endpoint                                       | Purpose                                               |
| ---------------------------------------------- | ----------------------------------------------------- |
| `GET /healthz`                                 | Check replica readiness                               |
| `GET /v1/jwks` or `GET /.well-known/jwks.json` | Read the cloud's public Ed25519 verification keys     |
| `GET /v1/gate-policy`                          | Fetch the signed sign-in policy in compact JWS format |

Verify the gate policy signature against the cloud JWKS. A grace policy applies only before its `until` deadline. For `429 rate_limited` and `503 cloud_unavailable`, honor the `Retry-After` header.

## Cloud account and computer access

- `GET /v1/devices` lists the account's devices. An admin can promote or demote another device, edit its scopes, or revoke it with the device routes. Each trust change increments the account's security epoch.
- `GET /v1/computers` lists the account's linked computers. Use `POST /v1/computers/reservations` to reserve a slot before SSH setup. On resume, send `previous_reservation_id` again. Release the reservation when setup is cancelled. A link token can be redeemed again by the same computer key to recover the original link result.
- Request a fresh grant for each connection attempt at `POST /v1/computers/{id}/grants` with the route kind. Use the bound device key. Free grants cover `direct` and `ssh`. Compozy Pro also covers `cloud`.
- Follow `GET /v1/account/events` for device, computer, plan, and approval changes. After a reconnect, read the current state. A decision accepted for delivery can be checked at `GET /v1/approvals/{id}` until the computer provides its `applied_result`.

Keep link tokens, grants, computer key proofs and private keys out of logs and tool results.
