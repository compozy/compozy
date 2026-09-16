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
A real application prompt then exposed a third boundary: the session launch seeded
`ANTHROPIC_MODEL` with the logical ID but ACP selected the catalog transport ID with `[1m]`.
The native override changed the adapter's advertisement, so negotiation rejected that transport ID.
Launch now seeds the same validated transport identity while public/persisted IDs remain logical.

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
- Focused Web suites pass: 119 tests across catalog mapping, rendered selector and prompt-runtime
  hydration. New coverage distinguishes unknown capability and applies thought-level notifications
  only to the effective model without overwriting pending prompt intent.
- Existing account-scoped ACP subprocess/SQLite integration passed; expanded persistence coverage
  is being verified.
- Heavy gates and complete builds/typechecks/lint/tests belong to GitHub CI by explicit instruction.

## Remaining acceptance evidence

Rendered full-list selection, favorite/recent selection, and Low persistence through browser and
lab-daemon restart are verified. Live prompt acknowledgement through Compozy, model switching
and successful provider recovery remain pending. The first PR head completed Greptile review without findings. CodeRabbit identified three capability
boundary findings and a documentation warning; React Doctor identified hook complexity. These
are being remediated, with current-head review and CI still required. Initial CI also identified Go
format/lint, product-language wording, and a terminal golden-path E2E failure; none is claimed green.

## Sources

- [Claude model configuration](https://code.claude.com/docs/en/model-config)
- [ACP session config options](https://agentclientprotocol.com/protocol/v1/session-config-options)
- Installed official `@agentclientprotocol/claude-agent-acp` 0.78.0 model/effort implementation.

## Review and failure-path follow-up

Each transport now retains its inspected options through persistence. Catalog presentation and
Claude launch share the same transport selection, so a sibling binding cannot supply its effort
profile. Explicit negotiation policy and confirmed-capability fields cross the public boundary.
Logical session resume now defers validation to the next concrete prompt bind, matching creation.

The real application selected Fable 5.1 with its five advertised stops and persisted Low across a
lab-daemon restart. Retrying discovery hit the installed account's HTTP 429 rate limit. The public
source status retained its previous five rows and last-success timestamp, marked them stale, and
reported the upstream error. No cache/database edits or provider-default substitution were used.
The application prompt and recovery walkthrough are still pending after the production corrections.

## Rendered application and remaining blocker

[Inspected screenshot and caption](https://github.com/compozy/compozy/pull/658#issuecomment-5702791815)
show the real selector with Fable 5.1 selected and favorited, five advertised stops, Low selected,
and a visible stale-source warning. Reload preserves the favorite and selected Low. The distinct
Fable 5 row remains separate. No operator session was used.

The next application prompt reaches Claude with coherent model identity but returns account HTTP
429. Public readback preserves selected Low and leaves the runtime unbound; it does not claim
provider acknowledgement. Failed refresh preserves the prior five live rows and last-success time.
The direct-adapter successful prompt does not close this application-level acceptance gap.

The second review round identified empty-snapshot preservation and absent-policy semantics,
with canonical source, merge, and persistence coverage. A separate review suggestion to validate
configuration-matrix coordinates against post-selection live controls was not adopted: the released
extension contract declares those coordinates against the logical row. Coverage preserves that
distinction and the existing SQLite foreign keys. ACP complete snapshots intentionally
withdraw effort levels when the thought-level option disappears. This preserves the tested
config-option-update contract rather than retaining stale selectable levels.

CI exposed a migration-test context reused after an expensive upgrade; close/read now use fresh
bounded contexts, matching neighboring migration suites. All preservation assertions remain.
Heavy checks continue in GitHub CI. Final-head CI and all reviewer dispositions are still pending.
