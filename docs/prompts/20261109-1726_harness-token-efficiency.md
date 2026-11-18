---
type: improvement
created: 2026-11-09 17:26
source: compozy/compozy · branch harness-token-efficiency (worktree) · no issue yet
---
# Cut the token and context cost CompozyOS adds to Claude Code sessions, and fix the inflated cost ledger

## Problem

A measurement of four weeks of real usage (2026-10-12 → 2026-11-09, 166k Claude API calls, 96% of them
inside CompozyOS-managed sessions) shows that CompozyOS-managed Claude sessions spend most of their
tokens re-reading a very large context, and that a measurable part of that context is put there by the
harness itself, repeatedly, without need. Separately, the cost CompozyOS records for Claude sessions is
wrong by two orders of magnitude.

The outcome wanted: CompozyOS stops injecting redundant bytes into provider prompts, gives the operator
a way to bound context growth, avoids avoidable full-context cache rewrites, and records correct cost.
Every item below is in scope. They are independent workstreams (W1–W10), listed in priority order.

## Evidence

Method: every Claude Code transcript modified since 2026-10-19 was parsed; each API call's real usage
(`input`, `cache_read`, `cache_creation`, `output`) was attributed to what occupied the context at that
call. Attribution by origin is estimated from character counts and scaled to the real token count of
each call (about ±15%). Cost-equivalent shares use the standard multipliers (cache read 0.1×, cache
write 1.25×/2×, output 5×).

Totals for CompozyOS-managed Claude sessions (1,715 main sessions + 909 native subagent transcripts):

| Metric | CompozyOS-managed | Plain Claude Code (same operator) |
| --- | --- | --- |
| API calls | 157.7k | 8.4k |
| Cache-read tokens | 44.9 B | 1.76 B |
| Cache-write tokens | 650 M | 35 M |
| Output tokens | 115 M | 5.8 M |
| Average context per call | 289k | 213k |
| Average context at first call | 72.8k | 52.9k |

Cost-equivalent share by origin (CompozyOS-managed sessions):

| Origin | Share |
| --- | --- |
| Tool results (Bash, Read, other) | 29.0% |
| Model output kept in context | 22.5% |
| Operator config (CLAUDE.md, MEMORY.md, skill listing, MCP servers, hooks) | 13.3% |
| **Context injected by CompozyOS (startup + per turn)** | **10.2%** |
| Output generation | 9.2% |
| Claude Code fixed (system prompt, built-in tools, reminders) | 5.9% |
| Cache rebuilds (miss → full context rewritten) | 4.3% |
| **CompozyOS native tool results** | **3.1%** |
| Human prompts, images, files, compact summaries | 1.8% |
| **Wake / inter-session prompt text** | **0.7%** |

Breakdown of the CompozyOS-injected 10.2%:

| Item | Share of total spend | Avg tokens at first call |
| --- | --- | --- |
| `<compozy-situation-context>` blocks | 3.49% | 3,187 |
| Startup "Session instructions" plain text (agent prompt + `compozy` router skill body) | 2.10% | 7,087 |
| `<available-skills>` / `<current-available-skills>` catalog | 2.01% | 4,734 |
| Hosted tool names listed by the provider's deferred-tool listing | 1.50% | 3,485 |
| Hosted tool schemas loaded eagerly | 0.80% | 1,356 |
| `<compozy-runtime-context>` | 0.22% | 845 |

Other measured facts the workstreams rely on:

- Sessions run on a 1,000,000-token window (`context_size = 1000000` in the per-session usage ledger)
  and almost never compact: 37 compactions across 1,715 sessions. 269 sessions that exceeded 400k of
  context account for 78% of main-session cache reads. If no call had exceeded 200k of context, the
  processed token volume would have been 40.0% lower; with a 400k bound, 13.7% lower (upper bounds —
  compaction has its own cost).
- Daemon-initiated turns (monitor wakes, inter-session messages, session wakes, subagent wakes, task
  notifications): 6,706 turns, 46.2% of main-session cache reads, 2.63 M cache-read tokens per turn on
  average, because they land on sessions averaging ~420k of context. 1,696 of those turns were trivial
  (≤2 API calls and <400 output tokens) and still cost ~790 M cache-read tokens.
