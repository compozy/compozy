---
id: RT-session-auto-title
area: RT
title: Generate a useful title from the first real task
persona: Bruno
journey: J-17
expected: After the first real user task, the session receives one concise automatic title that survives refresh and is consistent between the workspace list and session topbar.
entry_points: web new session; web session list; web session detail
qa_status: pass
bug_ids: BUG-20260713-background-session-indicator-title;BUG-20260802-automatic-title-persistence-race
fix_status: fixed
retest_status: pass
fix_commits: 8eeb8a38;855b273
evidence: /Users/pedronauck/dev/qa-labs/compozy-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/rt-compozy84-two-workspace-badge-title.dom.txt;/Users/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260802-195112-911343-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-08-02-bundles-removal.md
overlaps:
---

The title must not expose evaluator language, raw ids, or an empty placeholder.

Latency and retry (issue #1028): with `[roles.auto_title]` on a Claude route (for example
`claude-haiku-5-5`), the hidden title child starts input-only, with no hosted MCP and no operator Claude Code
tools, MCP servers, plugins, hooks, `CLAUDE.md`, or auto-memory. Its title turn finishes in a few seconds with
about one thousand prompt tokens, and the user session's own start and turn are not delayed. Point
`roles.auto_title.model` at an unavailable model: each failed attempt records one `role.auto_title.failed` event
per turn (`compozy logs --workspace <ref> --session <id> --type role.auto_title.failed -o json`) and the
session stays "New session". Restore the model with `compozy config set roles.auto_title.model <model>`: the next
turn applies the title. Attempts stop after three failures.

QA 2026-10-29 (isolated lab, Claude + `claude-haiku-5-5`): title child 2.4s end to end (spawn 1.6s, first token
0.5s), 1.4k context tokens, versus 21.8s and 54k tokens before; three concurrent sessions all titled; two
failures recorded attempts 1 and 2 with `retry_on_later_turn=true`, and the third turn applied the title.
