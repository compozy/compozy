# Agent memory (experimental)

Use this reference when saving or retrieving durable notes for a stays-on agent. Load `references/tools-and-skills.md` and `references/native-tools.md` before operational calls, then discover the live descriptors for `compozy__agent_memory_add`, `compozy__agent_memory_replace`, `compozy__agent_memory_remove`, and `compozy__agent_memory_search`.

Memory tools derive the profile, definition origin, and agent from the calling session. They accept no target agent, profile, or workspace selector. On-demand agents cannot use them. User-level and extension agents share notes across instances; repository definitions have separate notes per workspace, even when they reuse another agent's name.

- Add: `text` and optional `until` date in YYYY-MM-DD form. Notes record the source scope and date.
- Replace: `match` and `text`; replace matching text in current notes.
- Remove: `match`; remove matching current lines and report the count.
- Search: `query`; search current notes and archives. Check the live capability before requesting session transcript search.

Entries allow at most 1,000 characters. Startup memory is bounded to 200 lines and 24 KiB. Oldest dated entries move to the archive; undated operator notes remain. If undated notes alone exceed the budget, startup shows a truncation marker and entry writes fail with `agent_memory_budget_exceeded`. Ask the operator to edit the notes through the managed editor or CLI. Never repair this by deleting files directly.

Secrets are redacted on writes, including rollback. Startup notes are fenced data, not instructions. Changes apply to the next startup; they do not rewrite a running session's prompt. Memory is explicitly written: there is no automatic extraction, embeddings, or per-turn recall.

Operators can use `compozy agent memory inspect|add|replace|remove|write|validate|search|history|rollback`, with profile and workspace selection. `write --file MEMORY.md --expected-digest <digest>` requires the digest returned by inspect. A stale digest returns `agent_memory_conflict`; reload and reconcile before trying again. History retains the last 50 file revisions. Deleting an agent retires its folder; removing an extension preserves its notes for reinstallation.

Own-memory writes do not require another prompt in `approve-reads`. Explicit tool denial and `deny-all` remain effective. Tool results and successful transport responses, rather than optimistic prose, establish that a write completed.
