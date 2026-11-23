# Remote Computers

## Compozy Cloud

`https://cloud.compozy.com` serves the Remote Computers control plane. Agent sessions and their data stay on their computers.

| Endpoint                                       | Purpose                                               |
| ---------------------------------------------- | ----------------------------------------------------- |
| `GET /healthz`                                 | Check replica readiness                               |
| `GET /v1/jwks` or `GET /.well-known/jwks.json` | Read the cloud's public Ed25519 verification keys     |
| `GET /v1/gate-policy`                          | Fetch the signed sign-in policy in compact JWS format |

Verify the gate policy signature against the cloud JWKS. A grace policy applies only before its `until` deadline. For `429 rate_limited` and `503 cloud_unavailable`, honor the `Retry-After` header.

## Cloud sign-in

Start a device grant with `POST /v1/device-flow`. Show the operator the returned user code and `verification_uri_complete`. The operator compares the code with `/activate` and completes Google or GitHub sign-in in the browser. Poll `POST /v1/device-flow/token` at the returned interval. Add five seconds on `slow_down`, and stop on `access_denied` or `expired_token`. Cancel a superseded attempt with `DELETE /v1/device-flow/{device_code}`.

Enroll with `POST /v1/devices/enroll` using the single-use enrollment token and a DPoP proof from the per-account device key. `POST /v1/session/refresh` rotates the refresh credential. `POST /v1/session/logout` revokes that session and keeps its enrolled device. Keep device codes, enrollment and refresh credentials, and OAuth callback URLs out of tool results and logs.

Use the Google activation page's **Recover admin access** action after losing the last active admin device. The provider must verify a fresh sign-in.

## Cloud account and computer access

- `GET /v1/devices` lists the account's devices. An admin can promote or demote another device, edit its scopes, or revoke it with the device routes. Each trust change increments the account's security epoch.
- `GET /v1/computers` lists the account's linked computers. Use `POST /v1/computers/reservations` to reserve a slot before SSH setup. On resume, send `previous_reservation_id` again. Release the reservation when setup is cancelled. A link token can be redeemed again by the same computer key to recover the original link result.
- Request a fresh grant for each connection attempt at `POST /v1/computers/{id}/grants` with the route kind. Use the bound device key. Free grants cover `direct` and `ssh`. Compozy Pro also covers `cloud`.
- Follow `GET /v1/account/events` for device, computer, plan, and approval changes. After a reconnect, read the current state. A decision accepted for delivery can be checked at `GET /v1/approvals/{id}` until the computer provides its `applied_result`.

Keep link tokens, grants, computer key proofs and private keys out of logs and tool results.
