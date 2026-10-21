# Key and CLI credential collectors

`RegisterKeyCollectors(registry, options...)` adds `openrouter`, `vercel-ai-gateway`,
`zai`, `moonshot`, `kimi-cli`, `cursor`, and `gemini` to an existing registry. It does
not replace Codex or Claude collectors. The daemon composition root supplies the
registry to the service. Constructor options inject an origin, HTTP transport, or
clock for deterministic tests; they do not resolve credentials from environment
variables or add user configuration.

All four bound-key collectors read `ResolvedRoute.BoundSecrets["api_key"]`. The
builtin slots target `OPENROUTER_API_KEY`, `AI_GATEWAY_API_KEY`, `ZAI_API_KEY`, and
`KIMI_API_KEY` respectively. Those environment names are not slot names. `moonshot`
measures **Kimi Coding**, not Moonshot Open Platform balance.

CLI stores are read only from the resolved route:

| Collector | Credential location | Source label |
| --- | --- | --- |
| OpenRouter | Bound key | OpenRouter API key |
| Vercel AI Gateway | Bound key | Vercel AI Gateway API key |
| z.ai | Bound key | z.ai API key |
| Kimi Coding | Bound key | Kimi Coding API key |
| Kimi CLI | `$KIMI_CODE_HOME/credentials/kimi-code.json`, otherwise `$HOME/.kimi-code/credentials/kimi-code.json` | Kimi CLI OAuth |
| Cursor | `$XDG_CONFIG_HOME/cursor/auth.json`, otherwise `$HOME/.config/cursor/auth.json` | Cursor CLI OAuth |
| Gemini | `$HOME/.gemini/oauth_creds.json` | Gemini CLI OAuth |

`USERPROFILE` supplies the resolved home when `HOME` is absent. Relative homes
fail closed. There is no ambient home lookup, browser import, token refresh,
credential rewrite, or device-ID creation. Cursor derives its HTTP session cookie
from the CLI JWT; it does not import a browser cookie. Known account IDs are read
locally, never fetched by `Identity`. Collection rechecks the identity before
sending requests; the service must also re-probe before publication.

Amounts preserve decimal precision. Cursor cents become USD; OpenRouter account
credits are separate from its key spending cap. Vercel lifetime spend is an
uncapped `SpendCap` (`total_used`) because the normalized model has no standalone
spend field. A missing limit has no percentage and does not mean unlimited.
Kimi ratio pools never become request counts. Gemini IDs are `model:<slug>` with
no invented duration, following UT-078. Percentages above 100 remain visible.

Collectors return typed errors. When Gemini plan discovery succeeds but quota
returns 403, collection returns `provider_unavailable` and a partial snapshot
containing the plan; the service should preserve that account metadata without
marking quota data fresh. Last-good/stale retention remains service-owned.

The payload mappings follow the Provider Usage spec Integration Points,
`analysis/03_analysis_providers-compozy-set.md`, and the vendored provider cards.
`collectors_test.go` owns UT-019, UT-077, UT-078 and collector-level credential
fencing, precision, 401/429/5xx, parse-error, and body-cap cases. Tests use local
HTTP servers and temporary credential stores; no real account is contacted.

Cross-surface impact: these collectors add data to the existing normalized
provider-usage contract, with no new CLI/tool/hook/config surface or persistent
state. The integration owner must register them in the daemon and consolidate
provider rows, official-skill documentation, QA scenario steps, and the shared
change-impact audit with the other Provider Usage slices.
