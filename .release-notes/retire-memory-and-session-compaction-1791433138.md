---
title: Remove memory, Dream, and Knowledge; sessions use native agent compaction
type: breaking
---

CompozyOS now centers on local agent sessions, Tasks, Loops, Goals, and explicit Gateway access.
Agent memory, Dream consolidation, the Knowledge app, and the workspace `knowledge/` prompt injector
are removed from all product surfaces with no compatibility aliases. CompozyOS also stops compacting
sessions itself: the agent owns its context window, every rebuild of a session into a new agent
session is bounded, and the agent's native compaction is observed and can be requested.

### Migration

Back up `compozy.db` and the daemon state before upgrading, and export any memory database state you
need (index, decisions, Dream history) with the previous release first. Nothing needs editing by hand
after the upgrade; the daemon starts normally:

- Migration `00128_retire_memory.sql` permanently drops the memory tables and
  `goose_db_version_memory`, deletes `memory.consolidated` automation triggers (run history stays),
  and deletes legacy `dream` and `memory-extractor` sessions. Migration `00009_unarchive_compaction_spans.sql`
  restores history that the removed CompozyOS compaction had archived, so old compacted sessions show
  their full history again.
- `config.toml` is archived on load: `[memory]`, `[roles.dream]`, `[roles.checkpoint_summary]`,
  `[roles.memory_extractor]`, `[roles.memory_controller]`, `[session.compaction]`, `memory.consolidated`
  trigger entries, and the hook-matcher keys `compaction_reason` / `compaction_strategy` move, commented
  and lossless, to the end of the same file under `# Archived retired memory and compaction settings; these values are inactive.`
  The daemon logs `config.retired_keys_archived` once.
- `memory_policy` in `SOUL.md`, retired `compozy__memory*` tool and toolset IDs in tool policies, and
  extension manifest entries `memory.backend`, `memory/*`, and `memory.read|write` are ignored with a
  warning. Saved desktop layouts drop the Knowledge window.
- Markdown memory files, `knowledge/` directories, and `ledger.jsonl` files stay on disk and are no
  longer read. No file is deleted. These ignore and archive rules are removed in v0.6.0.

Port scripts and extensions off `compozy memory`, `/api/memory*`, `compozy__memory_*`, the Host API
`memory/*`, and the `memory.backend` capability; no replacement memory feature is provided. New and
changed surfaces, all experimental: `compozy session compact`, `POST .../sessions/{session_id}/compact`,
`compozy__session_compact`, the Compact now action, the `compaction` history item, the reshaped
`session.compaction_fired` payload and usage markers, and observation-only `context.pre_compact` /
`context.post_compact` hooks with a `compaction_trigger` matcher.

There is no in-place downgrade. Restore a complete pre-upgrade backup before running an older binary.
See the [migration guide](https://compozy.com/docs/migration#memory-removal) for the state disposition
and integration changes.