- Prompt cache TTL is one hour. A call after an idle gap of 65 minutes or more missed the cache 100% of
  the time (190 occurrences, 78 M tokens rewritten, ~425k tokens each). Gaps under 58 minutes hit the
  cache ≥93% of the time, including across the 30-minute idle runtime release, so process restart by
  itself does not break the cache.
- Situation context: 3,584 blocks in 2,045 first prompts (1.75 per prompt, ~4.7k chars each);
  1,559 blocks in 2,481 human prompts (~8.6k chars each); 1,777 blocks in 2,177 inter-session prompts
  (~8.0k chars each). In a later-turn block of 10,226 chars, `session.lineage.permission_policy` is
  9,598 chars (`tools` 8,326, `skills` 1,142) while every other section is already replaced by a
  ~90-char `{"unchanged":true,...,"reuse":"previous_full_section_same_acp_session"}` marker.
- One measured first prompt (48,443 chars): `<compozy-runtime-context>` 2,496; first
  `<compozy-situation-context>` 2,679; `<available-skills>` 15,427; plain text after it 13,592
  (starts with "Resolve canonical `compozy__skill_view` through the active harness" and contains the
  `compozy` router skill body); second `<compozy-situation-context>` 12,961; `<current-available-skills>`
  216; trailing boilerplate 739.
- Hosted tool names: the provider lists ~330 deferred names of the form
  `mcp__compozy-hosted-tools__compozy__<tool>`; the prefix alone is 36 characters per name.
- Average native tool result size (chars): `session_history` 7,549; `session_status` 5,012;
  `session_archive` 4,703; `session_spawn` 4,255; `tool_info` 4,101; `subagent_status` 3,168;
  `session_create` 2,863; `agent_list` 16,204; `subagent_capabilities` 9,973. `skill_view` (7,465) is
  expected content and is not a target.
- One-shot daemon sessions pay a full cold context: `system` sessions on `claude-opus-5-5` write ~41k
  cache tokens per single-call session (186 sessions); heartbeat `triage` sessions on Haiku write
  65–78k per session.
- Cost ledger: for one orchestrator session, the per-turn ledger shows `cost_amount` rising
  monotonically (1.34, 1.51, 2.46, 2.64, 3.09 …, max 300.9) while token columns are per-turn deltas.
  `token_stats.total_cost` for that session is 59,163.13 — the sum of 420 cumulative values — against
  a real cost of about 300.9. Across the profile, `token_stats` reports USD 195,298 for Claude in one
  week; Claude Code's own cost records sum to roughly USD 14k for four weeks. Token columns are correct
  (921 M cache-read in `token_stats` vs 942 M in the transcript for the same session).

## Current State

**Situation context rendering.** `internal/situation/render.go` renders sections `self`, `workspace`,
`session`, `soul`, `task`, `capabilities`, `limits`, `provenance`. `internal/situation/prompt_compaction.go`
hashes each rendered section and replaces an unchanged one with a reuse marker:

```go
func promptSectionSignatures(sections []renderedSection) map[string][sha256.Size]byte {
	signatures := make(map[string][sha256.Size]byte, len(sections))
	for _, section := range sections {
		signatures[section.name] = sha256.Sum256(section.raw)
	}
	return signatures
}
```

The `session` section is built in `internal/situation/service_payload.go`:

```go
return contract.AgentSessionPayload{
	ID:    strings.TrimSpace(info.ID),
	Name:  strings.TrimSpace(info.Name),
	Type:  info.Type,
	State: info.State,

	Lineage:   contract.SessionLineagePayloadFromStore(info.Lineage),
	CreatedAt: info.CreatedAt.UTC(),
	UpdatedAt: info.UpdatedAt.UTC(),
}
```

`UpdatedAt` (and `State`, `Name`) change between turns, so the section hash never matches and the full
`lineage.permission_policy` is re-rendered on every prompt that carries a situation block. The startup
block comes from `startupSessionPayload` in `internal/situation/service_session_payload.go` (no
lineage), and the first turn prompt then renders a second, full block in the same message.

**Skills catalog.** `internal/skills/catalog.go` renders `<available-skills>` at startup and
`<current-available-skills>` per turn (already collapsed to an `unchanged="true"` marker when
nothing changed, plus a fixed boilerplate paragraph). With `home_policy = "operator"`, Claude Code
also injects its own skill listing for the same skills, and the names appear a third time in
`permission_policy.skills`.

