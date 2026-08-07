---
id: REL-beta-installer-provenance
area: REL
title: Verify the hosted beta installer through Sigstore
persona: Dora
journey: J-evaluate-compozy-beta
expected: `https://compozy.com/install.sh` serves a script pinned to the latest published release tag, downloads the `compozy` archive and checksum bundle from `compozy/compozy`, uses only compatible local Cosign v3 or the pinned Cosign v3.1.3 verifier, verifies the release workflow identity and archive checksum without provenance fallback, and installs the matching binary. Each GitHub release also carries an `install.sh` asset pinned to its own tag.
entry_points: https://compozy.com/install.sh; latest GitHub release; checksums.txt; checksums.txt.sigstore.json
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-release-cosign-v3-20260807-170358-478309-lab/qa-artifacts/qa/evidence/installer-live-beta5.txt; /Users/pedronauck/dev/qa-labs/compozy-release-cosign-v3-20260807-170358-478309-lab/qa-artifacts/qa/evidence/cosign-real-bundles.txt
last_report: docs/qa/reports/2026-08-07-release-cosign-v3.md
overlaps: REL-beta-install-paths
---

QA impact 2026-07-27: Compozy migration Task 10 made the hosted installer a single v0.3 Sigstore
contract with an explicit beta target and the `compozy/compozy` release identity. Planning flag only;
post-publish verification belongs to Task 10's single-cut runbook. Task 13 must not select or
simulate this scenario.

QA impact 2026-07-29: the hosted installer default advanced to the published beta.2 receipt. The
scenario remains `untested`; the next release QA pass owns live archive, provenance, checksum, and
installed-version evidence.

QA impact 2026-08-06: the hosted installer became a rendered route. The committed source is now
`scripts/install.template.sh` with a `__COMPOZY_PINNED_VERSION__` placeholder; the site route
injects the latest published release tag and goreleaser injects each release's own tag into the
attached `install.sh` asset. The Sigstore chain, pinned cosign verifier, and explicit-tag posture
are unchanged and locked by `public-install-contract.test.ts` (template safety, TS/shell render
parity, route headers/body) plus `make installer-check`. Live provenance evidence against a
published release stays deferred to the post-publication acceptance pass.

QA impact 2026-08-07: the installer verifier moved to Cosign v3.1.3. Compatible local v3 binaries
remain usable; older or unreadable local versions trigger the pinned bootstrap, while a real
signature or checksum rejection remains terminal. This scenario was reset for a live beta.5 walk.

QA verdict 2026-08-07: the candidate installer downloaded the pinned Cosign v3.1.3 verifier,
accepted the real beta.5 release identity and bundle, verified the archive checksum, installed the
binary, and independently reported `compozy 0.3.0-beta.5`. Invalid version input left no binary.
