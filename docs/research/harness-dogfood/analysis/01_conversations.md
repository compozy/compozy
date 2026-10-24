# Conversation evidence: orchestration friction

Baseline: `74f33b7d460866b8061544e7eb98b73dbe972901`. Investigation date: 2026-10-22 local / 2026-10-23 UTC. Source timestamps below are UTC. This slice made no product changes and did not mutate the live Compozy home.

## Scope and method

All SQLite connections used `file:...?...mode=ro` through Python `sqlite3`; no daemon restart, session mutation, WAL checkpoint, or live write was performed. The inventory came from `~/.compozy/compozy.db`, especially `sessions` and `session_subagents`. Conversations came from `~/.compozy/sessions/<session-id>/events.db`, using both `events` and `transcript_entries`. Provider tool-call updates were grouped by `tool_call_id`; a streaming update is not another invocation.

At the initial inventory, there were **223 session records and 223 corresponding event databases**: 70 `user`, 133 `spawned`, and 20 `system`. **82 were archived** (62 user, 20 spawned); 141 were not archived. Creation dates ranged from 2026-10-19 to 2026-10-23. The available conversations therefore do not represent several weeks of retained history. Recent Git history was checked back through October 1 to distinguish historical fixes. No event rows had `archived=1`; session archival is separate from event archival.

The event/type inventory covered every one of the 223 databases. It contained 429 `user_message` rows: 221 explicitly attributed to another session and 208 without that origin. Filtering to `session_type=user` left **105 origin-free rows in 20 sessions**; these are candidates for authored user input, not a trustworthy human-message count, because a continuation bootstrap can also lack origin. All 70 user-type sessions were screened by metadata and input origin. Detailed conversation/adjacent-tool inspection selected eight conversations across active and archived states, plus four delegated-child result records and the full blocking/recovery segment of one child:

| Session | State at inventory | Why selected |
| --- | --- | --- |
| `sess-8a00287790c82242` | Archived | Large cross-provider implementation orchestrator; explicit correction from subagents to governed child sessions |
| `sess-fbab1dc607f782e8` | Archived | Cross-workspace implementation of improvements discovered by the preceding orchestrator |
| `sess-bbf023e40086b881` | Active | Current issue orchestrator with many children, operator corrections, crash recovery, and merge handoff |
| `sess-98d4f8a42fbebfda` | Archived | Operator reproduced profile-specific agent discovery failure |
| `sess-271c8af963f77be9` | Archived | Highest recorded child count, 45; repeated design-quality and progress corrections |
| `sess-b43d2e72c2916e14` | Archived | Spec/reviewer coordination and question-formatting corrections |
| `sess-cc1f6108f1e83aa1` | Archived | Source conversation for the currently failing fork |
| `sess-1f21a9fed7f33635` | Active | Forked Codex orchestrator with eight delegated children and directly observed permission/bootstrap failures |

Counts were read during active use, not from an atomic snapshot spanning all databases. The session IDs and event sequence/timestamp pairs below are the reproducible evidence. Business requirements, customer material, prompt bodies, credentials, and raw tool output are deliberately not copied into this report.

## 1. P1 — Forked user sessions lose the delegation tool budget

**Classification: current defect, with live causal evidence and a matching path in the baseline source.** An independent isolated reproduction has not yet been run by this read-only slice.

`sessions` records show that `sess-1f21a9fed7f33635` is `session_type=user`, `lineage_kind=fork`, with parent/root `sess-cc1f6108f1e83aa1`. Its source root has **242 concrete tool grants**, while the fork's `permission_policy_json.tools` is empty. Its delegated child `sess-9ab78bb254a056df` is `session_type=spawned`, `lineage_kind=spawn`, and also has an empty grant list. A separate active root, `sess-bbf023e40086b881`, has 240 grants, so this is not a universal absence of tool policy data.

The fork delegates three implementation slices at parent sequences **1062, 1067, 1073**, 2026-10-23 **02:06:03–02:06:33**. Requests use `mode=async`, `role=implementation`, Codex/Astra, and no permission narrowing. Their task texts are 2,267, 2,509, and 2,195 characters. The durable results for children `sess-e914b48c909850a1`, `sess-a5f772874b5b2790`, and `sess-53633778d213090f` all report stopping before any edits because `skill_view`/`tool_search` were unavailable and the task-orchestration reference could not be loaded.

The parent tries to pass explicit native tools at **1093**. At **1096, 02:07:27.219165**, the tool rejects the request with `permission_escalation_denied`, identifying `compozy__skill_view` as outside the caller's budget. Yet the **same parent successfully invokes `skill_view` at 1100**, seconds later. This establishes a mismatch between what the parent can use and what the delegation validator considers inheritable; it is stronger evidence than the children's prose alone.