**Startup prompt.** `internal/acp/agent_process_prompt.go` builds the
"Session instructions (treat as system guidance for this conversation):" message, which carries the
runtime context, the startup situation block, the agent prompt, the skills catalog and the `compozy`
router skill body.

**Hosted tools.** `internal/toolmeta/registry.go:10` — `const HostedServerName = "compozy-hosted-tools"`.
The provider-visible name is `mcp__<server>__<canonical id>`, and canonical IDs already start with
`compozy__`.

**Claude session options.** `internal/acp/claude_session_meta.go` — `claudeCodeSessionMeta` already
passes `env`, `settings` and `disallowedTools` under `claudeCode.options`; `claudeCodeInputOnlySessionMeta`
launches the title pass without the operator's Claude Code environment (comment: "cuts ... the prompt
from about 45k to under 1k tokens"). `skills/compozy/references/configuration.md` states
"CompozyOS has no compaction settings: the agent owns its context window", and the retired
`session.compaction` settings are archived on load.

**Idle release and wakes.** `internal/session/manager_idle_release.go` and
`internal/session/work_supervision.go` release an idle runtime after `session.supervision.idle_release_after`
(30m here). Wake prompts are produced in `internal/monitor/message.go`,
`internal/tools/builtin/session_monitor.go` and `internal/daemon/session_wake_bridge.go`.

**Cost ledger.** `internal/observe/cost_estimation.go` passes the provider-reported `usage.CostAmount`
through as an `actual` / `agent_reported` cost. `internal/store/globaldb/queries/observe.sql` then adds
it to the running total:

```sql
    WHEN excluded.total_cost IS NULL THEN token_stats.total_cost
    WHEN token_stats.total_cost IS NULL THEN excluded.total_cost
    ELSE token_stats.total_cost + excluded.total_cost
```

`internal/store/globaldb/queries/observe_overview.sql` does the same for `token_usage_daily`. The
Claude ACP adapter reports cost as a session-cumulative amount, so every turn adds the whole running
cost again.

## Requirements

Deliver each workstream as its own commit so it can be reviewed and reverted alone. If a workstream
cannot be completed safely within these constraints, stop that workstream, leave the tree clean for it,
and report exactly what blocked it — do not ship a partial or workaround version.

**W1 — Operator-controlled auto-compact window for Claude sessions.**
- A config surface lets the operator bound how full a Claude session's context gets before the
  provider compacts. Claude Code documents `CLAUDE_CODE_AUTO_COMPACT_WINDOW` (100000–1000000) and the
  `autoCompactWindow` setting for this.
- It must be settable at least globally and per agent definition, so long-lived orchestrator agents can
  use a different bound from workers. Follow the existing config layering and validation conventions.
- Default behavior does not change: with the setting absent, CompozyOS passes nothing and the provider
  keeps its own window.
- Out-of-range or malformed values are rejected at config validation with the existing error style.
- The value reaches the provider on `session/new`, `session/load` and `session/resume`.
- Config reference, settings payloads and the `compozy` skill reference are updated; the sentence
  claiming CompozyOS has no compaction settings is corrected. The retired `session.compaction` keys keep
  their existing archive-on-load behavior and must not be confused with the new key.

**W2 — Stop re-sending the static part of the situation context.**
- Within one ACP session, a turn prompt must not re-render `lineage.permission_policy` (or any other
  lineage content) when it has not changed since the last full delivery. Volatile fields such as
  `updated_at`, `state` and `name` must not defeat reuse of static content.
- The full content is sent again whenever the existing reuse rules say it must be: new ACP session,
  provider compaction since the last prompt, observed context drop, or a real change in the content.
- The first message of a session must not carry the same situation content twice.
- The `/agent/context` HTTP/UDS payload and the `AgentContextPayload` contract are unchanged; this is a
  prompt-rendering change only.
- Success: for a session whose policy lists ~330 tools, the situation block of a second and later
  human or inter-session prompt is under 1,000 characters when nothing changed.

**W3 — Remove skill-catalog duplication.**
- The startup skills catalog stops spending bytes on what the provider already receives. Every skill
  the session is allowed to use stays discoverable and loadable through `compozy__skill_view`, and the
  per-turn "current catalog" semantics stay correct when a skill is added, removed or changed mid-session.
