AUDIT REPORT
------------
Claim: <what is being audited (e.g., "Compozy slug auth-refactor task_07 status: completed")>
Compozy slug: <.compozy/tasks/<slug>/ or "n/a">
Command: `<full verification command>`
Executed: <timestamp or relative time>
Exit code: <0 or non-zero>
Output summary: <key pass/fail lines, counts, build result>
Warnings: <none or list>
Errors: <none or list>
Verdict: PASS or FAIL

AUTOMATED COVERAGE
------------------
Support detected: <yes or no>
Harness: <playwright, cypress, webdriverio, generic, or none>
Canonical command: `<full E2E command>` or `none`
Required flows:
  - <flow name>: <existing-e2e | needs-e2e | manual-only | blocked>
  - <flow name>: <existing-e2e | needs-e2e | manual-only | blocked>
Specs added or updated:
  - <spec path>: <why this spec changed>
  - <spec path>: <why this spec changed>
Commands executed:
  - `<command>` | Exit code: <0 or non-zero> | Summary: <key result>
  - `<command>` | Exit code: <0 or non-zero> | Summary: <key result>
Manual-only or blocked:
  - <flow name>: <reason>
  - <flow name>: <reason>

TASK IMPLEMENTATION AUDIT
-------------------------
Plan sources:
  - <task/phase/spec path>
Summary:
  - Tasks audited: <count>
  - PASS: <count>
  - PARTIAL: <count>
  - FAIL: <count>
  - REOPEN: <count>
  - BLOCKED: <count>
  - Fixed during audit: <count>
Results:
  - Task: <task_NN.md path>
    Declared status (frontmatter): <pending | in_progress | completed>
    Audit verdict: <PASS | PARTIAL | FAIL | REOPEN | BLOCKED>
    Techspec deliverable: <section in _techspec.md or "none">
    Implementation evidence: <files, specs, commands>
    Verification evidence: <commands and outcomes>
    Requirement → Test mapping: <covers/weak/missing counts>
    Gaps: <none or missing requirements/checklist items>
    AI audit findings: <none | list of red flags from references/ai-implementation-audit.md with verdict>
    Transcript anomalies: <none | genuine-failure | grader-bug | ambiguous-task | bypass-exploit>
    Action: <none | fixed | frontmatter reverted to <status> | BUG-NNN filed>
Reopened tasks (frontmatter reverted from `completed`):
  - <task_NN.md path>: <reason> | New frontmatter status: <pending | in_progress> | Bug: <BUG-NNN or none>
Memory file written: <.compozy/tasks/<slug>/memory/qa-execution.md or "n/a">
state.yaml: read-only (cy-codex-loop owns mutation via update-state.py)

SUITE HEALTH SNAPSHOT
---------------------
Flaky rate (canonical suite): <X.X%> (threshold: <2%)
Flaky events this run: <count>
  - <test name>: <attempts> attempts, retry outcome: <pass | fail>, category: <async-wait | concurrency | order-dep | external | non-determinism | other>
Mutation score (when harness exists): <X.X% on <module> | "n/a">
Coverage delta vs baseline: <+X.X% | -X.X% | unchanged>
Blocked scenarios: <count>
Manual-only items: <count>
AI audit findings: <count of FAIL/PARTIAL verdicts from references/ai-implementation-audit.md>

QUALITY GATES
-------------
- Flaky rate <2%: PASS | FAIL | N/A
- Zero FAIL from AI test-hygiene audit on P0/P1: PASS | FAIL | N/A
- Zero Critical/High issues open: PASS | FAIL | N/A
- Coverage delta ≥ baseline: PASS | FAIL | N/A
- Zero unresolved flaky-suspect on P0 flows: PASS | FAIL | N/A
Overall: PASS or FAIL (FAIL on any gate blocks unconditional final PASS)

ISSUES FILED
-------------
Total: <number of BUG-*.md files created in audit-output-path/audit/issues/>
By severity:
  - Critical: <count>
  - High: <count>
  - Medium: <count>
  - Low: <count>
Details:
  - <BUG-ID>: <short-title> | Severity: <level> | Priority: <P0-P3> | Status: <pending | resolved | invalid | flaky-suspect | quarantined> | Reopens task: <task_NN.md path or "none"> | Red flag: <RF-1..RF-6 or "n/a">
