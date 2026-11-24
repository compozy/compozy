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

## Sign-in

- Set `COMPOZY_CLOUD_GOOGLE_CLIENT_ID` and `COMPOZY_CLOUD_GOOGLE_CLIENT_SECRET` for a Google Web application client. Set `COMPOZY_CLOUD_GITHUB_CLIENT_ID` and `COMPOZY_CLOUD_GITHUB_CLIENT_SECRET` for a GitHub OAuth App. Register `https://<cloud-host>/v1/auth/callback/google` and `https://<cloud-host>/v1/auth/callback/github` on the respective apps.
- Enable the Google OIDC `auth_time` claim in the Google Auth Platform security settings. Admin recovery requires a fresh authentication timestamp for the current activation attempt.
- Configure an invisible Turnstile widget for the cloud host. Set `COMPOZY_CLOUD_TURNSTILE_SITE_KEY` and `COMPOZY_CLOUD_TURNSTILE_SECRET_KEY` for the widget. The server checks the hostname and expects the actions `activate` and `browser_authorize`.
- Set `COMPOZY_CLOUD_PKCE_ENC_KEY` to an unpadded base64url encoding of 32 random bytes. Both replicas in the environment use the same value for OAuth verifier and refresh credential encryption. Changing it invalidates outstanding provider callbacks and existing refresh credentials, so coordinate rotation across both replicas.

Apply the cloud schema migration before enabling sign-in. Cloudflare supplies the request location and client IP to private replicas. The service stores the coarse location and uses the IP only for Turnstile verification.

## Account email and metrics

Verify the sender domain `mail.compozy.com` in Resend and publish its SPF, DKIM, and DMARC records. Set `COMPOZY_CLOUD_RESEND_API_KEY` and `COMPOZY_CLOUD_RESEND_NEWS_SEGMENT` for each environment. The optional `COMPOZY_CLOUD_RESEND_FROM` defaults to `no-reply@mail.compozy.com`. The API key needs access to send mail and manage contacts.

Create the Resend contact property `compozy_unsubscribe_url`. Set `COMPOZY_CLOUD_UNSUBSCRIBE_KEY` to an unpadded base64url encoding of 32 random bytes and use the same key on both replicas. Keep the key stable for already-sent links. Use the contact property for the opt-out link in product news. A sender supplying RFC 8058 headers uses the contact's HTTPS URL for `List-Unsubscribe` and `List-Unsubscribe=One-Click` for `List-Unsubscribe-Post`. Include the company's postal address in the message footer.

To scrape account counts, set `COMPOZY_CLOUD_METRICS_TOKEN` and send it as a bearer credential to `GET /metrics`. The page exposes aggregate counts by first sign-in provider and activity windows of 1, 7, and 30 days. Installation counts include session, link, and token sources.

## Billing

- Set `STRIPE_PRICE_PRO_MONTHLY` and `STRIPE_PRICE_PRO_ANNUAL` to the same environment's recurring Managed Payments prices. Supply `COMPOZY_CLOUD_STRIPE_SECRET_KEY` to the cloud containers.
- Register `https://<cloud-host>/v1/billing/stripe-webhook` with Stripe and set `COMPOZY_CLOUD_STRIPE_WEBHOOK_SECRET` to that endpoint's signing secret. Subscribe it to `customer.subscription.created`, `customer.subscription.updated`, `customer.subscription.deleted`, `invoice.paid`, `invoice.payment_failed`, and `checkout.session.completed`.
- Configure the Customer Portal to switch between both prices, update payment methods, and cancel at the end of the period. Configure Smart Retries to cancel after the final failed attempt.

Apply the billing migration before starting the containers. Use test-mode Checkout to walk the standalone purchase and the retained-hostname purchase before enabling live billing.

## Cloudflare rate limits

Configure the following zone rules for the cloud host in each environment. Count requests by client IP.

| Expression                                                                                         | Threshold         |
| -------------------------------------------------------------------------------------------------- | ----------------- |
| `http.request.method eq "POST" and http.request.uri.path eq "/v1/device-flow"`                     | 10 per hour       |
| `http.request.uri.path eq "/activate" or starts_with(http.request.uri.path, "/v1/auth/callback/")` | 30 per 10 minutes |
| `http.request.uri.path eq "/v1/browser/authorize"`                                                 | 30 per 10 minutes |
| `http.request.uri.path eq "/billing/upgrade/sign-in"`                                              | 30 per 10 minutes |

Use a 429 response with the JSON code `rate_limited` and a `Retry-After` value in seconds. Exercise the zone limits and the Turnstile checks in `RT-sign-in-first-run.md` after the auth routes are integrated. Inside each replica, authenticated account writes share a 60-request allowance per account per minute.
