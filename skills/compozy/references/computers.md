# Remote Computers

## Compozy Cloud

`https://cloud.compozy.com` serves the Remote Computers control plane. Agent sessions and their data stay on their computers.

| Endpoint                                       | Purpose                                               |
| ---------------------------------------------- | ----------------------------------------------------- |
| `GET /healthz`                                 | Check replica readiness                               |
| `GET /v1/jwks` or `GET /.well-known/jwks.json` | Read the cloud's public Ed25519 verification keys     |
| `GET /v1/gate-policy`                          | Fetch the signed sign-in policy in compact JWS format |

Verify the gate policy signature against the cloud JWKS. A grace policy applies only before its `until` deadline. For `429 rate_limited` and `503 cloud_unavailable`, honor the `Retry-After` header.

## Local account

Use `compozy account login` to sign in through the browser. Show the activation URL and code to the operator and let them approve the grant. Use `compozy account status --json` to read the account, plan usage, and current device. Use `compozy account logout` to end the session on this computer. `compozy account upgrade --annual` opens annual checkout when billing is available.

Local HTTP and UDS expose the same `/api/account` routes for sign-in, status, devices, approvals, and billing. Subscribe to `GET /api/account/sign-in/events` before starting a login. It carries attempt progress and state without account details. Follow `GET /api/account/stream` after sign-in. On `stream_reset`, read current status and approvals before following changes again.

A cloud outage leaves an existing sign-in intact. Retry account actions after the response's `Retry-After` when present. The account tool and routes are local to the installation.

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

## This computer

Use `compozy remote link` to link this computer and turn on its remote access. Use the printed activation URL and user code if sign-in is needed. SSH setup pipes the single-use link token to `compozy remote link --token-stdin`.

Read status with `compozy remote status --json` or `compozy__remote_status`. The tool is local to this computer and has no remote scope. Use `compozy remote disable` to close the listeners and keep the account link, `compozy remote enable` to reopen them, and `compozy remote unlink` to remove the account link and directory entry. Unlinking uses a device session from the linked account.

The same operations are available over HTTP and UDS under `/api/remote`. Use `compozy remote devices list --json` to read effective scopes for devices that opened a session or requested approval on this computer. Narrow one device with `compozy remote devices scopes <device> --set <scopes>`. Approve or deny a pending key with `compozy remote devices approve|deny <device>`, using `--approval-id` to choose among multiple pending keys. `compozy remote devices revoke <device>` revokes that device on the account.
