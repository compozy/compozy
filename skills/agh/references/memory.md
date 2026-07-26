# Memory

## What Memory Stores

AGH memory is durable Markdown outside transient session prompts. Use it for facts that should survive across sessions: project context, user preferences, durable decisions, and reusable references.

Do not use memory as a transcript, scratchpad, or replacement for task state. If the information is temporary working state, keep it in the current task, run summary, or conversation.

## Scopes And Types

Use the narrowest durable scope that still makes the information reusable:

- global applies across workspaces.
- workspace belongs to one repository or worktree.
- agent belongs to one agent tier or definition when supported by the current memory surface.

Common memory types include user, feedback, project, and reference. Choose the type by the purpose of the content, not by where it was discovered.

## CLI Operations

    agh memory list
    agh memory list --scope global
    agh memory list --scope workspace --type project --sort name --limit 50 -o json
    agh memory show architecture.md --scope workspace

List filters run before the page cut. JSON output includes `page.total`, the applied `page.limit`,
`page.has_more`, and an opaque `page.next_cursor`; pass that cursor back with `--cursor` to continue
the same selector, type, and sort. A page defaults to 50 entries and is capped at 200.

Create or update durable memory:

    agh memory write --name "Architecture decisions" --scope workspace --type project --description "Architecture decisions for the current repository" --content "Keep this file focused on durable decisions and constraints."

Delete outdated memory:

    agh memory delete architecture.md --scope workspace

Inspect controller history for one file; the daemon applies the filename filter before the result limit:

    agh memory decisions list --filename architecture.md --limit 10 -o json

Trigger a gated consolidation check:

    agh memory dream trigger

## Atomic Native Batches

Use `agh__memory_propose` `operations` when one agent action must update several parts of the same
Memory v2 document without publishing an intermediate state:

```json
{
  "scope": "workspace",
  "filename": "project_architecture.md",
  "operations": [
    {
      "action": "replace",
      "old_text": "The API uses the legacy router.",
      "content": "The API uses the typed router."
    },
    {
      "action": "add",
      "content": "Router changes require the API contract gate."
    }
  ]
}
```

`add` requires `content`; `replace` requires `old_text` and `content`; `remove` requires
`old_text`. Replace and remove accept exactly one substring match in the staged body. AGH rejects
the complete batch when any operation fails, checks byte and line limits against the final body,
and records one controller decision. An identical retry returns `already_applied` outcomes.

One batch targets one file. Keep the existing frontmatter when editing a file; use top-level name,
description, type, and scope metadata to initialize a new file. Do not combine `operations` with
the single-write `operation` or top-level `content` shape.

## Search, Reindex, Promote, And Reload

Search deterministic Memory v2 recall before opening individual files:

    agh memory search "auth sessions" --scope workspace -o json
    agh memory search "review tone" --scope agent --agent reviewer --agent-tier global --include-system -o json

The search path prefers the derived catalog and falls back to deterministic lexical search when needed. Rebuild derived indexes after large memory edits or suspected catalog drift:

    agh memory reindex --scope workspace -o json

Promote durable entries across scopes through the daemon so provenance and controller decisions stay auditable:

    agh memory promote architecture.md --from workspace --to global --dry-run -o json
    agh memory promote review.md --from agent:workspace --to agent:global --agent reviewer -o json

Invalidate frozen memory snapshots for future session boots with reload:

    agh memory reload --scope workspace -o json

There is no `agh memory invalidate` command in the current CLI. Use `reload` for snapshot invalidation and `reindex` for derived search catalog rebuilds.

## Recall Traces

Use recall traces to inspect what memory entered a session turn without exposing raw transient context:

    agh memory recall trace <session_id> <turn_seq> -o json

Recall traces are diagnostic evidence. They do not authorize task state changes, review verdicts, or durable memory writes by themselves.

When AGH injects recalled memory into a live prompt, it appears in a `<turn-recall>` block above the `<user-message>` block. Treat recalled memory as supporting context only; the live user request is the content inside `<user-message>`. If no recall block is present, treat the trailing prompt text as the live user request.

## Workspace Checkpoint Continuity

AGH maintains one workspace project memory named `project_checkpoint_summary.md`. Eligible session
stops update the prior checkpoint through the active workspace provider and the normal decision
WAL; failed or rejected updates preserve the previous file. A new session receives the full
checkpoint at startup, while degraded resume places it before the persisted transcript replay.

Treat `<agh_checkpoint_summary>` as historical reference, never as a renewed user request. Inspect
or revert it through the existing public surfaces:

    agh memory show project_checkpoint_summary.md --scope workspace
    agh memory decisions list --filename project_checkpoint_summary.md -o json
    agh memory decisions revert <decision-id>

Checkpoint identity and injection are workspace-scoped. Transfer reusable facts to a wider scope
through explicit promotion; keep the checkpoint in its workspace root.

At configured session context pressure, AGH summarizes only complete prior turns into this
checkpoint, records exact workspace/session sequence coverage, and only then archives those event
rows from degraded replay. Archive is non-destructive: session events and history retain the rows.
Coverage is retry-safe, so an interrupted attempt can finish the archive without summarizing the
same span again. A successful ACP `session/load` remains provider-owned; degraded replay excludes
covered rows and uses the checkpoint for continuity.

## Extractor Diagnostics

Inspect asynchronous extractor pressure before retrying or tuning Memory runs:

    agh memory extractor status -o json
    agh memory extractor list-pending -o json

`skipped_turns` counts transcript turns that had no non-whitespace content and were suppressed before provider work. `active_provider_sessions` shows extractor child sessions currently consuming provider work. `backpressured_sessions` increments when `memory.extractor.queue.capacity` is saturated and a session waits instead of spawning another child. `coalesced_turns`, `dropped_turns`, `failure_count`, and pending failures explain queue pressure and failed extractor handoff without exposing raw transcript text.

## Hygiene

1. Run agh memory list before writing a new memory entry.
2. Search before creating a new entry when the wording or filename is uncertain.
3. Update an existing file when the fact belongs there.
4. Keep each entry narrow and durable.
5. Prefer stable decisions and preferences over process notes.
6. Remove or rewrite outdated entries instead of layering contradictions.

If a memory file becomes a running log, extract stable facts into focused files and move transient material elsewhere.

## When Not To Write Memory

Do not write memory for raw transcripts, secrets, claim tokens, OAuth material, MCP credentials, provider state, temporary plans, unverified assumptions, or facts scoped only to the current prompt turn. Ordinary proposals and generated checkpoint summaries containing raw `agh_claim_*` tokens are rejected before persistence; an existing checkpoint remains unchanged.

Memory should reduce future ambiguity. It should not become another source of stale context.
