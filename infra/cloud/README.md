# Compozy Cloud foundation

The root `Dockerfile` builds `compozy-cloud`. Start the service with `compozy-cloud serve` and apply additive schema migrations with `compozy-cloud migrate up` before rolling out the image. Concurrent migration commands against a schema serialize through an advisory lock on the direct Postgres endpoint.

## Environments

| Environment | Cloud host                  | Neon branch | Hosted route domain   |
| ----------- | --------------------------- | ----------- | --------------------- |
| Production  | `cloud.compozy.com`         | `main`      | `compozy.run`         |
| Staging     | `staging.cloud.compozy.com` | `staging`   | `staging.compozy.run` |

Use the environment variables listed in `production.env.example` and `staging.env.example`. Store credential values separately per environment in Cloudflare secrets and pass them into container startup.

- `COMPOZY_CLOUD_DATABASE_URL` is the pooled URL for request queries.
- `COMPOZY_CLOUD_DATABASE_DIRECT_URL` is the direct URL for `LISTEN cloud_events` and migrations. Both URLs use `sslmode=verify-full` in production and staging.
- `COMPOZY_CLOUD_SIGNING_KID` identifies the active Ed25519 key, and `COMPOZY_CLOUD_SIGNING_SEED` holds its unpadded base64url seed.
- `COMPOZY_CLOUD_JWKS_OVERLAP` optionally supplies a JWKS document of additional public Ed25519 keys. Publish the next public key on both replicas before switching the active key, then remove the previous public key after its verification window.
- `COMPOZY_CLOUD_LISTEN_ADDR` defaults to `:8080`. The Container uses this port on its private network.

The edge Worker owns the Cloudflare Containers declaration and the `cloud-0` and `cloud-1` instance names. Its configuration uses `basic`, placement in `ENAM`, a 24-hour sleep interval, and rollout steps of 50% and 100%. The image stops admitting new requests on SIGTERM, signals `going_away` to control links, and allows up to 60 seconds for the drain.

Use separate OAuth clients, Stripe prices and webhook secrets, Turnstile credentials, and tunnel API credentials for staging. Use Stripe test mode and synthetic accounts in staging.

## Cloudflare rate limits

Configure the following zone rules for the cloud host in each environment. Count requests by client IP.

| Expression                                                                                         | Threshold         |
| -------------------------------------------------------------------------------------------------- | ----------------- |
| `http.request.method eq "POST" and http.request.uri.path eq "/v1/device-flow"`                     | 10 per hour       |
| `http.request.uri.path eq "/activate" or starts_with(http.request.uri.path, "/v1/auth/callback/")` | 30 per 10 minutes |
| `http.request.uri.path eq "/v1/browser/authorize"`                                                 | 30 per 10 minutes |

Use a 429 response with the JSON code `rate_limited` and a `Retry-After` value in seconds. Exercise the zone limits and the Turnstile checks in `RT-sign-in-first-run.md` after the auth routes are integrated. Inside each replica, authenticated account writes share a 60-request allowance per account per minute.
