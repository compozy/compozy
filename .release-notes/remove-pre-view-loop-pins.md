---
title: Loop tool pins use only their selected result view
type: fix
breaking: true
---

v0.6.1 removes the one-release compatibility translation for Loop tool pins created before native
tool result views shipped in v0.6.0 (#1432). Tool actions compare their pinned input and selected
output contract directly with the live tool. The automatic full-result fallback, pre-view input
digest translation, and table translating superseded output-schema digests have been removed.

## Migration notes

Unfinished Loop runs started before v0.6.0 can no longer resume actions using pre-view tool pins.
The next affected action fails with `ActionContractStale` (`action_contract_stale`). Start a new
run from the existing definition with `compozy loop run <name>` to pin current tool contracts.
Resuming the old run does not refresh its pins. Review definitions that read full-only fields
and add literal `result_view: full` to those nodes before starting the replacement run.

Definitions, completed runs, and their history are preserved. Runs started with current per-view
pins continue to use their selected contract, including after a daemon restart. No database,
configuration, CLI, HTTP/UDS, native tool ID, or hook shape changes in this removal.
