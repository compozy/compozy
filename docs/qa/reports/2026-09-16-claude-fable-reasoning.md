# Claude Fable 5.1 reasoning — issue 655

Status: blocked verification. Implementation is in open PR #658; successful application prompt/recovery remains unverified because native Claude rejects the real attempts. This report does not claim delivery.

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
lab-daemon restart are verified. Live prompt acknowledgement through Compozy, queued-prompt execution, and successful provider
recovery remain pending. Model selection changes are verified below. The first PR head completed Greptile review without findings. CodeRabbit identified three capability
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
The application prompt and recovery walkthrough remain blocked after the production corrections.

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

## Follow-up walkthrough versions

The Web was served from code head fac7fe19d. The isolated daemon binary includes the production
changes through 5511164; later empty-snapshot and absent-policy corrections have focused race and
persistence coverage. The known, persisted Fable profile used in this walk is unchanged by those
corrections. This is retained evidence for that unchanged path, not a final-binary application pass.
The remaining successful prompt/recovery walk must use the final daemon binary.

| Journey | Observed result | Verdict |
| --- | --- | --- |
| Exact full-list and existing favorite selection | Fable 5.1 remains distinct from Fable 5; five advertised stops are rendered. | Pass |
| Valid effort on model switch | Fable Low to Sonnet retains Low. | Pass |
| Invalid effort on model switch | Haiku removes the effort control and selected effort; returning to the Fable favorite selects its advertised High default. | Pass |
| Rapid input and reload | From Low, End/Home/ArrowRight accepts Max and temporarily disables later input during persistence; public readback and reload retain Max. | Pass |
| Narrow viewport | At 390×844 the 320-pixel picker fits, without horizontal page overflow; slider remains visible. | Pass |
| Failed refresh | Prior successful rows remain available as stale with an explicit warning and original success timestamp. | Pass |
| Applied effort and successful next prompt | The application remains unbound and preserves selected intent after native provider errors. | Blocked verification |
| Real queued prompt and successful recovery | Cannot establish successful Fable turns while the provider rejects the account. | Blocked verification |

[Narrow viewport and switch evidence](https://github.com/compozy/compozy/pull/658#issuecomment-5703095524)
uses an inspected screenshot without host paths or session identifiers.

A minimal control using the same installed native Claude and official ACP adapter, with no
CompozyOS system context or MCP servers, selected exact Fable 5.1 and acknowledged Low. Its prompt
then failed with `errorKind=rate_limit` and the account's Fable-limit message. This independently
establishes the usage-capacity blocker. A later application retry additionally returned a native
authentication-resolution error during model validation; it also preserved selected Max without
marking it applied. No login, credit purchase, account change, host restart, or credential repair was
performed. The remaining walk must resume with working native authentication and available Fable
capacity; a different model or mocked answer cannot satisfy it.

React Doctor completed code head fac7fe19d with no new issues. Greptile completed that head and
correctly retains finding 4029705246 for incomplete real QA. CodeRabbit withdrew the proposed
unconditional hydration early return after checking the complete-snapshot protocol. Other review
coverage and final CI remain tracked in the PR. A Darwin CI dependency-download timeout requires a
same-head retry; no check has been disabled. Heavy gates remain CI-only.
