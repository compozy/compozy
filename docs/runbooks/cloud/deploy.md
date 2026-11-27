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

| Required GitHub environment variable | Worker and container binding |
| --- | --- |
| `COMPOZY_CLOUD_STRIPE_PORTAL_CONFIG_ID` | `COMPOZY_CLOUD_STRIPE_PORTAL_CONFIG_ID` holds the `bpc_…` configuration ID for the same Stripe environment as the prices. |

Generate the Ed25519 configuration with `compozy-cloud keys generate <kid>` and save the output directly to the environment secret. Generate independent 32-byte random values for the PKCE encryption and unsubscribe keys. Encode both as base64url without padding. Rotate non-signing secrets yearly and after suspected exposure. Rotate the vendor credential, deploy and verify with the replacement, then revoke the previous credential. For PKCE, expect in-flight sign-ins to restart. For unsubscribe links, coordinate the change with the contact sync and current broadcast links.

## Before public launch

- Unpause the two Better Stack monitors after the first production smoke. Confirm SPF and DKIM for `mail.compozy.com` and DMARC alignment with a delivered staging message.
- Publish the Google OAuth app after the terms and privacy pages are public. Confirm staging and production callbacks against their respective OAuth apps.
- Run the staging restore drill before launch and every quarter.
- File the Public Suffix List entry for `compozy.run` at least eight weeks before stable. Publish the abuse contact page and confirm PSL eligibility first. The operator adds the verification `_psl` TXT record and submits the change to `publicsuffix/list`. Add the staging rule if the submission requires isolated site boundaries for `*.staging.compozy.run`.
- Configure and verify a permanent redirect from `https://compozy.app` to `https://compozy.com`. The operator owns the registrar, DNS and redirect.

## Hosted route certificates

The tunnel provisioner creates a proxied DNS record for each assigned hostname. In a full Cloudflare zone, Universal SSL covers a single label under the zone. A deployment with deeper names such as `<assigned-host>.staging.<zone>` needs an active edge certificate for `*.staging.<zone>` before verification. Order an advanced wildcard certificate for that namespace if it needs coverage. Check the assigned URL with normal TLS verification. The Cloud service health endpoint uses a separate hostname.

The Cloudflare API can confirm record creation before every resolver can return the record. A resolver can continue to return a cached negative answer after publication. The host retries a negative lookup through Cloudflare's public resolver for the readiness probe. The fallback uses TCP port 53. The probe verifies TLS for the assigned hostname and checks the computer descriptor before it reports `ready`.

## Hosted webhook ingress

Create a Configuration Rule in the hosted route zone for [issue #2063](https://github.com/compozy/compozy/issues/2063).
Name it `hosted-webhooks-skip-bic (#2063)`. Set Browser Integrity Check to **Off**.
The Rulesets API uses phase `http_config_settings`, action `set_config`, and action parameters `{"bic": false}`.

The control plane assigns `<slug>-<id>.<zone>` in production and `<slug>-<id>.staging.<zone>` in staging.
The ID suffix contains four lowercase base32 characters. The rule matches one assigned label and the two webhook POST path prefixes.
The daemon still checks the binding and HMAC signature.

Replace `<zone>` with the hosted route zone and `<service-zone>` with the public service zone.
The offsets below assume a 12-byte production suffix, including the leading dot.
If the suffix length changes, add the same difference to each length and negative offset magnitude.

```text
(http.request.method eq "POST"
  and (
    starts_with(http.request.uri.path, "/api/webhooks/global/")
    or starts_with(http.request.uri.path, "/api/webhooks/workspaces/")
  )
  and not (http.host in {
    "<zone>" "www.<zone>" "cloud.<zone>" "api.<zone>" "app.<zone>"
    "staging.<zone>" "staging.cloud.<zone>" "cloud.staging.<zone>" "api.staging.<zone>" "app.staging.<zone>"
    "<service-zone>" "www.<service-zone>" "cloud.<service-zone>" "app.<service-zone>" "staging.cloud.<service-zone>"
  })
  and (
    (
      http.host wildcard "*-*.<zone>"
      and not (http.host wildcard "*.*.<zone>")
      and len(http.host) gt 17
      and substring(http.host, -17, -16) eq "-"
      and trim(substring(http.host, -16, -12), "abcdefghijklmnopqrstuvwxyz234567") eq ""
    )
    or (
      http.host wildcard "*-*.staging.<zone>"
      and not (http.host wildcard "*.*.staging.<zone>")
      and len(http.host) gt 25
      and substring(http.host, -25, -24) eq "-"
      and trim(substring(http.host, -24, -20), "abcdefghijklmnopqrstuvwxyz234567") eq ""
    )
  )
)
```

1. Send a signed webhook POST from Python's default `urllib` client in staging.
2. Check for a daemon response. A 404 with the daemon's `Referrer-Policy: no-referrer` confirms ingress for an unused endpoint.
3. Send a descriptor GET with the same User-Agent. Check for Cloudflare HTTP 403 with error 1010.

Related procedures [Monitoring and on-call](monitoring.md), [Signing keys](key-rotation.md), [Restore drill](../../qa/scenarios/RT-cloud-restore-drill.md).
