---
id: RT-sdk-node-lifecycle
area: RT
title: Stop only the daemon owned by the Node app
persona: Ana
journey: J-build-external-app
expected: The Node-only start entry uses the selected private home, attaches without spawning when a daemon exists, and close stops only the same daemon that instance started.
entry_points: @compozy/client/node; Compozy.start; close
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

Use a unique lab home and record daemon PID, address, and socket. Start with no daemon, connect, and close with stopDaemon true; confirm only that daemon stops. Start a daemon independently, call start again, and confirm close leaves it alive. Start an owned daemon, replace it through the public lifecycle, and confirm close does not stop the replacement PID. Exercise missing binary and readiness failure and inspect actionable diagnostics without touching the default operator home.

QA impact 2026-10-17: App SDK implementation adds this user-visible journey. Authored planning only;
no live execution verdict is recorded. Execute in the assigned isolated lab and attach current-head
evidence before marking it passed.

QA impact 2026-10-18: distinguish stop acknowledgement from process exit. Exercise a daemon
whose SIGTERM handler delays shutdown; the owning close must wait for that child
to exit within its deadline, with no late writes after the home is released. The existing Node
lifecycle unit suite covers delayed exit and a replacement between ownership and CLI discovery, and TestDaemonE2EAppSDK runs the public
package against a real daemon. Preserve the foreign-PID and reused-client checks.
