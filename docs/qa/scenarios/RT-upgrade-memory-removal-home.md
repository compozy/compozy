---
id: RT-upgrade-memory-removal-home
area: RT
title: Upgrade a memory-enabled home without a manual step
persona: Dora
journey: J-validate-compozy-hard-cut
expected: A COMPOZY_HOME written by the previous release with memory populated, a Dream run, a `memory.consolidated` trigger, `[memory]` and `[roles.dream]` in config.toml, a SOUL with `memory_policy`, a saved layout with a Knowledge window, and a session compacted by the old pressure compaction starts on the new build with no manual step: retired config tables are archived once into a commented block (a concurrent edit or read-only config only logs a warning and retries on the next load), an agent with memory toolsets starts its session, no memory tables remain, the Markdown memory files are intact, the layout opens without Knowledge, the compacted session shows its full history in history, transcript, search, and anchors, and SOUL validate reports the SOUL valid.
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
  `memory-extractor` session exist, plus the `checkpoint-summary` spawn-role session the old pressure compaction
  created (give one of them a session type other than `dream`, because the migration matches the spawn role);
- an `[[automation.triggers]]` entry and a stored trigger with `event = "memory.consolidated"` (with a run in its
  history);
- `config.toml` carrying `[memory]` (plus `[memory.workspace]`, `[memory.session]`), `[roles.dream]`,
  `[session.compaction]`, and a hook matcher using `compaction_reason`; the same retired tables in one profile
  and one workspace `config.toml`; include a retired trigger whose descendant filter table appears after
  an unrelated role table, both with and without a preceding retained trigger;
- an agent whose `AGENT.md` lists `toolsets: [compozy__memory]` and a tool policy naming a `compozy__memory_*`
  tool, and an extension manifest declaring `provides = ["memory.backend"]`, `requires = ["memory/recall"]`, and
  `memory:read` consent; include an extension automation resource with retired `memory.consolidated`
  and supported `session.stopped` triggers plus a supported job;
- a SOUL.md with `memory_policy` frontmatter, and a `<workspace>/knowledge/` directory;
- a saved desktop layout with a Knowledge window next to a session window and a Settings window on
  `/settings/memory`;
- a session compacted by the old pressure compaction (archived span), and a `ledger.jsonl` session file.

Record `session list`, the compacted session's `history`, `status -o json`, the file hashes, and `config.toml`
bytes; stop the daemon cleanly; back up the home.

**Upgrade (new build, same home).** Start the daemon with no manual step and check:

1. The daemon reaches readiness. Global migration `00130` and session migration `00009` apply once; a second
   restart changes nothing and `PRAGMA foreign_key_check` is empty.
2. Each retired table found in the global, profile, and workspace `config.toml` is removed and appended,
   commented, under `# Archived retired memory and compaction settings; these values are inactive.` with its old
   values intact; `config.retired_keys_archived` is logged once; the second start leaves the file byte-identical.
   Archiving never blocks startup. If a concurrent edit refuses publication, or the file or its directory is
   read-only, startup succeeds using the in-memory overlay with the retired settings inactive, logs the warning
   `config.retired_keys_archive_failed` with `path` and `reason` (a boot loads the config more than once, so
   expect the warning on each refused load, not exactly once), leaves the file bytes untouched, and retries on the
   next load: make the directory writable (or finish the edit), restart, and the archive block then appears with
   `config.retired_keys_archived`.
   The hook declaration that used `compaction_reason` loses that key (archived) and keeps running, observing every
   agent compaction; the same keys in an extension manifest are ignored with a warning. `compozy config set
   memory.enabled true` is still refused (`cli: config path "memory.enabled" is not supported by config set`).
3. The agent with memory toolsets starts its session; `tools.retired_ids_ignored` is logged and the rest of its
   policy applies. The extension loads with the memory entries dropped and `extension.retired_entries_ignored`
   warnings; its automation resource skips the `memory.consolidated` trigger with the same warning (entry
   `memory.consolidated`, the resource file named) while the `session.stopped` trigger and the supported job
   materialize and the extension's other resources load. `AGENT.md` and `SKILL.md` hook declarations ignore the
   retired `compaction_reason` / `compaction_strategy` matcher keys with one `agent.retired_entries_ignored`
   (`agent`, `entries`) or `skills.retired_entries_ignored` warning per owner while retaining supported matchers
   and other declarations, leaving authored files byte-identical (another invalid field in the same matcher is
   still rejected). A Host API
   call to `memory/recall` returns JSON-RPC `-32601`.
4. No memory tables remain and `status` carries no `memory` object (`compozy status -o json | jq 'has("memory")'`
   → `false`, `schema_version` `2026-10-07`, one global entry in `daemon.schema_streams`). The `memory.consolidated`
   trigger is gone while its run history stays; `dream` sessions and the `memory-extractor` and
   `checkpoint-summary` spawn-role sessions are gone, whatever their session type. Their
   retained session directories, metadata, and databases keep identical hashes after boot and repeated
   reconciliation, and no catalog row reappears. Each unsupported session emits one WARN
   `observe.session_recovery_skipped` per observer lifetime with `session_id`, `session_type`, `spawn_role`,
   and `reason` (`unknown_session_type` or `retired_spawn_role`). A normal `user` session directory with valid
   metadata and database but no catalog row, and an orphan whose spawn role is a custom advisory role, are
   recovered into the catalog. This recovery boundary is permanent, not a v0.6.0 shim.
