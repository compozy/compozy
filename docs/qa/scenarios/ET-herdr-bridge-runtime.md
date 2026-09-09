---
id: ET-herdr-bridge-runtime
area: ET
title: Follow Compozy sessions safely in herdr
persona: Ada
journey: J-extension-distribution
expected: The catalog bridge installs, displays live agent rows, preserves rows through a herdr outage, renders safe message text, and closes only its own completed panes.
entry_points: catalog validation CLI; hook.sh; bridge.py --status; herdr agent list
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-pr-560-herdr-bridge-20260909-151549-489795-lab/qa-artifacts/qa/bridge-runtime.json
last_report: docs/qa/reports/2026-09-09-pr-560-herdr-bridge.md
---

Install the packaged bridge through the catalog validator's production installer. Use isolated
bridge state and send documented session lifecycle hook payloads into `hook.sh`. Confirm the
row through herdr's public pane/agent API and `bridge.py --status`, then end the session and
confirm only its pane closes. Repeat status with herdr unavailable: persisted mappings must
remain. Feed message fragments containing terminal escapes through `colorize.py`; printable
text, newlines, and tabs remain while controls are escaped. A missing workspace resolves the
session owner before the scoped events request; a real workspace never changes scope.

The owning Python suite covers nanosecond ordering, outage handling, safe rendering, private
spool permissions, missing-workspace resolution, cursor continuity, and command diagnostics.

Recovery follow-up: after a complete save, damage the primary map and send another
hook. The last complete recovery copy must preserve pane identity. If no valid copy
exists, queued hooks remain on disk, and maintenance emits a nonzero diagnostic.
Reconciliation telemetry must leave the map lock available to incoming hooks.

Private-state follow-up: invoke maintenance and stdin hooks under umask 022 with an
existing 0755 state directory and 0644 map. The directory must become 0700 and state
files, locks, and logs must be 0600 before writing.

Renderer diagnostics: send malformed JSON followed by a valid message through the
real renderer CLI. It must emit an escaped stderr diagnostic and display the next
message successfully. Closing the output pipe remains a normal shutdown.
