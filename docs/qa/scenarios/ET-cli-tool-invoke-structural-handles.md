---
id: ET-cli-tool-invoke-structural-handles
area: ET
title: Preserve public structural handles in generic CLI tool output
persona: Ada
journey: J-agent-marketplace-parity
expected: `compozy tool invoke <tool_id> -o json` preserves daemon-authored public IDs, digests, and continuation cursors while still redacting sensitive fields and secret-shaped free text.
entry_points: compozy tool invoke <tool_id> -o json; POST /api/tools/:tool_id/invoke over HTTP and UDS
qa_status: blocked-verify
bug_ids: BUG-20260729-tool-invoke-structural-redaction
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: ET-038; ET-api-marketplace-namespace
---

QA impact 2026-07-29: generic CLI tool-result sanitization now preserves public structural handles
through the canonical field-aware redactor. This new public CLI behavior remains untested for the
next QA cycle under the tracker-impact rule; the root-fix replay belongs to the active report and is
not a terminal verdict for this scenario.
