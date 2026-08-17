---
id: REL-beta-channel-contract
area: REL
title: Inspect one truthful beta channel contract before publication
persona: Dora
journey: J-approve-compozy-beta-candidate
expected: The README, rendered site, official Compozy skill, installer source, update guidance, release workflow, and generated package metadata agree on per-architecture Electron beta packages, immutable GitHub Release assets, signed `compat.json`, one audited ref-CAS `channel-beta` generation, npm @compozy/cli@beta, pinned github.com/compozy/compozy Go install, Sigstore provenance, and no Homebrew path before stable, without claiming that an unpublished candidate is live.
entry_points: README Installation; packages/site install and desktop routes; skills/compozy/SKILL.md; scripts/install.sh; compozy update --check fixture/local contract; .github/workflows/release.yml desktop build/publish jobs; channel-beta desktop/generation.json and platform manifests; package metadata
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: REL-beta-install-paths; REL-beta-installer-provenance; REL-beta-self-update
---

story: As the release administrator, I can inspect one coherent pre-publish channel contract while
every public sentence remains honest that the candidate is not yet live.

Task 12 QA plan: local source/build/fixture evidence only. Task 13 must not call live registries,
install the unpublished version, accept a synthetic Sigstore result, or touch DNS.

QA impact 2026-07-29: the extension SDK publisher now accepts strict unprefixed SemVer containing both
prerelease and build metadata. Re-run the local dry-run contract with a version such as
`1.2.3-rc.1+build.7`; live registry publication remains outside this scenario.

QA impact 2026-08-16: reset after the beta desktop contract moved to per-architecture Electron
packages, signed `compat.json`, immutable GitHub Release URLs, and a ref-CAS `channel-beta` branch.
Re-walk in the electron-shell QA tail.
