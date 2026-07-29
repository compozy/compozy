---
id: ET-extension-published-source-installs
area: ET
title: Install extensions from GitHub releases and Git repositories
persona: Ada
journey: J-extension-distribution
expected: A GitHub release shorthand or git URL installs the requested immutable extension in one command with at most one unverified-source consent; matching GitHub sidecars record digest integrity without elevating trust, mismatches leave no installed state, and git-unavailable failures identify the missing binary deterministically.
entry_points: `compozy extension install github:owner/repo[@ref]`; `compozy extension install git:<url>[@ref]`; `POST /api/extensions`; `compozy__extensions_install`
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-017; ET-018; ET-023
---

Added by ext-improvs Task 05. Planning flag only; no QA session ran.

Exercise both source members against controlled fixtures. For GitHub, cover an absent sidecar, a matching
sidecar, and a mismatched sidecar. A match sets `digest_matched` but keeps the unverified tier,
`checksum_verified = false`, and the same consent requirement. A mismatch aborts before any registry write.

For git, pin a real fixture tag and verify provenance reports `installed_from = git_url`. Repeat with the
git executable unavailable and require the 503-class structured diagnostic. A nonexistent explicit local
path must name that path and must never fall back to curated or GitHub discovery.
