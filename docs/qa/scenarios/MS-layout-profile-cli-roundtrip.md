---
id: MS-layout-profile-cli-roundtrip
area: MS
title: Manage saved layouts from the CLI
persona: Bruno
journey: J-administer-window-manager
expected: `compozy layout-profile list|get|put|delete` round-trips a saved layout with `-o json`; `put --file` accepts a `window_layout` spec whose document is version 4, 5 or 6 and stores and returns it as version 6 tabs and splits (floating, minimized and stacked windows of an earlier document become tabs of their own); a list contains global records plus the workspace's own and never another workspace's; `--expected-version 0` creates and a non-zero value replaces or deletes that exact version, failing on a concurrent write; `--scope` accepts only workspace or global; delete refuses to run without `--expected-version`; `compozy layout export` writes a version 6 document that `compozy layout validate --file` accepts and `compozy layout apply --file --revision` reproduces.
entry_points: compozy layout-profile list|get|put|delete; compozy layout export|validate|apply; Settings › Layouts › Saved layouts
qa_status: untested
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-08-01-window-tabs/agent-02-layouts-applies-now.png; docs/qa/reports/2026-08-01-window-tabs.md
last_report: docs/qa/reports/2026-08-01-window-tabs.md
overlaps: MS-configure-window-manager; ET-window-manager-public-parity
---

story: As an agent or an operator at a terminal, I can manage saved layouts without opening the web UI.

qa-impact: 2026-07-24 new surface. The routes already existed on HTTP and UDS; only the CLI verbs are new, so the scope filter and CAS semantics are the daemon's existing ones. Flag only; the next QA cycle owns live testing.

qa-impact: 2026-07-31 v2 layout profiles are rejected under the v3 tab topology contract. Reset to
verify the version boundary and current round trip.

qa-impact: 2026-11-13 window-tabs-split S10. Layout documents are version 6 (desktops →
tabs → splits); versions 4 and 5 are still accepted everywhere a document enters and are converted
permanently, and the daemon rewrites stored v4/v5 profiles to v6 at boot. Walk: preconditions — a
project with a split tab, a second project, and a v5 layout document from a v0.6 build containing
a floating window, a minimized window and a two-window stack, plus a v4 document. 1)
`compozy layout export -o json > now.json`: `version` is 6, each desktop has `tabs` with `root`
leaf/split nodes, and there is no `groups`, `floating` or `minimized` field. 2)
`compozy layout validate --file now.json -o json` is valid. 3) Wrap each of the v6, v5 and v4
documents in a spec (`version: 1`, `id`, `display_name`, `aspect_variant: "any"`,
`overflow_policy: "stack"`, `document`) and run `compozy layout-profile put <id> --file <spec>
--expected-version 0 -o json` for each: all three succeed. 4) `compozy layout-profile get <id> -o json`
for the v5 one: its document is version 6 and the floating, minimized and stacked windows are each a
single-window tab. 5) Repeat `put` with `--expected-version 0` on an existing id: refused; with
the current version: replaced. 6) From the second project, `compozy layout-profile list -o json`
does not list the first project's workspace-scoped records but does list `--scope global` ones. 7)
`compozy layout-profile delete <id>` without `--expected-version` is refused; with it, removed. 8)
`compozy layout apply --file now.json --revision <current> -o json` succeeds and the shell is
unchanged. Reset for the current build.
