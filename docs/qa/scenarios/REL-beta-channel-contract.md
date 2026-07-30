---
id: REL-beta-channel-contract
area: REL
title: Inspect one truthful beta channel contract before publication
persona: Dora
journey: J-approve-compozy-beta-candidate
expected: The README, locally rendered site, official Compozy skill, installer source, update guidance, release workflow, and generated package metadata agree on the hosted v0.3 beta installer, npm @compozy/cli@beta, pinned github.com/compozy/compozy Go install, compozy/compozy release ownership, Sigstore-only provenance, and no Homebrew path before stable, without claiming that any live artifact already exists.
entry_points: README Installation; local packages/site install and runtime installation routes; skills/compozy/SKILL.md; scripts/install.sh; compozy update --check fixture/local contract; .github/workflows/release.yml; package metadata
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status: pending extension SDK publisher replay
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/beta-candidate/beta-origin-contract.txt; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/release-plan/release-plan-contract.txt; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/release-plan/pins.txt;/Users/pedronauck/dev/qa-labs/compozy-qa-gl-rel-wave1-20260730-045845-838751-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: REL-beta-install-paths; REL-beta-installer-provenance; REL-beta-self-update
---

story: As the release administrator, I can inspect one coherent pre-publish channel contract while
every public sentence remains honest that the candidate is not yet live.

Task 12 QA plan: local source/build/fixture evidence only. Task 13 must not call live registries,
install the unpublished version, accept a synthetic Sigstore result, or touch DNS.

QA impact 2026-07-29: the extension SDK publisher now accepts strict unprefixed SemVer containing both
prerelease and build metadata. Re-run the local dry-run contract with a version such as
`1.2.3-rc.1+build.7`; live registry publication remains outside this scenario.