The parent works around the failure by copying loaded instructions into three new task bodies at **1126, 1131, 1135**, 02:08:11–02:08:39: **104,867, 105,185, and 104,921 characters**. These replacement workers complete their slices. A fourth child, `sess-9ab78bb254a056df`, repeats the bootstrap stop at child **22–33, 02:33:37–02:33:40**, and **51–65, 02:34:52–02:34:55**. After the parent supplies the requested references at child **70, 02:38:59.467806**, it resumes normal provider-shell reads at **79, 02:39:08.395362**, then edits. The effect is wasted delegates, repeated prompts, latency, and substantial avoidable context expansion. The experiment does not isolate the exact token cost of any single reference.

**Current ownership and semantics:**

- `internal/session/manager_allowed_tools.go:87–117`, `materializeRootDelegationTools`, materializes concrete delegation tools for user/system sessions, but returns early at line 101 whenever `ParentSessionID` is nonempty. A fork is a first-class user session with provenance parentage, so this check excludes the affected session.
- Its existing `concreteDelegationTools` helper expands the resolved agent's explicit tools/patterns/toolsets against the native tool universe and removes agent deny patterns. This is the current materialization owner; do not replace it with a blanket list of all tools.
- `internal/daemon/native_tool_subagent_permissions.go:38–54` validates explicit child narrowing against the raw lineage budget. `internal/session/subagent_request.go:171–187` inherits that same raw budget; `internal/session/spawn.go:185–209` checks it again before and after the spawn hook.
- `internal/daemon/tool_policy_resolver.go:342–370` enforces lineage tools only for spawned/coordinator sessions. Thus `tools=[]` on a user session does not restrict its own native calls, while `tools=[]` on its spawned child becomes an enforced empty allowlist.
- Actual callable tools are **not derived from config alone**: `nativeToolPolicyResolver.Resolve` binds session workspace/profile, resolves layered config and agent restrictions, adds source trust/Loop ownership, then session restrictions and approval availability. `internal/tools/policy_evaluate.go` evaluates visibility, denies, session/agent allowlists, source policy, and permission ceiling. `RuntimeRegistry.SessionProjection` additionally applies provider availability (`internal/tools/registry_session_projection.go:121–164`). These checks remain authoritative when a child calls a tool.

**Minimal safe direction:** repair delegation-budget materialization for user fork/continue/provenance relations according to their governing lineage semantics, preserving explicit restrictions and the existing spawn subset checks. Audit both creation and resume of an already persisted affected fork. Do not interpret an empty spawned-worker grant as unlimited, bypass the registry, or silently grant withheld tools. The same current `skills/compozy/SKILL.md:90` unconditionally requires the full task-orchestration reference before acting as any “worker” or “reviewer”; scope this to runtime task authority so ordinary code work is not falsely blocked. Prompt wording is a complementary fix, not a substitute for the permission repair.

**Invariant and verification owner:** a user-derived session which can use a native capability retains the intended delegation ceiling, and a narrowed governed child can never exceed it. Reuse `internal/session/manager_lineage_test.go`, `manager_derive_test.go`, and `manager_allowed_tools_test.go` for materialization; `internal/daemon/native_tools_test.go` (`TestNativeSubagentPermissionBoundary`) owns actual delegate-boundary behavior. Final proof should be an isolated real session: root → fork → delegate → child `skill_view`, plus a denied capability that remains denied, and a resume of a persisted fork. Verify prompt behavior with an actual small code/review task, not assertions that freeze English prose.

## 2. P2 — Validation diagnostics erase the field needed to recover

**Classification: current defect in agent-facing diagnostic usefulness; live failed/corrected pair.**

In active `sess-1f21a9fed7f33635`, **1755–1756, 2026-10-23 02:37:52**, `monitor_create` receives a **203-character description**. The public schema permits 200 (`internal/tools/builtin/session_monitor.go:84`). The result only says `schema_invalid: invalid tool request`; its structured error contains code, reason, and tool ID, but no field or limit. At **1759–1760, 02:37:57**, the agent retries with a 119-character description and also drops valid `timeout_seconds` and `until` arguments; that request succeeds. The lack of a field-level diagnosis made the correction broader than necessary.

`internal/api/core/tool_errors.go:131–155` builds a deliberately masked error envelope, and `safeToolErrorMessage` maps invalid-input failures to the generic text at lines 185–189. `internal/tools/descriptor_input_error.go` only exposes limited schema diagnostics for descriptors with an explicit `InputErrorCode`; monitor creation has none. `internal/mcp/hosted_proxy_transport_error.go` preserves safe operator failure details, so there is already a transport pattern for useful bounded diagnostics.

**Minimal safe direction:** expose safe schema facts such as field path, violated keyword, and required maximum, with a recovery hint. Do not serialize arbitrary backend error strings or user field values. Keep stable error/reason codes. The owning invariant is that invalid requests remain unexecuted while HTTP/UDS/hosted-MCP callers receive the same actionable, non-sensitive validation fact. Reuse `internal/tools/dispatch_test.go`, `internal/api/core/tools_test.go` (`TestToolErrorResponses`), and the existing hosted transport/parity suites at their distinct boundaries; an isolated real hosted call with a 201-character monitor description should identify `description` and 200 and produce no monitor.

