---
id: RT-upgrade-memory-removal-home
area: RT
title: Upgrade a memory-enabled home without a manual step
persona: Dora
journey: J-validate-compozy-hard-cut
expected: A COMPOZY_HOME written by the previous release with memory populated, a Dream run, a `memory.consolidated` trigger, `[memory]` and `[roles.dream]` in config.toml, a SOUL with `memory_policy`, a saved layout with a Knowledge window, and a session compacted by the old pressure compaction starts on the new build with no manual step: retired config tables are archived once into a commented block, an agent with memory toolsets starts its session, no memory tables remain, the Markdown memory files are intact, the layout opens without Knowledge, the compacted session shows its full history, and SOUL validate reports the SOUL valid.
entry_points: previous-release compozy binary on a lab COMPOZY_HOME, then the new build on the same home; compozy daemon start; config.toml; compozy status -o json; compozy automation triggers -o json; compozy session list|history; compozy agent soul validate; compozy extension list -o json; Web desktop root with the saved layout
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-retired-product-surfaces-absent; RT-pressure-context-compaction; RT-preserve-shared-schema-isolation; RT-authored-context-lifecycle; RT-session-lineage-upgrade
---

Compatibility regime: user state, lossless upgrade with no manual step. Retired memory surfaces must be dropped,
archived, or ignored without blocking daemon start.

**Setup (previous release).** Build the previous release (v0.3.0, commit c46be43c7) into the lab with
`git archive`, boot it on an isolated `COMPOZY_HOME` with the acpmock provider, and create:

- memory enabled and populated: Markdown memory files at profile, workspace, and agent scope, plus `_inbox/`
  candidates, with their SHA-256 recorded;
- one Dream run (`compozy memory dream trigger`) and one extractor run, so Dream history and a `dream` and a
  `memory-extractor` session exist;
- an `[[automation.triggers]]` entry and a stored trigger with `event = "memory.consolidated"` (with a run in its
  history);
- `config.toml` carrying `[memory]` (plus `[memory.workspace]`, `[memory.session]`), `[roles.dream]`,
  `[session.compaction]`, and a hook matcher using `compaction_reason`; the same retired tables in one profile
  and one workspace `config.toml`;
- an agent whose `AGENT.md` lists `toolsets: [compozy__memory]` and a tool policy naming a `compozy__memory_*`
  tool, and an extension manifest declaring `provides = ["memory.backend"]`, `requires = ["memory/recall"]`, and
  `memory:read` consent;
- a SOUL.md with `memory_policy` frontmatter, and a `<workspace>/knowledge/` directory;
- a saved desktop layout with a Knowledge window next to a session window and a Settings window on
  `/settings/memory`;
- a session compacted by the old pressure compaction (archived span), and a `ledger.jsonl` session file.

Record `session list`, the compacted session's `history`, `status -o json`, the file hashes, and `config.toml`
bytes; stop the daemon cleanly; back up the home.

**Upgrade (new build, same home).** Start the daemon with no manual step and check:

1. The daemon reaches readiness. Global migration `00128` and session migration `00009` apply once; a second
   restart changes nothing and `PRAGMA foreign_key_check` is empty.
2. Each retired table found in the global, profile, and workspace `config.toml` is removed and appended,
   commented, under `# Archived retired memory and compaction settings; these values are inactive.` with its old
   values intact; `config.retired_keys_archived` is logged once; the second start leaves the file byte-identical.
   The hook declaration that used `compaction_reason` loses that key (archived) and keeps running, observing every
   agent compaction; the same keys in an extension manifest are ignored with a warning. `compozy config set
   memory.enabled true` is still refused (`cli: config path "memory.enabled" is not supported by config set`).
3. The agent with memory toolsets starts its session; `tools.retired_ids_ignored` is logged and the rest of its
   policy applies. The extension loads with the memory entries dropped and one `extension.retired_entries_ignored`
   warning; a Host API call to `memory/recall` returns JSON-RPC `-32601`.
4. No memory tables remain and `status` carries no `memory` object (`compozy status -o json | jq 'has("memory")'`
   → `false`, `schema_version` `2026-10-07`, one global entry in `schema_streams`). The `memory.consolidated`
   trigger is gone while its run history stays; the `dream` and `memory-extractor` sessions are gone. Their
   retained session directories, metadata, and databases keep identical hashes after boot and repeated
   reconciliation, and no catalog row reappears. Each unsupported session emits one WARN
   `observe.session_recovery_skipped` per observer lifetime with `session_id`, `session_type`, `spawn_role`,
   and `reason`. A normal `user` session directory with valid metadata and database but no catalog row is
   recovered into the catalog.
5. The Markdown memory files, `_inbox/` candidates, `knowledge/` directory, and `ledger.jsonl` are present with
   unchanged hashes; nothing reads them.
6. The Web desktop opens the saved layout without the Knowledge window, the session window renders with no
   console error, the Settings window lands on `/settings`, and a reload keeps the layout stable.
7. The previously compacted session shows its full history again (`compozy session history`, no archived span);
   its old `session.compaction_fired` rows remain visible in `compozy session events` as opaque history and
   produce no usage marker.
8. `compozy agent soul validate` reports the SOUL valid with no diagnostic for `memory_policy`, the file is not
   rewritten, and the persona applies in a new session.
9. Roles: `compozy roles list` shows `coordinator` and `auto_title` only; `roles show dream` fails with
   `role_unknown`.

Rollback is a full restore of the pre-upgrade backup; do not run an older binary against the upgraded databases.

Teardown per L-029: run the bootstrap manifest `TEARDOWN_COMMAND` (or `make qa-reap`) on every terminal path and
keep `teardown.json` with `clean: true`. Files may stay for forensics; processes may not.

QA impact 2026-10-07 (memory removal): new in this change; no prior verdict. Automated owners: the global and session
upgrade integration suites, the config archive tests, and the layout reconcile tests; this scenario is the real
run. The shims (config archive, SOUL, tool-ID, and extension ignore filters) are scheduled for removal in v0.6.0
and this scenario retires with them.

Recovery boundary follow-up: `internal/observe/reconcile_test.go` owns real-database repeated-reconcile,
file-preservation and normal-orphan recovery coverage; `internal/daemon/daemon_integration_test.go` owns the
same retained-directory invariant through an actual daemon boot. This automated slice does not replace the
complete previous-release upgrade lab walk above.
