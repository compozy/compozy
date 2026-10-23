---
id: REL-beta-channel-contract
area: REL
title: Inspect one truthful beta channel contract before publication
persona: Dora
journey: J-approve-compozy-beta-candidate
expected: The README, rendered site, official Compozy skill, installer source, update guidance, release workflow, and generated package metadata agree on per-architecture Electron packages, immutable GitHub Release assets, signed `compat.json` whose `min_app_version` is the oldest app live on any reached channel, one audited ref-CAS generation on `channel-stable` mirrored to `channel-beta` for each stable release, npm `@compozy/cli` on `latest`, Homebrew `compozy/compozy/compozy`, pinned github.com/compozy/compozy Go install, and Sigstore provenance, without claiming that an unpublished candidate is live.
entry_points: README Installation; packages/site install and desktop routes; skills/compozy/SKILL.md; scripts/install.sh; compozy update --check fixture/local contract; .github/workflows/release.yml desktop compatibility/build/publish jobs; channel-stable and channel-beta desktop/generation.json and platform manifests; package metadata
qa_status: untested
bug_ids: BUG-20260817-explicit-release-web-assets-sync; BUG-20260817-macos-finalized-dmg-integrity-stale; BUG-20260817-macos-channel-manifest-includes-dmg; BUG-20260817-linux-channel-manifest-includes-deb
fix_status: fixed
retest_status: pending
fix_commits: 800fbce; 7846f4b; bd5ecef; 361c785e; 437cea3d
evidence: docs/qa/reports/2026-08-17-electron-shell.md
last_report: docs/qa/reports/2026-08-17-electron-shell.md
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

QA impact 2026-10-23: v0.4.0 introduces the stable desktop channel. `compozy-desktop-release channels`
maps a stable release to `channel-stable` plus a `channel-beta` mirror, `min-app-version` keeps the
oldest live beta app served, and the in-app updater offers a newer stable release to beta installs.
npm `latest` and Homebrew carry the stable release; the npm `beta` dist-tag is not moved by CI
(trusted publishing cannot change dist-tags). Re-walk before publication.
