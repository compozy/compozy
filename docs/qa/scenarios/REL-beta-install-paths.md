---
id: REL-beta-install-paths
area: REL
title: Install CompozyOS through a documented beta channel
persona: Dora
journey: J-evaluate-compozy-beta
expected: The hosted installer, `npm install -g @compozy/cli@beta`, and the pinned `go install` command from the latest release notes each install the same latest published v0.3 beta binary; `https://compozy.com/install.sh` and the site install surfaces resolve that tag dynamically (no hand-maintained version literal anywhere); the hosted installer opens bootstrap when it has an interactive terminal, npm and Go require `compozy install`, and the README and site offer no Homebrew path before v0.3.0 stable.
entry_points: README Installation; compozy.com install section; compozy.com/docs/getting-started/installation; npm registry; Go module proxy
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report: docs/qa/reports/2026-07-29-site-improvs-deep-review.md
overlaps: REL-beta-installer-provenance; REL-beta-self-update
---

QA impact 2026-07-27: Compozy migration Task 10 moved public distribution to the explicit v0.3 beta
channels and removed Homebrew from every shipped beta install surface. Planning flag only; Task 10's
post-publish single-cut runbook owns live registry and install acceptance.

QA impact 2026-07-27: Task 11 rewrote the landing and launch post around the same hosted-installer,
npm `beta`, and pinned Go version contract. The scenario remains `untested`; no release channel was
published or retested in this content task.

Task 12 boundary: post-publish backlog only. Task 13 must not select or simulate this scenario.

QA impact 2026-07-29: the published beta.2 receipt now owns the living installer/docs default. Verify
all three channels report beta.2, each method reaches an explicit bootstrapped state without
repeating the hosted installer wizard, and the first session infers the current repository.

QA impact 2026-08-06: install surfaces moved from a hand-maintained pinned literal to dynamic
resolution. `compozy.com/install.sh` is now a site route that bakes the latest published release
tag into the served script (5-minute revalidation, committed fallback tag when GitHub is
unreachable); the installation page and landing render the pinned commands from the same resolver;
README and both migration guides link the latest release for the exact `go install` tag instead of
carrying a number. Local evidence: `public-install-contract.test.ts` (route headers/body, render
parity, dry-run of the rendered script, anti-literal guard) and `make installer-check`. Live
three-channel parity against the published latest release stays deferred to the post-publication
acceptance pass.

QA impact 2026-08-13: beta.15 exposed an npm package before its referenced GitHub archives were
public, so a successful registry publish still installed with HTTP 404. The release workflow now
requires public GitHub assets and real macOS/Linux package installation before npm publication.
Reset to `untested` until the repaired beta.15 install and the complete beta.16 flow are walked.
