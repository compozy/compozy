# Harness dogfooding: prompts, native tools, and MCP

Read-only research at checkout `74f33b7d4`, observed on 2026-10-22/23. Product changes and verification belong to the parent implementation. This slice changed only this report.

## Scope and evidence quality

- Opened the global database and all 223 retained per-session `events.db` files with SQLite URI `mode=ro`. The global catalog contained 82 archived sessions. Session creation spans 2026-10-19T23:06:58Z through 2026-10-23T02:33:06Z. These sources do not establish behavior over all the preceding weeks.
- Surveyed all retained `tool_call` and `tool_result` rows, including archived-session databases, then read the particular event ranges below. One survey counted 67,557 tool events; a subsequent deduplicated survey counted 25,940 calls. The daemon remained active, so these are separate observation snapshots, not a transactionally frozen denominator.
- Deduplication uses `(session directory, tool_call_id)`, not the number of streaming call updates. Native names are matched as complete canonical/hosted tool names, never a substring of shell-command titles.
- At the latter snapshot, native skill reads numbered 1,238: 693 in archived sessions and 545 in nonarchived sessions. The three most-read Compozy references were `native-tools.md` (155 reads, 83 sessions), `tools-and-skills.md` (151, 78), and `tasks-and-orchestration.md` (141, 73). These counts measure calls, not token cost or redundant work.
- Inspected the six retained `compozy*.log` files for MCP/skill messages. Their retained window was approximately 2026-10-22 09:48–23:38 -03:00. Log retention/noise remediation is owned by the parent slice. No evidence here supports a general third-party MCP connectivity defect.
- Read current prompt assembly, native tool schemas/handlers, hosted MCP result/error conversion, permission inheritance, history pagination, relevant tests, and recent Git history. No tests or runtime mutations were performed in this research slice.

Evidence notation below is `~/.compozy/sessions/<id>/events.db`, `events.sequence`, and UTC timestamp. Reports retain identifiers, sizes, error classes, and technical behavior; they do not copy private task briefs or unrelated conversation text.

## 1. Derived user sessions lose their effective delegation tool budget

**Classification:** verified current defect; high confidence. This is the principal deterministic cause behind the prompt stalls in finding 2.

### Evidence

The read-only global rows show:

| Session | Type / lineage | Parent | Stored native tool grants |
| --- | --- | --- | ---: |
| `sess-cc1f6108f1e83aa1` | user / root | none | 242 |
| `sess-1f21a9fed7f33635` | user / fork | `sess-cc1f6108f1e83aa1` | 0 |
| `sess-9ab78bb254a056df` | spawned / spawn | `sess-1f21a9fed7f33635` | 0 |

In the forked parent's events:

- Sequence 1093, 2026-10-23T02:07:25.401259Z: `subagent_delegate` requests explicit native tools including `compozy__skill_view`.
- Sequence 1096, 02:07:27.219165Z: structured error `permission_escalation_denied`, stating that `compozy__skill_view` is outside the caller's budget.
- Sequences 1099/1100, 02:07:44.036996Z/02:07:44.062033Z: the same parent successfully invokes `skill_view`, returning 5,669 characters of skill content. This proves that the parent's actual callable capability and its delegation budget disagree.

The failure at sequence 1096 has ACP `status: completed` while its structured payload is an error. Counting only failed ACP statuses would miss this incident.

### Current mechanism

- `internal/session/manager_allowed_tools.go:87-117`, particularly line 101, skips concrete tool-grant materialization whenever lineage has a `ParentSessionID`. That includes a derived **user** session with provenance, not only a governed spawned worker.
- `internal/daemon/tool_policy_resolver.go:342-370` enforces stored session tool grants for spawned/coordinator sessions, but returns without imposing them for user sessions.
- `internal/daemon/native_tool_subagent_permissions.go:38-56` validates explicit narrowing against the stored list.
- `internal/session/subagent_request.go:157-187` inherits that same stored list; `subagent_delegate.go:90-107` carries it into the spawned child.