## 3. P2 — A bounded turn count is still an expensive way to read a child's answer

**Classification: persistent usability/performance gap, observed twice; not an assertion that existing pagination violates its documented contract.**

- Archived orchestrator `sess-8a00287790c82242`, **583, 2026-10-21 19:50:12.609783**: `session_history(limit=2)` returns a retained-result envelope with `truncated_from_bytes=817418`.
- Active orchestrator `sess-bbf023e40086b881`, **607, 2026-10-22 21:27:47.547287**: `session_history(limit=1)` returns `truncated_from_bytes=1649590`.

The first incident is also reported to the improvement worker at `sess-fbab1dc607f782e8` **353, 2026-10-21 19:51:51.152115**, specifically while trying to inspect a child's result. The result envelope is valid and preserves the full data; it does not give the caller a compact answer.

At the baseline, `internal/daemon/native_tool_session_pages.go:91–149` bounds backward selection by turns, then `readNativeSessionTurnEvents` hydrates every event of each selected turn. `internal/daemon/native_tool_session_calls.go:278–317` returns the resulting full event history. Documentation correctly says the limit is turns. Recent #885 improved first-prompt reply watches and avoids duplicate bare completion wakes, but direct child inspection and truncated-result recovery still point at full history.

**Minimal safe direction:** provide an additive, bounded answer/message read using the existing transcript projection, with turn/message identity and an explicit continuation/truncation contract. Preserve the full event-history surface for diagnostics. Prefer the already-existing reply-watch final answer when awaiting a known message. Reuse the native session read suite in `internal/daemon/native_tools_test.go` and the transcript/store owner for reconstruction; verify with an isolated long, tool-heavy child turn that a final-answer read returns the consolidated text within its byte limit without loading tool payloads. Do not add another independently maintained summary store.

## Historical corrections and work already underway

| Evidence | Current disposition |
| --- | --- |
| `8a00287790c82242` 193, 2026-10-21 19:38:56: operator corrects use of subagents when child sessions were requested; 425/436/446 reject managed-worktree spawn paths; `fbab1dc607f782e8` 340/353 reports sandbox and provider-shell confusion | #885 (`00dcc3da2`) adds spawn worktree binding, inherited permissions, initial prompt/reply watch, and field/rule diagnostics. `skills/compozy/references/tasks-and-orchestration.md:34–39` now explicitly separates provider shell from withheld Compozy capabilities. Do not reimplement these original requests. |
| `8a00287790c82242` has 17 failed native `session_create` results and 22 failed `session_archive` results, many due to output-schema rejection after the action; e.g. 328 at 19:42:27 | #885 replaces duplicated runtime schemas with `sessionRuntimeOutputSchema`; current source co-ships it. Treat these recorded old failures as historical unless a current response violates the new schema. Counts are unique result rows, not streaming tool-call updates. |
| `98d4f8a42fbebfda` 32/82, 2026-10-22 21:04:46/21:07:24: operator insists the active profile's designer agent should be discoverable and asks for a reproduction | Fixed in current main by `e4e0192cc`, session-bound agent catalog resolution. |
| Active orchestrator `bbf023e40086b881` 353/1244/1365/1416: split prose, list collapse state, All Profiles parity and nested organization | Corresponding recent fixes are in current main: `4c3656a42`, `f373028fb`, `fff59e6d5`, `080db8dbf`. These are evidence of successful dogfooding, not new findings. |
| `bbf023e40086b881` 3124, 2026-10-23 01:43:31: long-history opening does not reach the end; running sessions need separate stop/archive operations | Current active implementation sessions `sess-32632022cb5223c4` and `sess-a0dbd5311773107a` own issues #907/#908. Avoid duplicate changes; this slice did not verify their later PR/CI state. |
| `fbab1dc607f782e8` 1543 and `bbf023e40086b881` 2143: operator repeatedly directs expensive gates to CI because the machine becomes overloaded | Useful orchestration-contract evidence; insufficient alone to justify an automatic command blocker or global resource scheduler. Preserve the current user-specified verification budget in worker prompts. |

## Observability limitations and recommendations

An origin-free `user_message` is not necessarily human input: system/reviewer bootstrap prompts and derived-session continuation text appear in that category. Metrics should use explicit ingress provenance rather than classifying by event name or session type. This investigation used adjacent content and metadata to avoid counting agent messages as operator corrections.

Many Codex event envelopes use the provider thread UUID in `content.session_id`, while the event database directory and global catalog use the Compozy `sess-*` ID. Keep both identifiers distinct when joining evidence. Reading only that payload field or only the current session list would miss archived and derived-context relationships.

Only three days of retained conversation databases were available at this location. Deleted sessions referred to by surviving conversations may no longer be available. No claim about a multi-week failure rate, all historical human messages, or whole-product performance is supported. The priority above rests on reconstructed incident chains and current code paths, not keyword counts.
