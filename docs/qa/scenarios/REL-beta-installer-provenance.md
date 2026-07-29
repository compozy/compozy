---
id: REL-beta-installer-provenance
area: REL
title: Verify the hosted beta installer through Sigstore
persona: Dora
journey: J-evaluate-compozy-beta
expected: `https://compozy.com/install.sh` targets `v0.3.0-beta.2`, downloads the `compozy` archive and checksum bundle from `compozy/compozy`, verifies the release workflow certificate identity and archive checksum, installs `compozy`, and never resolves or falls back to a legacy PEM/SIG contract.
entry_points: https://compozy.com/install.sh; GitHub release v0.3.0-beta.2; checksums.txt; checksums.txt.sigstore.json
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: REL-beta-install-paths
---

QA impact 2026-07-27: Compozy migration Task 10 made the hosted installer a single v0.3 Sigstore
contract with an explicit beta target and the `compozy/compozy` release identity. Planning flag only;
post-publish verification belongs to Task 10's single-cut runbook. Task 13 must not select or
simulate this scenario.

QA impact 2026-07-29: the hosted installer default advanced to the published beta.2 receipt. The
scenario remains `untested`; the next release QA pass owns live archive, provenance, checksum, and
installed-version evidence.
