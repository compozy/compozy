---
id: RT-agent-memory-startup
area: RT
title: Agent memory is a bounded startup snapshot shared by definition identity
persona: Dora
journey: J-31
expected: Stays-on sessions receive fenced notes from the correct profile and definition origin. On-demand sessions receive no memory. Existing sessions retain their startup notes after edits.
entry_points: agent memory CLI and API; session startup
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: MS-web-stays-on-placement
---

1. In an isolated lab, create a user-level stays-on agent and add a note from workspace A.
2. Start that agent in workspace B. Inspect the startup manifest and confirm the note retains its source scope inside the agent-memory data fence.
3. Edit the note, then send another turn to the running session. Its startup snapshot remains unchanged. Start a new session and confirm it receives the edited note.
4. Define a repository agent with the same slug. Confirm it starts with separate memory; repeat with another operator profile.
5. Include Markdown fences and a closing `compozy-agent-memory` tag in a note. Confirm the rendered section keeps both inside the data envelope.
6. Confirm an on-demand agent receives no memory section and an empty memory file adds no section.

Implementation evidence: `TestAgentMemoryStartup` exercises the composed startup assembler with the real file store. The public-interface walk remains pending the CLI/API slices; unit evidence does not mark this scenario walked.

API parity checks: repeat inspect, digest-checked edit, add, replace, remove, search, validation, history and rollback over HTTP and UDS. A stale digest returns 409 with `agent_memory_conflict`; an on-demand definition returns 422 with `agent_not_stays_on`. For agent callers, use two repository definitions with the same slug and confirm the caller cannot select the other origin. The shared-handler suite exercises file-backed mutations; network transport and native-tool walks remain pending.

Native-tool checks: in a stays-on session, add a note, replace its text, search the result, then remove it. Confirm the structured digest/budget and removal count. Reject target agent/profile/workspace fields and on-demand callers. Repeat own-memory writes under `approve-reads`, then confirm `deny-all` still requires approval. Registry tests cover the file store and dispatch; the live lab walk remains pending final integration.

2026-11-01 (always-on-agent N-47) QA impact: replace a repeated substring so the
result exceeds 1,000 characters. Expect `agent_memory_entry_too_large` and unchanged
content/digest. A result of exactly 1,000 Unicode characters remains accepted,
including entries with expiry metadata. Owning filesystem suite: `TestStore/Should_bound_replacement_results_and_preserve_notes_after_rejection`.

2026-11-01 (always-on-agent N-52) QA impact: a memory retirement filesystem
failure must return an error before deleting the agent definition. Repair the
filesystem and retry the same deletion; it must retire the original notes and
remove only the requested definition. Canonical handler coverage extends
`TestBaseHandlersAgentDefinitionMutations/Should_delete_only_the_effective_workspace_definition_and_disclose_the_global_twin`.

2026-11-01 (always-on-agent D-26) QA impact: call memory inspect/read from an
agent session in Global scope without a project selector. Resolve that profile's
Global definition and memory origin without passing an empty project reference
to the workspace resolver. Owning shared-route regression: `TestAgentMemoryRoutes/Should_read_Global_memory_as_an_authenticated_Global_agent`.

2026-11-01 (always-on-agent N-48) QA impact: a same-name manually started session
without instance lineage must receive a denial from authenticated memory routes
and native memory tools. An instance main or same-agent descendant retains access.
Add a note in a named project and confirm its source is the project name, not its
opaque ID. Read, history, and mutation digests use `sha256:<hex>`; round-trip that
complete value through edits and rollback. Owning suites: `TestAgentMemoryRoutes`,
`TestAgentMemoryNativeTools`, and the real filesystem `TestStore` suite.
