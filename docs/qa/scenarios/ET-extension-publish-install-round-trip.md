---
id: ET-extension-publish-install-round-trip
area: ET
title: Publish and consume an extension release without a catalog gatekeeper
persona: Ada
journey: J-extension-distribution
expected: `compozy extension publish` uploads one deterministic archive and its matching SHA-256 sidecar to the selected GitHub release, after which a second isolated Compozy home can install, invoke, update to a behavior-changing release, and remove the extension without any Compozy catalog submission.
entry_points: `compozy extension publish`; `compozy extension install github:owner/repo`; `compozy extension update`; `compozy extension remove`; `compozy__extensions_publish`
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-extension-code-first-authoring; ET-extension-published-source-installs; ET-019; ET-020
---

Added by ext-improvs Task 05. Planning flag only; no QA session ran.

Use a server-side credential binding for the native tool and a process-local `GITHUB_TOKEN` for the
daemon-free CLI verb. The credential must occur in zero bytes of requests, results, errors, events, logs,
or transcripts. Verify the archive bytes hash to the uploaded sidecar, the consumer records only the
integrity fact, and the second release changes the contributed tool's observed behavior after update.