5. The Markdown memory files, `_inbox/` candidates, `knowledge/` directory, and `ledger.jsonl` are present with
   unchanged hashes; nothing reads them.
6. The Web desktop opens the saved layout without the Knowledge window, the session window renders with no
   console error, the Settings window lands on `/settings`, and a reload keeps the layout stable.
7. The previously compacted session shows its full history again (`compozy session history`, no archived span);
   transcript pages (`GET …/transcript`), search, outline, and fork/rewind anchors resolve the restored entries
   with their original identities and tool routes, not only `history` and `events` (session migration `00009`
   restores the transcript projection with the events). Rewind-excluded messages remain excluded; reopening does not advance the
   projection generation again. An assistant completed by a system continuation remains completed even
   when its tool result arrived after that boundary; its final updated sequence stays unchanged across
   repeated opens. Interleaved turns retain their identities and completed text even when a late tool
   result belongs to an entry created before the other turn, or foreign events precede its completing
   boundary. A failed compaction attempt followed by a late tool result and a successful retry also
   preserves the original identity and public transcript sequence across repeated opens. Its old
   `session.compaction_fired` rows remain visible in `compozy session events` as opaque history and
   produce no usage marker.
8. `compozy agent soul validate` reports the SOUL valid with no diagnostic for `memory_policy`, the file is not
   rewritten, and the persona applies in a new session.
9. Roles: `compozy roles list` shows `coordinator` and `auto_title` only; `compozy roles show dream` exits 71 and
   prints `error: Role operation failed` / `role_unknown: dream` (with `-o json`, `diagnostic.code` is
   `role_unknown`; `GET /api/roles/dream` returns 404 with the same diagnostic).

Rollback is a full restore of the pre-upgrade backup; do not run an older binary against the upgraded databases.

Teardown per L-029: run the bootstrap manifest `TEARDOWN_COMMAND` (or `make qa-reap`) on every terminal path and
keep `teardown.json` with `clean: true`. Files may stay for forensics; processes may not.

QA impact 2026-10-07 (memory removal): new in this change; no prior verdict. Automated owners: the global and session
upgrade integration suites, the config archive tests, and the layout reconcile tests; this scenario is the real
run. The shims (config archive, SOUL, tool-ID, and extension ignore filters) are scheduled for removal in v0.6.0
and this scenario retires with them, except the session recovery boundary in step 4, which is permanent: move
its checks into a durable recovery scenario before retiring this one.

Recovery boundary follow-up: `internal/observe/reconcile_test.go` owns real-database repeated-reconcile,
file-preservation and normal-orphan recovery coverage; `internal/daemon/daemon_integration_test.go` owns the
same retained-directory invariant through an actual daemon boot. This automated slice does not replace the
complete previous-release upgrade lab walk above.

Review round 1 automated coverage: the owning session upgrade suite now seeds real previous-version
projected events and uses the actual archive cut before reopening; persistence tests cover noncontiguous
trigger filters and retained-trigger isolation; extension materialization covers mixed retired/supported
resources; the durable layout suite verifies profile/drop/route audit fields after one persistence.
These checks pass individually; the complete previous-release upgrade lab walk remains untested.

Review round 1 addendum automated coverage: config persistence exercises concurrent edit and read-only
publication failures plus retry; daemon boot exercises a real read-only config directory. Agent/skill
loaders retain supported hook matchers while ignoring retired keys; migration coverage includes historical
`checkpoint-summary` rows; replay coverage exercises the exact fitting eight-message tail boundary.
The complete previous-release lab walk is still untested.

Review round 2 automated coverage: `TestSessionDBUnarchiveCompactionSpans` projects the exact seven-event
late-tool ordering on schema 8, uses the actual archive path, and checks completed text, original identities,
tool routing, final sequence, and one-time generation advancement after two opens. This check does not
replace the complete previous-release upgrade lab walk.

Review round 3 automated coverage: the same upgrade suite retains the seven-event case and adds the exact
nine-event interleaving (including the ordinary second-turn chunk and both terminals), plus an eleven-event
ordering where foreign assistant events precede the completing user boundary. Real previous-version
projection and archival are compared with two reopened projections for every identity field, text/state,
route, and generation. The complete previous-release home/Web lab remains untested.

Review round 4 automated coverage: the same real upgrade suite adds the exact eight-event failed-attempt,
late-result, successful-retry ordering. Both fired events are projected before the successful archival;
every original identity and public sequence is compared after two opens, including the surviving trigger
entry. Earlier interleaving, delayed-completion, rewind and reused-route cases remain in the suite.
The complete previous-release home/Web lab remains untested.

CI fix 2 automated layout leg: the single Web Memory removal E2E-008 now reads and writes the raw
clientstate snapshot during an isolated cold restart, then verifies Session visibility, removal of
Knowledge, unchanged floating geometry across reload and no console errors. That browser leg passed;
it does not replace the complete previous-release home upgrade lab above.
