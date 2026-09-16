# Claude Fable 5.1 reasoning — issue 655

Status: implementation and isolated application QA in progress. Not delivery evidence yet.

## Root cause and prior coverage

The installed official Claude ACP adapter 0.78.0 with Claude Code 2.1.266 advertises
`claude-fable-5-1[1m]`. Selecting it returns `effort`, category `thought_level`, with provider default,
low, medium, high, xhigh and max. Selecting Haiku removes effort. Product reasoning support was
therefore not the missing capability.

The first identity loss occurred in Claude catalog mapping: partial version-token matching and a
family fallback could map an exact new version to an older curated identity. Separately, live ACP
discovery discarded every config option except model and never acquired each model's effort profile.
The models.dev-only exact Fable 5.1 row consequently had reasoning support but no adjustable levels.
The Web mapper passed that absence through; the selector incorrectly described it as provider-managed.
The active composer also ignored session config-option updates, and successful ACP writes were not
checked against returned current values before being reported as applied.

PR #579 covered Codex, Cursor and OpenCode with real provider evidence, not the installed Claude
Fable 5.1 discovery/binding journey. Its open effort identifiers and existing regressions are retained.

## Current evidence

- Direct installed-adapter walkthrough: Fable 5.1 selected by exact transport ID; low then high effort
  acknowledged in returned config options; real prompt completed with `end_turn`. Provider usage
  identified `claude-fable-5-1`. This is transport evidence, not yet an application walkthrough.
- Focused ACP/modelcatalog race tests pass, including per-model discovery, models without effort,
  and explicit failure when a provider does not confirm requested configuration.
- Focused Web suites pass: 118 tests across catalog mapping, rendered selector and prompt-runtime
  hydration. New coverage distinguishes unknown capability and applies thought-level notifications
  only to the effective model without overwriting pending prompt intent.
- Existing account-scoped ACP subprocess/SQLite integration passed; expanded persistence coverage
  is being verified.
- Heavy gates and complete builds/typechecks/lint/tests belong to GitHub CI by explicit instruction.

## Remaining acceptance evidence

Rendered application selection, favorites/recent continuity, catalog warm/reload behavior, live
prompt acknowledgement through Compozy, model switching and failure/recovery walkthroughs remain
pending. Current-head CI and CodeRabbit, Greptile and React Doctor reviews have not run yet.

## Sources

- [Claude model configuration](https://code.claude.com/docs/en/model-config)
- [ACP session config options](https://agentclientprotocol.com/protocol/v1/session-config-options)
- Installed official `@agentclientprotocol/claude-agent-acp` 0.78.0 model/effort implementation.
