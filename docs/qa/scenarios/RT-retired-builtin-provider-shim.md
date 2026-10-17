---
id: RT-retired-builtin-provider-shim
area: RT
title: Keep a retired built-in provider reference working for one release with a replacement warning
persona: Ada
journey:
expected: "`compozy provider list` on an isolated lab daemon lists only the kept built-ins (claude, codex, gemini, opencode, kimi-cli, qwen-code, copilot, cursor, pi, openrouter, zai, moonshot, vercel-ai-gateway, xai) plus declared custom providers; a config with `defaults.provider = \"hermes\"` still resolves with the former built-in definition and the daemon log records one `provider.retired_builtin` warning for `hermes` with `removed_in=v0.5.0` and a `replacement` field holding the exact `[providers.hermes]` entry; after declaring `[providers.hermes]` with its own `command`, the provider resolves as a plain custom provider and no new warning is logged."
entry_points: compozy provider list -o json; compozy provider inspect hermes -o json; compozy config set defaults.provider hermes; $COMPOZY_HOME/config.toml; $COMPOZY_HOME/logs/compozy.log; GET /api/providers over HTTP and UDS; https://compozy.com/docs/agents/providers#retired-built-in-providers
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-retired-builtin-providers-20261017-211440-188126-lab/qa-artifacts/qa/retired-provider-evidence.md
last_report:
overlaps: MS-028; RT-081
---

Added for issue #761 (removal of 12 long-tail built-in providers). Bootstrap an isolated lab with
`eng-qa-bootstrap`; the walk needs no real Hermes executable because it observes config resolution
and the daemon log, not a Hermes session.

1. Start the lab daemon with no `[providers.*]` entries. `compozy provider list -o json` returns only
   the kept built-ins. None of `hermes`, `openclaw`, `blackbox`, `cline`, `goose`, `junie`, `kiro`,
   `openhands`, `qoder`, `mistral`, `groq`, or `minimax` (or their aliases) appears.
2. Set `defaults.provider = "hermes"`, restart the daemon, and resolve the default (provider inspect
   or a session create that stops at provider launch). Resolution succeeds with
   `command = "hermes acp"`. `compozy.log` holds exactly one `provider.retired_builtin` line for
   `provider=hermes` with `removed_in=v0.5.0` and a `replacement` holding the `[providers.hermes]`
   TOML entry. Resolving it again in the same daemon run logs no second warning.
3. Replace any `[providers.hermes]` tables in `config.toml` with the logged `replacement` and restart. Resolution
   still succeeds with the declared command, and the daemon log records no new
   `provider.retired_builtin` warning.
4. Repeat step 2 with an alias (`hermes-agent`) and with a pi-backed ID (`mistral`) to confirm the
   alias canonicalizes to `hermes` and the pi-backed warning's replacement carries
   `harness = "pi_acp"`, `runtime_provider = "mistral"`, and the `MISTRAL_API_KEY` credential slot.

2026-10-17 targeted lab walk: PASS for steps 1-3 and the pi-backed half of step 4. Base `main`
listed 26 built-ins; the branch listed 14. With `defaults.provider = "hermes"` and a partial
`[providers.mistral]` override the daemon started, the CLI and daemon each logged one
`provider.retired_builtin` line per ID with `removed_in=v0.5.0` and the full replacement, and
`provider inspect mistral` matched the pre-change bound-secret shape. Declaring `[providers.hermes]`
listed Hermes as a custom provider and silenced its warning. Alias canonicalization (`hermes-agent`)
is covered by `internal/config` unit tests, not by this walk.