Thus the fork remains able to call tools directly, but delegates an empty ceiling and cannot explicitly grant its callable tools. Do not generalize this to every root session: root materialization already exists and the source root in this chain has 242 entries.

### Minimum sufficient correction and validation

Distinguish provenance lineage from authority lineage when materializing the delegation ceiling. A derived user session needs a concrete ceiling corresponding to its resolved authorized tool policy; a spawned/coordinator session must continue to honor its inherited/narrowed ceiling. Reuse the existing materializer and policy owners instead of introducing a second generic permission resolver or granting all tools.

Invariant: a forked/continued user session can delegate a native tool it is authorized to use, while explicit denies, an explicit empty override, child narrowing, provider-disabled session MCP, and spawned-parent ceilings remain effective.

- Owning session suite: `internal/session/manager_lineage_test.go`, existing case “Should persist concrete root delegation tools for nil and empty overrides”; `manager_allowed_tools_test.go` owns concrete allowlist/deny expansion.
- Native boundary suite: `internal/daemon/native_tools_test.go`, `TestNativeSubagentPermissionBoundary`, for callable-parent versus child-ceiling integration.
- Real validation: derive a user session, delegate a small read-only task requiring one permitted native tool, observe successful child use, and confirm a denied tool remains absent/refused. A mock that manually supplies a populated parent lineage does not cover this trigger.

## 2. The inline router turns ordinary delegated work into a mandatory native-tool bootstrap

**Classification:** verified prompt usability defect; high confidence. Prompt correction is complementary to finding 1: intentionally bounded children must also be able to perform ordinary provider-owned repository work.

### Evidence

Child `sess-9ab78bb254a056df`:

- Sequences 22–33, 2026-10-23T02:33:37Z: the agent stops without changing files because `skill_view`/`tool_search` are unavailable and the instructions require `tasks-and-orchestration.md` before acting as a worker.
- Sequences 51–65, 02:34:54Z: it repeats the same blocker after another task instruction.
- Sequence 70, 02:38:59.467806Z: the parent supplies a 120,469-byte prompt containing the complete references.
- Sequence 79, 02:39:08.395362Z, followed by 82/85/89: provider-shell repository reads begin. The task did not first require a daemon Task claim or review verdict.

The parent also redelegated three short briefs after similar child stalls. Its sequences 1062/1067/1073 at 02:06:03–02:06:33Z contain briefs of 2,267/2,509/2,195 characters. Replacement briefs at 1126/1131/1135 at 02:08:11–02:08:39Z contain 104,867/105,185/104,921 characters after references were included. This is measured prompt expansion, not a token or latency estimate.

### Current mechanism

- `skills/compozy/SKILL.md:22`, router row around line 40, and final STOP at line 90 apply mandatory full-reference reading to “coordinator, worker, or reviewer” without distinguishing a general delegated coding role from a daemon Task role.
- Lines 79–88 permit only a narrow native-descriptor fallback. The exception does not explain ordinary provider shell/edit/search work.
- The correct boundary already exists in `skills/compozy/references/tasks-and-orchestration.md:34-39`: missing native grants do not forbid ordinary provider tools, while CLI/filesystem bypass of a withheld Compozy capability remains forbidden. That explanation is inside the very reference the child cannot read.
- `internal/daemon/prompt_sections.go:130-165` injects the router when tools and skills sections are enabled; this configuration-level selection does not establish that a child's effective `skill_view` grant is callable.

At this checkout the three broad manuals total 121,001 bytes (`native-tools.md` 65,636; `tools-and-skills.md` 24,533; `tasks-and-orchestration.md` 30,832). `runtime-operations.md` is another 124,017 bytes. The frequency and expansion above support narrowing the router's trigger; they do not justify deleting the operating contracts.

### Minimum sufficient correction and validation

Place the provider/native permission distinction in the inline router. Say that a delegated code/research/review assignment alone does not create a daemon Task claim or task-review authority. Require task lifecycle guidance when actually using those contracts. Missing native reference access should block only the dependent Compozy operation; ordinary authorized provider work can continue. Preserve all prohibitions on bypassing native policy through CLI or direct state access.

