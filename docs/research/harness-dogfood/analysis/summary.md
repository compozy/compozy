# Harness dogfooding: evidence and implemented improvements

Investigation: 2026-10-22/23. Baseline: `74f33b7d460866b8061544e7eb98b73dbe972901`.
Implementation branch: `fix/harness-dogfood-2026-10-22`, isolated from the operator checkout.

## What the retained evidence supports

Read-only inventory covered 223 retained session databases, including 82 archived sessions.
The conversations span approximately 3.5 days, October 19–23, rather than several weeks.
Recent Git/PR history was inspected separately to distinguish current defects from fixes already
shipped. The daemon remained active, so counts from separate researchers are observation
snapshots, not a transactionally consistent fleet census. Reports retain technical identifiers
and bounded facts; raw conversations, credentials and business briefs were not copied.

The six retained log files cover about 14 hours at the observed rotation settings. Successful
GET access lines account for 79.7% of their bytes; all access lines account for 83%, and hook
dispatch lines another 13%. Reducing those successful records at INFO should retain more useful
incident context within the same size limit. This is a volume-based expectation, not a measured
post-deployment retention or latency improvement.

Independent evidence:

- [Conversation and archived-session analysis](01_conversations.md).
- [Prompt, skill, native tool and hosted MCP analysis](02_prompts_tools.md).
- [Claude Opus 5.5 initial analysis](03_opus_initial.md).
- [Idle release implementation and worker verification](04_idle_release_fix.md).

The Opus worker ran as a real Claude TUI through herdr, with `claude-opus-5-5 --effort max`;
the controller independently observed **Opus 5.5 / max** in the live status line. Its report's
uncertainty about effort reflects the worker's own visibility, not an unverified controller
launch. Native researchers and Opus inspected distinct evidence in parallel; the controller
verified the high-impact source paths and owns the final implementation and delivery checks.

## Decisions and resulting behavior

| Observed problem | Change | Evidence and scope |
| --- | --- | --- |
| A forked user could call a tool but could neither grant it explicitly nor pass it to a child. Source had 242 grants; fork and child had zero. | Treat fork/continue as provenance, not governed-child authority, when materializing the existing delegation ceiling. Materialize after lineage refresh so legacy resumed sessions are repaired. | Red/green manager regressions cover new fork/continue and legacy resume. Existing daemon/hosted-MCP journey now covers root, fork and continue with child calls and a grandchild. Spawned ceilings, explicit denies and narrowing remain authoritative. |
| Ordinary delegated coding stalled because an unavailable native manual was required merely for being called a worker/reviewer. One parent expanded a short brief to roughly 105 KB. | Put the provider/native distinction in the inline router. Actual Compozy operations and daemon Task authority still require the matching guidance. | Instruction scope and all reference paths reviewed. No new prompt assembly branch. No claim of measured token savings or improved model compliance from an automated wording assertion. |
| Invalid monitor input returned only `schema_invalid`, leading to speculative retries. | Attach bounded, schema-authored correction guidance to existing operator-safe error fields. Monitor validation exposes its authored field constraints through that same contract. | Validator, API/MCP projection and actual hosted-stdio rejection checks retain stable codes, block execution and exclude submitted secrets/unknown keys. Generic backend errors stay masked. |
| Reading a child's answer with a one- or two-turn limit produced 0.8–1.65 MB raw history artifacts. | Direct answer recovery to the existing `type=agent_message` history filter, with turn identity and existing cursors/artifact continuation. | Real SQLite contains large tool results and chunked Unicode answers; filtered reads return complete small answers in both page directions without hydrating tool payloads. No new endpoint, projection or compatibility alias. |
| Idle runtime cleanup used a 5-second caller deadline while ACP budgets 5 seconds for stop plus 3 for drain. Nine observed Codex releases failed at that boundary. | Report the driver stop budget through the existing adapter and use it for runtime replacement/release; give the stopped hook its own bounded context. | Existing idle-release suite verifies that the stop can finish beyond five seconds, exit ownership remains durable, and the next prompt uses session/load. ACP subprocess tests and the daemon late-reply journey pass. |
| Successful read polling and hook events dominate the bounded logs; hook records lack available identity. | Successful GET/HEAD/OPTIONS, successful hooks and empty sweeps/reconciliations use DEBUG. Mutations/repairs stay INFO; rejections/failures remain WARN/ERROR. Hook records include available session/workspace/turn/task/run identity. | Existing HTTP redaction/client-identity checks and hook dispatch suites verify severity and correlation. Persisted hook/event records and metrics remain intact. DEBUG restores detailed request tracing. |

History `limit` remains a count of matching turns, not a byte bound. Assistant chunks are returned
in sequence order and must be combined; a large assistant answer can still use the existing
artifact continuation. Omitting `type` preserves full diagnostic history. No session data is
rewritten, no SQLite/config shape changes, and no tool ID or route is removed.

## Additional confirmed findings retained for follow-up

These are existing limitations, not claimed fixes in this change:

1. **Provider rate limits exhaust wake delivery attempts too quickly (Opus F1).** The 200 ms
   multiplier and three-failure budget are confirmed in current code. One observed turn already
   ran tools before failing; two successor wakes exhausted the budget before provider recovery.
   The notification becomes disposed, while the subagent result remains saved and readable.
   Reply-watch admission likewise does not guarantee a successful recipient turn. A safe repair
   needs a persistent rate-limit/auth recovery policy, restart behavior, bounded backoff, and
   protection against replaying partially executed actions or overriding explicit cancellation.
   Merely increasing a timer or blindly replaying the turn would not establish those guarantees.
2. **Auto-title cost (F4).** Forty-five observed full Opus title sessions consumed a provider-reported
   $16.57, with a median 16.7-second prompt. No default provider/model was silently changed.
   A cheaper configured role or bounded title path needs explicit model-selection/fallback semantics.
3. **Inline image duplication (F6).** Retained ledger and transcript projections contain roughly
   957 MB of base64 payloads across 35 sessions. Moving them to artifacts needs a lossless
   authoritative-ledger migration and projection rebuild plan under SD-013; deletion is not a fix.
4. **Further observability work (F5/F8).** Repeated deterministic warning sources, UDS caller
   attribution and ambiguous stop-escalation labeling remain separate concerns. Changing persisted
   numeric stop causes requires a compatible reader transition.
5. **Catalog budget saturation (F7).** All 162 sampled first-turn skill spans reached the cap.
   This establishes saturation, not that a wrong skill was selected. No catalog policy changed.

The independent report's restart/liveness and one-off transport hypotheses remain unconfirmed.
#885 (`00dcc3da2`), #892 (`13aeecc40`) and `cbd984bd0` already address the observed historical
output-schema/skill-source/budget-envelope failures; they were not reimplemented. Concurrent
transcript-scroll/archive work was left untouched.

## Verification boundary

The [focused verification record](../../../qa/reports/2026-10-23-harness-dogfood.md) lists executed
checks and their inputs. Real daemon/CLI/HTTP/hosted-MCP subprocesses use the repository's
existing deterministic ACP fixtures in disposable homes. This is integration evidence, not a
new live Claude/Codex persona walk or a production rollout. The source `~/.compozy` home was
read-only throughout research. No browser behavior changed.

Final local gate and exact-head CI results belong to the PR delivery record. The PR must remain
unmerged; this investigation does not authorize changing the operator's configuration or
rewriting existing session state.