- Providers that do not list skills natively keep a usable catalog.
- Success: the startup catalog for a ~100-skill workspace on Claude with `home_policy = "operator"`
  shrinks by at least half in characters, measured before and after.

**W4 — Shrink the startup instructions text.**
- Reduce the fixed plain text every session receives at startup (the router skill body and its
  surrounding boilerplate), without losing the rules a session cannot work without: the wait/monitor
  rule, how to load references, and the instruction not to bypass withheld capabilities.
- Sessions must still be able to reach every reference the router lists.
- Success: measured reduction in characters for a `general` session first prompt, reported before and
  after; target at least 40% for the plain-text portion.

**W5 — Shrink the hosted tool name and schema footprint.**
- Reduce the bytes a provider spends listing hosted tools. Options are yours: a shorter provider-visible
  server name, advertising only the tools the session's policy and agent definition actually allow,
  trimming eagerly loaded schemas, or a combination.
- Canonical `compozy__*` tool IDs are a public surface and do not change.
- If the provider-visible name changes, every place that matches on the old name (permission
  allowlists, tool approvals, hooks, transcript projection, tool routing, docs, tests) keeps working for
  existing users under the compatibility policy.

**W6 — Compact native tool results.**
- Mutation receipts and status reads returned to agents stop carrying large static payloads the caller
  did not ask for (for example a full session object with policy lists on `session_create`,
  `session_archive`, `session_spawn`), and list/status tools default to bounded output.
- Callers can still obtain the full detail explicitly.
- Tool output shape is a public surface: apply the compatibility policy, and record in the report which
  fields moved or became opt-in.
- Success: average result size for `session_create`, `session_archive`, `session_spawn` and
  `session_status` drops by at least half on a representative session, measured before and after.

**W7 — Reduce no-op daemon-initiated turns.**
- Find which wake kinds produce turns in which the session does nothing useful, and remove the ones the
  daemon can prove are redundant — for example several wakes for one session delivered as separate
  turns when they could be delivered together, or a wake repeating a state the session was already told.
- No wake that carries new information may be dropped or delayed past its current guarantees. Monitor,
  reply-watch and session-wake delivery contracts stay intact.
- Report what was measured, what was removed, and what was left alone and why.

**W8 — Avoid full-context cache rewrites on long idle resumes.**
- Give the operator an opt-in way to have a large-context session compacted before its prompt cache
  expires (the measured TTL is one hour), so a resume hours later does not rewrite ~400k tokens.
- Off by default. It must not compact a session that is mid-turn, awaiting an answer, or below a
  configurable context size, and it must be observable in the session's event stream.
- The existing `compozy__session_compact` path is the compaction owner; do not add a second one.

**W9 — Lean provider environment for daemon-owned and worker sessions.**
- One-shot daemon sessions (`system`, heartbeat `triage`) stop loading the operator's full Claude Code
  environment when their role does not need it, in the same spirit as the existing input-only title
  pass.
