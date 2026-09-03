---
id: ET-skill-view-actionable-errors
area: ET
title: Recover from a skill resource or definition error
persona: Ada
journey: J-load-skill-in-managed-session
expected: A native skill_view failure distinguishes a missing resource from a malformed skill definition, preserves the operator-safe path and YAML location across hosted MCP, and gives the agent a specific recovery step while keeping the primary public message stable and safe
entry_points: managed session prompt; compozy__skill_view; hosted MCP tools/call; POST /api/tools/invoke
qa_status: pass
bug_ids:
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-skill-terminal-recovery-20260902-205559-939126-lab/qa-artifacts/qa/test-cases/skill-view-hosted-recovery.md
last_report: docs/qa/reports/2026-09-02-skill-terminal-recovery.md
overlaps: ET-compozy-native-tool-invocation;ET-managed-session-skill-loading
---

Ask a managed agent to read one missing skill resource and one skill whose `SKILL.md` frontmatter is
malformed. The two failures must remain different after the hosted MCP hop: the missing resource is
permanent until the exact relative path is corrected, while the malformed definition identifies the
file and YAML location for the operator and tells the agent to repair the definition before retrying.
Public API output must retain stable reason codes and safe primary messages; operator diagnostics
remain separately marked as details rather than replacing the user-facing message.

After repairing the definition, the same native read must succeed without restarting the daemon.
The managed process must not fall back to the operator CLI or read the file directly.
