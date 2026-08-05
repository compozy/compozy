---
title: Reversible session archiving and list actions
type: feature
---

A stopped session can now be archived so it leaves the default catalog without deleting anything. History, events, ledger, and the saved runtime choice stay readable, and unarchiving puts the session back exactly as it was — still stopped, so a normal prompt restarts it. Both session lists gained a row menu with state-aware Stop, Archive, Unarchive, and Delete, a delete confirmation, and a separate section for archived sessions. (#309)

- `compozy session archive <id>` and `compozy session unarchive <id>`, plus `compozy session list --archived` for archived only and `--include-archived` for both. Agents get `compozy__session_archive` and `compozy__session_unarchive`, and extensions get `sessions/archive` and `sessions/unarchive` under `session.write`.
- The catalog contract takes `archive=exclude|only|include` and defaults to `exclude`, with exact filtered totals and cursor fingerprints. Archived sessions are excluded from normal metrics.
- Archiving is stopped-only and idempotent. An archived session stays readable, but prompt, attach, and resume are refused until you unarchive it. Hard delete is unchanged, and archive stays catalog metadata rather than a lifecycle state.
- Bridge providers now wait for HTTP route readiness before serving, which closes a startup race across the bundled Discord, Google Chat, GitHub, Linear, Slack, Teams, Telegram, and WhatsApp runtimes.

Migration notes: existing sessions are unarchived, so nothing changes until you archive something.
