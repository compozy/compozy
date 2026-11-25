# Deploy Compozy Cloud

A main push that changes the cloud binary, service, shared protocol or edge Worker runs the staging deploy after the Go lanes and preflight succeed. The deploy applies Goose migrations through the direct Neon endpoint, rolls the containers in 50 percent and 100 percent steps and checks readiness, JWKS and device-flow creation. The readiness response identifies the commit and deploy attempt baked into the container image.

Use the direct Neon URL for the non-superuser role that owns the Compozy Cloud database.

After staging succeeds, create and push an immutable `cloud-v<semver>` tag on that exact main commit. The workflow verifies the staging job for that commit before asking the GitHub `production` environment reviewer to approve. Production uses the same migrations, gradual rollout and smoke sequence. Restrict creation of `cloud-v*` tags to release operators and keep the required reviewer enabled on `production`.

## Runtime bindings

Set these secrets in each GitHub deploy environment. Wrangler stores them separately on each Cloudflare Worker version and the Worker passes them to its containers. Store deploy credentials only in GitHub Actions.

| Service | Names |
| --- | --- |
| Neon | `COMPOZY_CLOUD_NEON_POOLED_URL`, `COMPOZY_CLOUD_NEON_DIRECT_URL` with `sslmode=verify-full` |
| Signing and encryption | `COMPOZY_CLOUD_SIGNING_KEYS`, `COMPOZY_CLOUD_PKCE_ENC_KEY`, `COMPOZY_CLOUD_UNSUBSCRIBE_KEY` |
| OAuth | `COMPOZY_CLOUD_GOOGLE_CLIENT_SECRET`, `COMPOZY_CLOUD_GITHUB_CLIENT_SECRET` |
| Turnstile | `COMPOZY_CLOUD_TURNSTILE_SECRET_KEY` |
| Hosted routes | `COMPOZY_CLOUD_CLOUDFLARE_TUNNEL_API_TOKEN` |
| Deployment only | `COMPOZY_CLOUD_CLOUDFLARE_DEPLOY_API_TOKEN` |
| Stripe | `COMPOZY_CLOUD_STRIPE_SECRET_KEY`, `COMPOZY_CLOUD_STRIPE_WEBHOOK_SECRET` |
| Resend | `COMPOZY_CLOUD_RESEND_API_KEY`, `COMPOZY_CLOUD_RESEND_CONTACTS_API_KEY` |
| Observability | `COMPOZY_CLOUD_SENTRY_DSN`, `COMPOZY_CLOUD_GRAFANA_PROM_TOKEN`, optional `COMPOZY_CLOUD_METRICS_TOKEN` for authenticated account metrics |

Use GitHub environment variables for `COMPOZY_CLOUD_GOOGLE_CLIENT_ID`, `COMPOZY_CLOUD_GITHUB_CLIENT_ID`, `COMPOZY_CLOUD_TURNSTILE_SITE_KEY`, `COMPOZY_CLOUD_CLOUDFLARE_ACCOUNT_ID`, `COMPOZY_CLOUD_CLOUDFLARE_RUN_ZONE_ID`, `STRIPE_PRICE_PRO_MONTHLY`, `STRIPE_PRICE_PRO_ANNUAL`, `COMPOZY_CLOUD_RESEND_NEWS_SEGMENT`, `COMPOZY_CLOUD_GRAFANA_PROM_URL` and `COMPOZY_CLOUD_GRAFANA_PROM_USER`. Set the Access and product news variables listed in the [cloud admin and product news runbook](admin-and-product-news.md) on the same GitHub environment.

Generate the Ed25519 configuration with `compozy-cloud keys generate <kid>` and save the output directly to the environment secret. Generate independent 32-byte random values for the PKCE encryption and unsubscribe keys. Encode both as base64url without padding. Rotate non-signing secrets yearly and after suspected exposure. Rotate the vendor credential, deploy and verify with the replacement, then revoke the previous credential. For PKCE, expect in-flight sign-ins to restart. For unsubscribe links, coordinate the change with the contact sync and current broadcast links.

## Before public launch

- Unpause the two Better Stack monitors after the first production smoke. Confirm SPF and DKIM for `mail.compozy.com` and DMARC alignment with a delivered staging message.
- Publish the Google OAuth app after the terms and privacy pages are public. Confirm staging and production callbacks against their respective OAuth apps.
- Run the staging restore drill before launch and every quarter.
- File the Public Suffix List entry for `compozy.run` at least eight weeks before stable. Publish the abuse contact page and confirm PSL eligibility first. The operator adds the verification `_psl` TXT record and submits the change to `publicsuffix/list`. Add the staging rule if the submission requires isolated site boundaries for `*.staging.compozy.run`.
- Configure and verify a permanent redirect from `https://compozy.app` to `https://compozy.com`. The operator owns the registrar, DNS and redirect.

Related procedures [Monitoring and on-call](monitoring.md), [Signing keys](key-rotation.md), [Restore drill](../../qa/scenarios/RT-cloud-restore-drill.md).