Invariant: an ordinary delegated repository task can begin with intentionally unavailable Compozy native tools, without a 100 KB parent-pasted manual; actual daemon Task claims/reviews still require their authority guidance.

This is a behavioral prompt contract. Do not add brittle tests that assert the exact explanatory prose. If startup routing code changes, extend its existing daemon prompt assembly suite. Validate the instruction change with a real bounded child performing a harmless repository read/edit and a separate task-authority case. `skills/bundled_test.go` verifies embedded resource availability, not LLM compliance.

## 3. Small history limits still return entire raw tool-heavy turns

**Classification:** verified current capability gap and efficiency problem; high confidence. The documented limit is a turn count, so this is not evidence that the existing count contract is broken.

### Evidence

- Archived caller `sess-8a00287790c82242`, sequences 581/583, 2026-10-21T19:50:12.512947Z/19:50:12.609783Z: `session_history` with `limit: 2` for `sess-b28590a8f6e4860a` returns a retained artifact of **817,423 bytes** (`truncated_from_bytes: 817418`). At 588 the agent tries to read only the tail at byte offset 811000; by 593/594 it falls back to CLI history extraction.
- Caller `sess-bbf023e40086b881`, sequences 605/607, 2026-10-22T21:27:47.138731Z/21:27:47.547287Z: `limit: 1` for `sess-d70e112bb2ef37e1` returns a **1,649,596-byte** artifact (`truncated_from_bytes: 1649590`). Sequences 610/611 immediately switch to CLI event extraction.
- These were the two directly observed canonical `session_history` calls in the deduplicated survey. This supports the concrete workflow gap; it does not establish a fleet-wide latency percentile.

### Current mechanism

- `internal/tools/builtin/sessions_read.go:124-145` defines `limit` as turns for history.
- `internal/daemon/native_tool_session_pages.go:139-147` expands each selected turn; lines 204–220 repeatedly append every matching event until exhausted.
- `internal/daemon/native_tool_observe_helpers.go:151-165` serializes all those events, including raw tool payloads.
- `internal/daemon/native_tool_session_calls.go:282-317` builds the complete result before the generic result offloader applies its bound.
- The official subagent result hint directs callers to full `session_history` (`internal/session/subagent_request.go:196-198`; `internal/daemon/native_tool_subagent_results.go:118`), although they commonly need the final answer rather than the execution ledger.
- `session_search` and `session_outline` already expose projected snippets and previews (`internal/tools/builtin/session_navigation.go:9-56`), but neither provides a complete selected answer. Reuse the projection where appropriate instead of inventing a second transcript store.

### Minimum sufficient correction and validation

Add an explicit compact/messages view to the existing native history surface (or reuse an existing complete transcript projection), preserving the current full-event default and raw continuation contract. Make the compact view exclude tool payloads and retain stable session/turn/message/sequence identifiers, ordering, archive selection, and explicit truncation/continuation. Point answer-recovery guidance to the compact option. Do not silently replace full history or label omitted tool events as a complete audit log.

Invariant: retrieving the answer from a tool-heavy turn avoids copying its raw tool output; full history remains lossless and unchanged. A single oversized assistant answer still needs a truthful byte bound/continuation; selecting fewer turns is not a byte bound.

Canonical suite: history pagination cases in `internal/daemon/native_tools_test.go` (`TestDaemonNativeTools`, existing forward/backward/default-page coverage around lines 7596–7791). Extend the owning suite with a turn containing large tool output plus a short final answer, and validate schema/transport behavior through existing hosted result tests. Real validation should query such a retained/test session through the native tool and read the answer without an artifact-tail guess or shell fallback.

## 4. Schema failures lose the safe field-level correction at the MCP boundary

**Classification:** verified current diagnostic gap; high confidence. Keep internal/backend masking intact.

### Evidence

