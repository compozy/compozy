---
id: REL-beta-install-paths
area: REL
title: Install CompozyOS through a documented beta channel
persona: Maia
journey: J-evaluate-compozy-beta
expected: The hosted installer, `npm install -g @compozy/cli@beta`, and `go install github.com/compozy/compozy@v0.3.0-beta.1` each install the same v0.3 beta binary; the README and site offer no Homebrew path before v0.3.0 stable.
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
channels and removed Homebrew from every shipped beta install surface. Planning flag only; the next
QA cycle owns live registry and install acceptance.

QA impact 2026-07-27: Task 11 rewrote the landing and launch post around the same hosted-installer,
npm `beta`, and pinned Go version contract. The scenario remains `untested`; no release channel was
published or retested in this content task.
