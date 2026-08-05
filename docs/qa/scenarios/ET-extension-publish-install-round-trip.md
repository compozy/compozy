---
id: ET-extension-publish-install-round-trip
area: ET
title: Publish and consume an extension release without a catalog gatekeeper
persona: Ada
journey: J-extension-distribution
expected: `compozy extension publish` uploads one deterministic archive and its matching SHA-256 sidecar to the selected GitHub release, after which a second isolated Compozy home can install, invoke, update to a behavior-changing release, and remove the extension without any Compozy catalog submission.
entry_points: `compozy extension publish`; `compozy extension install github:owner/repo`; `compozy extension update`; `compozy extension remove`; `compozy__extensions_publish`
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-ext-improvs-final-20260729-230047-267985-lab/qa-artifacts/qa/extension-charters.json;/Users/pedronauck/dev/qa-labs/compozy-go-modernization-closeout-20260804-121411-946266-lab/qa-artifacts/qa/evidence/external-extension-blocker.md;/Users/pedronauck/dev/qa-labs/compozy-go-modernization-targeted-f5-f8-20260804-134807-481811-lab/qa-artifacts/qa/evidence/extension-distribution.json
last_report: docs/qa/reports/2026-08-04-go-modernization-closeout.md
overlaps: ET-extension-code-first-authoring; ET-extension-published-source-installs; ET-019; ET-020
---

Added by ext-improvs Task 05. Planning flag only; no QA session ran.

Use a server-side credential binding for the native tool and a process-local `GITHUB_TOKEN` for the
daemon-free CLI verb. The credential must occur in zero bytes of requests, results, errors, events, logs,
or transcripts. Verify the archive bytes hash to the uploaded sidecar, the consumer records only the
integrity fact, and the second release changes the contributed tool's observed behavior after update.

QA impact 2026-08-03: deterministic publish archive generation now applies the same compressed,
raw-tar, and entry-count ceilings used by registry installation. Reset to untested so the real publish
round trip reconfirms archive bytes, sidecar digest, consumption, update, and cleanup; historical
evidence is retained.

QA impact 2026-08-03: GitHub API and upload credentials now remain on their exact trusted origins;
cross-origin redirects are revalidated and stripped of authorization. Re-walk the publish path and a
controlled cross-origin redirect to prove the token reaches only the configured GitHub origin.

QA 2026-08-04: remains `blocked-verify`. The scenario requires external mutation: a disposable
public GitHub repository, two releases, upload authorization, and a controlled redirect origin.
No such target or credential was supplied, and this QA pass did not create public state. The exact
fixture and authorization prerequisites are recorded in the linked blocker evidence.

QA continuation 2026-08-04: passed with `compozy/compozy-extension-qa-fixture`. The product publish
path created deterministic archives and SHA-256 sidecars for `v0.1.0` and behavior-changing
`v0.2.0`. A second isolated Compozy home installed `v0.1.0`, invoked
`qa-fixture-v1:first`, updated to `v0.2.0`, invoked `qa-fixture-v2:second`, and removed the extension.
Release provenance preserved integrity and unverified-trust facts separately. The canonical GitHub
client integration suite remains the owner for a controlled cross-origin redirect because it can
inspect authorization headers at both origins; the real publish also crossed the normal
`api.github.com` to `uploads.github.com` boundary without exposing credentials in public output.