- Agent definitions get an opt-in way to run with a reduced Claude Code environment (for example
  without the operator's extra MCP servers, claude.ai connectors or auto-memory). Default for user
  sessions is unchanged. Repository instructions (`CLAUDE.md`) stay available to coding workers unless
  the definition says otherwise.
- Success: measured first-call context for a triage/system one-shot session, before and after.

**W10 — Correct cost accounting.**
- A turn's recorded cost is the cost of that turn. When a provider reports a session-cumulative amount,
  CompozyOS stores the increment, and handles the cumulative counter restarting (new provider process,
  resumed session) without producing a negative or doubled value.
- Providers that report a per-turn amount keep working. Establish which semantics each adapter uses from
  the adapter or its documentation, not by assumption.
- `token_stats.total_cost` and `token_usage_daily.total_cost` for existing data: inflated totals must not
  keep being presented as `actual`. Either recompute them from the per-session usage ledgers where that
  is possible, or mark them so every reader shows them as unknown. This is user state: ship the
  migration, keep it lossless for every other column, and state the choice in the report.
- Regression coverage pins the invariant "sum of recorded turn costs equals the provider's final
  cumulative cost" for a multi-turn session, including one restart.

## Constraints & Non-goals

- Follow the repository `CLAUDE.md`, `internal/CLAUDE.md` and the compatibility policy (SD-013): user
  state upgrades losslessly, public surfaces keep the old shape for one release with a warning when they
  cannot be migrated losslessly, internal code is renamed without aliases.
- Load and follow the owning skills where they apply: `eng-code-guidelines`, `eng-test-conventions`,
  `eng-consolidate-test-suites`, `eng-schema-migration` (W10), `eng-contract-codegen-coship` (any wire
  change), `eng-data-boundaries` (W6), `use-modern-go`.
- Do not run `make gate`, `make gate-full`, repository-wide lint, full or `-race` test suites, or E2E.
  Run focused tests and focused lint for the packages you changed, with zero warnings or errors.
  Generated files change only through their owning generator.
- Production source files stay cohesive and at most 500 lines.
- Tests: name the invariant, owning layer and canonical suite before adding one; extend an existing
  suite; do not assert on prose, snapshots or implementation details unless that artifact is the
  contract. A test that exposes a regression is fixed in production code, never weakened.
- Work only in this worktree. Commit on branch `harness-token-efficiency` with
  `<type>: <description>` messages (no scopes), one commit per workstream. Do not push, open a pull
  request, or edit `docs/_memory/change-impact.md`; put impact notes in your final report.
- No destructive Git (`restore`, `checkout`, `reset`, `clean`, `rm`, `stash`).
- Non-goals: changing how the model reads files or how much it writes; changing Codex or other
  providers' prompts beyond what a shared code path requires (but do not regress them); adding cache
  columns to `token_usage_daily`; any Web UI redesign; changing default compaction behavior.

## Verification

- W1: config load/merge/validation suites for the new key; the ACP session-meta suite proves the value
  is present in the provider options for new, loaded and resumed sessions and absent when unset.
- W2: the `internal/situation` suite proves reuse across turns with changing `updated_at`/`state`,
  full re-delivery after compaction or a new ACP session, and a single block in the first message.
- W3, W4: the suites owning `internal/skills/catalog.go` and the startup prompt builder; assert on
  behavior (discoverability, presence of the required rules), not on full prose.
- W5: tool registry/routing suites, including the compatibility path for the old provider-visible name
  if it changes.
- W6: the native tool suites for each changed tool, with status-plus-body assertions.
- W7, W8: the monitor / session-wake / idle-release suites; prove no informative wake is lost.
- W9: the ACP session-meta suite for the lean option and the one-shot roles.
- W10: the observe and `globaldb` suites, plus the migration's fresh/reopen/equivalence coverage.
- Real run: build the binary and drive at least one multi-turn session through an isolated lab with the
  provider-free ACP mock (`internal/testutil/acpmock`, skill `eng-qa-bootstrap` /
  `eng-worktree-isolation`), capturing the first prompt and a later turn prompt. Report character
  counts before (base commit) and after for: whole first prompt, situation block on turn 2+, skills
  catalog, startup plain text, and the result size of the W6 tools.
- Update the affected `docs/qa/scenarios/` files for changed user-visible behavior, and list the
  scenarios you touched.

## Final report

Reply with, per workstream: status (done / blocked / not started), commit SHA, files changed, the
focused test and lint commands you ran with their results, before/after measurements, compatibility
regime and migration notes, and any open question. State plainly anything you did not verify.

## References

- `internal/situation/render.go`, `prompt_compaction.go`, `service_payload.go`,
  `service_session_payload.go`, `service_context.go`
- `internal/skills/catalog.go`
- `internal/acp/agent_process_prompt.go`, `internal/acp/claude_session_meta.go`
- `internal/toolmeta/registry.go:10`
- `internal/session/manager_idle_release.go`, `internal/session/work_supervision.go`
- `internal/monitor/message.go`, `internal/tools/builtin/session_monitor.go`,
  `internal/daemon/session_wake_bridge.go`
- `internal/observe/cost_estimation.go`, `internal/store/globaldb/queries/observe.sql`,
  `internal/store/globaldb/queries/observe_overview.sql`
- `internal/cli/session_usage_context.go` (existing "CompozyOS Sent ≈ N tokens" estimate)
- `skills/compozy/references/configuration.md`, `skills/compozy/references/native-tools.md`
- `docs/_memory/standing_directives.md` §SD-013
- Claude Code docs: `CLAUDE_CODE_AUTO_COMPACT_WINDOW`, `autoCompactWindow`
  (https://code.claude.com/docs/en/model-config, https://code.claude.com/docs/en/env-vars)
