---
id: ET-agent-plugin-dev-reload
area: ET
title: Reload a dev-linked Agent Plugins package
persona: Bruno
journey: J-extension-dev-lifecycle
expected: A portable package dev-links without install trust, publishes its mapped skills and MCP servers, reloads changed package content through the existing workspace-scoped generation loop, retains its instance data, and leaves other workspaces and any published instance unchanged.
entry_points: compozy extension dev <path>|reload <name>|status <name> --workspace|remove <name>; POST /api/extensions/dev and POST /api/extensions/:name/reload over HTTP and UDS; compozy__extensions_dev|reload|info|remove
qa_status: fail
bug_ids: BUG-20260914-agent-plugin-dev-rejected
fix_status: pending
retest_status: pending
fix_commits:
evidence: docs/qa/reports/2026-08-16-agent-plugins.md#session-debriefs
last_report: docs/qa/reports/2026-08-16-agent-plugins.md
overlaps: ET-extension-dev-reload-loop; ET-agent-plugin-data-removal
---

QA impact 2026-08-16: the established dev overlay lifecycle now accepts the Agent Plugins format.
Task 08 owns only the portable mapping and reload parity; the existing scenario remains canonical for
generic generation, last-good, logs, and workspace-isolation behavior.

QA 2026-08-16: the dev-linked portable package published its resources without install trust, reload
advanced only its workspace generation, retained instance data, and left the published instance and
the second workspace unchanged.

PR636 review retest: exercise standard, Claude, Codex and Cursor manifests with package names
that differ from their directory names. Inspection and development loading must agree on the
manifest identity, layout and diagnostics, and PLUGIN_DATA must use the authored package name.
The same securely acquired manifest bytes feed classification and loading; unsupported versions
and no-follow path restrictions remain enforced. Focused manifest tests passed; live reload re-walk pending.

The hook re-walk also installs a hook-only package in one workspace/profile and checks that it
executes exactly once there, never in another profile/workspace. Add a global attachment, link a
development generation, then reload a generation without hooks: the workspace override must suppress
the inherited hook in both cases. Restart, unlink, detach and disable the profile, checking the
applicable hook output at each transition. The owning real SQLite/subprocess integration passed;
this public scenario extension still requires its final live re-walk.
