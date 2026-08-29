---
id: LP-extension-owned-loop-tool-policy
area: LP
title: Run extension-owned Loop actions under the default tool policy
persona: Bruno
journey: J-loop-extension-actions
expected: A globally installed extension Loop calls tools owned by the same extension while the default external-source policy remains disabled; tools owned by another extension remain unavailable, and normal permission, risk, approval, availability, schema, and drift checks still apply.
entry_points: extension install and enable; resources.loops; Loop run through CLI, HTTP, or UDS; Loop status and task-run failure details
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /home/francisross/dev/qa-labs/compozy-extension-owned-loop-tool-policy-20260829-192832-898784-lab/qa-artifacts/qa/evidence/extension-owner-policy/result.json
last_report: docs/qa/reports/2026-08-29-extension-owned-loop-tool-policy.md
overlaps:
---

story: As an extension author, I can ship a Loop that uses my extension tools without asking operators to enable every external tool source.

Use two enabled test extensions under the default `tools.policy.external_default = "disabled"` setting. The first contributes a Loop and one tool referenced by an action node. The second contributes a different tool. Run the first Loop and prove its own action executes. Then reference the second extension tool from the first Loop and prove resolution remains denied. Restart the daemon, reload the persisted Run snapshot, and repeat the own action to prove the same-owner grant survives hydration without becoming a global policy grant.
