# Remote Computers

## Cloud service

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
