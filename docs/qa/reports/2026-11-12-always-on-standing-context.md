# Always-on standing context: operator rules, peer roster, provider MCP isolation

Scenario: `docs/qa/scenarios/RT-always-on-standing-context.md`. Scope: two isolated labs built by
`eng-qa-bootstrap` (targeted profile, surfaces cli, runtime and provider), each with its own
`COMPOZY_HOME`, ports and sockets. Real Claude (`claude-haiku-4-5-20251001`) and Codex
(`gpt-5.6-luna`) sessions ran on the operator's native provider login. No operator daemon or
`~/.compozy` state was read or written. Both teardowns report `"clean": true`.

Delivered prompt text was read from the lab sessions' provider transcripts; span sizes come from
`compozy session usage <id> --turns -o json`.

## Walk 1 — binary `v0.6.0-14-g7cc92be86`

| Step | Result | Evidence |
| --- | --- | --- |
| First turn carries the full block | pass | `1. File every GitHub issue through the writer peer…` and ``- writer (lab): … Accepts: `intake: <text>` \| `sweep:` ``; the `compozy-agent-memory` section holds the note only |
| Second turn carries the marker | pass | `<compozy-standing-context unchanged="true">…`, `standing` span 160 B, unchanged |
| Rule added while the session runs | pass | next turn carries the full block with rule 2 |
| Provider compaction | pass | `compozy session compact` completed, context 53,302 → 9,241; next turn full block (1,024 B), following turn marker (160 B) |
| Agent tools cannot change a rule | fail | replace and remove were refused and the file was unchanged, but the agent saw `schema_invalid: invalid tool request` instead of the reason |
| Add and search | pass | add wrote a dated note; search returned the rule with its `- [rule] ` prefix |
| Peers and pause | pass | `agent instance list --peers` returns `accepts`; after pause the roster shows `- writer (lab) [paused]` |
| On-demand agent | pass | no `standing` span |
| Session without an instance | pass | rules only (510 B), no `# Peer agents` |
| Memory file from before rules | pass | no `# Operator rules` section, three lines stay notes, digest unchanged |
| The rule binds | pass | asked where issues are filed, the agent answered "Through the writer peer with `intake: <text>`." |

The failed step was repaired in `3dec1b996` (the refusal is a denial that carries its reason). A
roster cache and its invalidation on instance changes landed in `6c57da4b7` and `7a5ba52e9`.

## Walk 2 — binary `v0.6.0-18-g7a5ba52e9`

| Step | Result | Evidence |
| --- | --- | --- |
| Full block, then marker | pass | 827 B full on turn 1, 160 B marker on turn 2 |
| Agent tools cannot change a rule | pass | raw tool result for replace and remove: `policy_denied: agent_memory_rule_protected: operator rules are changed only by the operator (compozy agent memory add\|replace\|remove, or the agent's Memory editor)`; rule line and digest unchanged |
| Roster after pause and resume | pass | turn 0.56 s after pause shows `[paused]`; turn 0.19 s after resume shows it active |
| Roster after a new instance | pass | turn 0.38 s after create lists the new peer |
| `accepts` in the peers read | pass | `{"name":"writer",…,"accepts":["intake: <text>","sweep:"]}` with `-o json` |

On the first attempt of the rule-edit step the agent declined to call the tools at all, quoting the
standing rule text; a second prompt framed as an operator-run refusal test was needed.

## Prompt size, first turn "Reply with the single word: ok"

| Session | Context used (tokens) | CompozyOS-delivered (est. tokens) | Standing span | MCP tool names from outside CompozyOS |
| --- | ---: | ---: | ---: | ---: |
| Claude always-on, `lean_environment: false` (previous behavior) | 48,266 | 7,383 | 965 B | 331 across 6 servers |
| Claude always-on, default | 41,601 | 7,381 | 973 B | 0 |
| Codex always-on | 36,120 | 7,378 | 967 B | not narrowed |
| Codex on-demand (reference) | 35,652 | 6,834 | none | not narrowed |

The Claude always-on agent drops 6,665 tokens of first-turn context (13.8%), all of it tool names
from the operator's own Claude setup: `rea` 229, `cua-driver` 62, `inspo` 15, `DaVinci_Resolve` 14,
`claude_ai_Claude_Docs` 8, `headroom` 3. Both sessions list the same 285 `compozy-hosted-tools`
names, which Claude Code already defers. The standing block costs about 240 tokens in full and 40 as
the marker. On Codex the always-on session costs about 470 tokens more than the on-demand reference
(doctrine, memory and standing sections); CompozyOS cannot narrow Codex's own MCP configuration over
ACP.

## Observations outside this change

- A lab CLI run from inside a managed session inherits `COMPOZY_SESSION_ID` and fails with
  `agent identity cannot be validated`; the bootstrap helper does not scrub it.
- `compozy__agent_instance_list` with `peers: true` includes the calling agent; the roster
  excludes it.
- One Claude session was titled from the standing block text instead of the user text.
- `compozy agent instance list --peers` shows `accepts` only with `-o json`.