- `sess-32632022cb5223c4`, sequences 846/848, 2026-10-23T02:10:42.814048Z/02:10:42.846431Z: `monitor_create` supplies `kind`, `delay_seconds`, and `note`; it omits required `description`. The only agent-visible error is `schema_invalid: invalid tool request`.
- The current descriptor still requires `kind` and `description`, and rejects additional properties (`internal/tools/builtin/session_monitor.go:93-111`). This is a directly attributable request correction, not an inferred backend failure.
- `sess-14d510d1a604eacc`, sequences 933/935, 2026-10-22T20:14:03Z: another `terminal_wait` rejection supplies the same generic message. Other monitor failures had apparently complete fields, so their actual cause must not be guessed from that message.

### Current mechanism

- `internal/tools/schema.go:71-78` retains a validation cause internally, while constructing only a generic public `ToolError` message. Required/property/type checks know their schema path.
- `internal/api/core/tool_errors.go:121-132` intentionally substitutes `safeToolErrorMessage`; only explicitly authored operator-safe details survive.
- `internal/mcp/hosted_proxy_transport_error.go` retains/filter-projects the existing `operator_cause` and `operator_recovery` details for transport-provided errors. This is an existing bounded route for safe correction guidance.

### Minimum sufficient correction and validation

Project a bounded, schema-derived correction through the existing safe-detail mechanism: for this example, identify declared `description` as missing and direct the agent to the live descriptor. Preserve `tool_invalid_input`/`schema_invalid` codes, validation-before-side-effect behavior, and backend-error masking.

Do not expose raw `err.Error()` indiscriminately. Do not echo submitted values, tool input, secrets, backend paths, arbitrary unknown-property names, or unconstrained nested keys. A declared property path and a small violation class are enough; unknown keys can be described as an unexpected property at the containing schema path. If exposing detailed paths, only use descriptor-declared segments and bounded numeric indexes. Sanitize and cap details in both structured and text-only hosted paths.

Invariant: malformed native input names the safe correction and causes no backend call; secret-bearing input and backend failures remain masked.

- Owning validation suite: `internal/tools/dispatch_test.go`, `TestRuntimeRegistryDispatchValidationAndPolicy` and its schema-error coverage.
- Public projection suite: `internal/api/core/tools_test.go`, `TestToolErrorResponses`.
- Hosted transport suite: `internal/mcp/hosted_proxy_result_test.go`, `TestHostedToolResultContract`, plus hosted error helper cases in `hosted_proxy_test.go`.
- Use one invariant owner per layer: validator creates safe structure, API preserves/masks it, MCP text and structured forms agree. Do not duplicate broad schema behavior across every individual tool.

## Historical signals deliberately excluded from new fixes

- Repeated session-create/status/archive MCP failures reported `data/session/runtime must NOT have additional properties`. The survey observed 24 failed-status create calls, 22 archive calls, and five status calls, largely carrying this message. Current `sessions_create_schemas.go:39-90` already shares a complete runtime schema, and `TestDaemonNativeSessionResultsMatchOutputSchemas` covers ready/recovering/unbound session results. Both landed in `00dcc3da2` (#885). Later retained errors do not establish that the current checkout still has the bug; the running daemon/provider catalog version was not pinned for each historical call.
- Oversized MCP success envelopes were already corrected by `cbd984bd0`; current `hosted_proxy_output_schema.go` accepts continuation/error envelopes. Finding 3 concerns the useful content and amount assembled, not re-fixing that envelope.
- `skill_view` source shape for command-qualified reads was fixed by `13aeecc40` (#892). Do not turn pre-fix skill errors into a second implementation of that fix.
- Skill verification/override warnings and hosted bind logging are observable, but are not sufficient evidence to change verification policy or weaken tool grants.

## Remaining limits and delivery implications

No production change, live provider replay, fresh MCP handshake, token benchmark, or integration test was performed by this slice. Parent implementation must verify each changed behavior and record public/native/official-skill impact. History additions and diagnostic payload changes must keep existing callers compatible and use their owning generators where applicable. Prompt changes alone do not repair the derived-session ceiling defect, and ceiling repair alone does not excuse an ordinary bounded worker from the router's current STOP rule.
