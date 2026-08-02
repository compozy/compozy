---
id: ET-extension-published-source-installs
area: ET
title: Install extensions from GitHub releases and Git repositories
persona: Ada
journey: J-extension-distribution
expected: A GitHub release shorthand or public HTTPS git URL installs the requested immutable extension in one command with at most one unverified-source consent; matching GitHub sidecars record digest integrity without elevating trust, mismatches leave no installed state, unsafe destinations are rejected before clone, and missing or outdated Git failures identify the required dependency deterministically.
entry_points: `compozy extension install github:owner/repo[@ref]`; `compozy extension install git:<url>[@ref]`; `POST /api/extensions`; `compozy__extensions_install`
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-ext-improvs-final-20260729-230047-267985-lab/qa-artifacts/qa/extension-charters.json;/Users/pedronauck/dev/qa-labs/compozy-go-modernization-closeout-20260804-121411-946266-lab/qa-artifacts/qa/evidence/extensions-closeout.json;/Users/pedronauck/dev/qa-labs/compozy-go-modernization-closeout-20260804-121411-946266-lab/qa-artifacts/qa/evidence/external-extension-blocker.md;/Users/pedronauck/dev/qa-labs/compozy-go-modernization-targeted-f5-f8-20260804-134807-481811-lab/qa-artifacts/qa/evidence/extension-distribution.json
last_report: docs/qa/reports/2026-08-04-go-modernization-closeout.md
overlaps: ET-017; ET-018; ET-023
---

Added by ext-improvs Task 05. Planning flag only; no QA session ran.

Exercise both source members against controlled fixtures. For GitHub, cover an absent sidecar, a matching
sidecar, and a mismatched sidecar. A match sets `digest_matched` but keeps the unverified tier,
`checksum_verified = false`, and the same consent requirement. A mismatch aborts before any registry write.

For git, pin a real fixture tag and verify provenance reports `installed_from = git_url`. Repeat with the
git executable unavailable and require the 503-class structured diagnostic. A nonexistent explicit local
path must name that path and must never fall back to curated or GitHub discovery.

QA impact 2026-08-03: git-source archive production now streams through compressed-byte,
raw-tar-byte, and entry-count budgets and transfers a close/remove temporary-file owner to the
installer. Reset to untested so the real tagged fixture proves the public install path still succeeds
and a bounded failure leaves no install or temporary archive behind; historical evidence is retained.

QA impact 2026-08-03: git-source installs now require credential-free public HTTPS URLs and Git 2.37
or newer. The clone pins validated DNS answers and disables redirects, proxy inheritance, credential
helpers, and repository hooks. Re-walk a public tagged fixture plus HTTP, SSH, embedded-credential,
private-address, mixed-DNS, missing-Git, and Git-2.36 failures across CLI, HTTP/UDS, and the native tool.

QA 2026-08-04: the policy and dependency-failure legs passed across CLI, HTTP, Web, and the native
tool, including the Git 2.36 and missing-Git diagnostics. The success, sidecar, and bounded-cleanup
legs remain `blocked-verify`: read-only GitHub search found no public repository with an installable
extension at its root, while private/local fixtures are correctly denied by the public-network policy.
Verification needs a disposable public HTTPS extension repository with a tagged root manifest.

QA continuation 2026-08-04: passed against the disposable public repository
`compozy/compozy-extension-qa-fixture`. A GitHub release install of `v0.1.0` recorded a matching
archive digest while retaining the unverified trust tier and explicit consent. A pinned public Git URL
reported `installed_from = git_url` and invoked the `v0.1.0` probe. Temporarily replacing the release
sidecar with a zero digest produced `extension_archive_digest_mismatch` before registry or extension
directory mutation. The original sidecar was restored, a fresh daemon installed the release again,
and the final remove left the fixture absent.
