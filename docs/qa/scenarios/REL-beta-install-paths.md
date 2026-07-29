---
id: REL-beta-install-paths
area: REL
title: Install CompozyOS through a documented beta channel
persona: Dora
journey: J-evaluate-compozy-beta
expected: The hosted installer, `npm install -g @compozy/cli@beta`, and `go install github.com/compozy/compozy@v0.3.0-beta.2` each install the same v0.3 beta binary; the hosted installer opens bootstrap when it has an interactive terminal, npm and Go require `compozy install`, and the README and site offer no Homebrew path before v0.3.0 stable.
entry_points: README Installation; compozy.com install section; compozy.com/runtime/core/getting-started/installation; npm registry; Go module proxy
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
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
