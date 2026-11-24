#!/usr/bin/env bash
set -euo pipefail

umask 077
bindings=$(mktemp "$RUNNER_TEMP/cloud-bindings.XXXXXX")
trap 'rm -f "$bindings"' EXIT

node --input-type=module - "$bindings" <<'JS'
import { writeFileSync } from "node:fs";

const required = [
  "COMPOZY_CLOUD_NEON_DIRECT_URL", "COMPOZY_CLOUD_NEON_POOLED_URL",
  "COMPOZY_CLOUD_SIGNING_KEYS", "COMPOZY_CLOUD_PKCE_ENC_KEY", "COMPOZY_CLOUD_UNSUBSCRIBE_KEY",
  "COMPOZY_CLOUD_GOOGLE_CLIENT_ID", "COMPOZY_CLOUD_GOOGLE_CLIENT_SECRET",
  "COMPOZY_CLOUD_GITHUB_CLIENT_ID", "COMPOZY_CLOUD_GITHUB_CLIENT_SECRET",
  "COMPOZY_CLOUD_STRIPE_SECRET_KEY", "COMPOZY_CLOUD_STRIPE_WEBHOOK_SECRET",
  "STRIPE_PRICE_PRO_MONTHLY", "STRIPE_PRICE_PRO_ANNUAL",
  "COMPOZY_CLOUD_TURNSTILE_SITE_KEY", "COMPOZY_CLOUD_TURNSTILE_SECRET_KEY",
  "COMPOZY_CLOUD_CLOUDFLARE_ACCOUNT_ID", "COMPOZY_CLOUD_CLOUDFLARE_RUN_ZONE_ID",
  "COMPOZY_CLOUD_CLOUDFLARE_TUNNEL_API_TOKEN", "COMPOZY_CLOUD_RESEND_API_KEY",
  "COMPOZY_CLOUD_RESEND_NEWS_SEGMENT", "COMPOZY_CLOUD_SENTRY_DSN", "COMPOZY_CLOUD_GRAFANA_PROM_URL",
  "COMPOZY_CLOUD_GRAFANA_PROM_USER", "COMPOZY_CLOUD_GRAFANA_PROM_TOKEN",
];
const optional = ["COMPOZY_CLOUD_METRICS_TOKEN"];
const missing = required.filter((name) => !process.env[name]);
if (missing.length) {
  console.error("Missing cloud environment bindings:", missing.join(", "));
  process.exit(1);
}
const values = [...required, ...optional]
  .filter((name) => process.env[name])
  .map((name) => [name, process.env[name]]);
writeFileSync(process.argv[2], JSON.stringify(Object.fromEntries(values)), { mode: 0o600 });
JS

args=(deploy --config wrangler.toml --secrets-file "$bindings")
if [[ "$COMPOZY_CLOUD_ENV" == staging ]]; then
  args+=(--env staging)
fi
if [[ -n "${COMPOZY_CLOUD_EXPECTED_RESTORE_POINT:-}" ]]; then
  args+=(--var "COMPOZY_CLOUD_EXPECTED_RESTORE_POINT:$COMPOZY_CLOUD_EXPECTED_RESTORE_POINT")
fi
printf '%s.%s.%s\n' "${CLOUD_REVISION:?missing cloud revision}" "$GITHUB_RUN_ID" "$GITHUB_RUN_ATTEMPT" \
  > infra/cloud-edge/build-revision.txt
cd infra/cloud-edge
bunx --package wrangler@4.168.0 wrangler "${args[@]}"
