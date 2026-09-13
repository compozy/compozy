---
id: ET-extension-published-source-installs
area: ET
title: Install extensions from GitHub releases and Git repositories
persona: Ada
journey: J-extension-distribution
expected: A GitHub release shorthand or public HTTPS git URL installs the requested immutable extension in one command with at most one unverified-source consent; matching GitHub sidecars record digest integrity without elevating trust, mismatches leave no installed state, unsafe destinations are rejected before clone, and missing or outdated Git failures identify the required dependency deterministically.
entry_points: `compozy extension install github:owner/repo[@ref]`; `compozy extension install git:<url>[@ref]`; `POST /api/extensions`; `compozy__extensions_install`
qa_status: untested
bug_ids:
fix_status:
retest_status: untested
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

Task04 restart follow-up (final tasks09/10): install a published extension with an explicit
workspace/profile, restart the daemon, and verify its process/tools/logs belong to that exact
workspace and profile. Global and unrelated-workspace views must not expose the instance.
Archive the selected profile and restart again: preserve the attachment and stop its runtime.
For an all-profile workspace attachment, verify the base runtime starts on boot and a named
profile uses a distinct process with a workspace-profile resource ceiling. Link a development
overlay, restart, then unlink it: the published workspace runtime must resume with its original
version and no global publication. Include an already-running named profile during the overlay
transition to exercise teardown and replacement across both instance keys.
Verify the named profile changes to the overlay version and returns to the published version
after unlink. The runtime suite owns injected activation/restoration failures and rollback; the
final daemon walk checks successful transitions, separate processes and correct reported scope.

QA impact 2026-09-13 (marketplace-catalog task03): repeat install and preview over CLI, HTTP/UDS,
and the native tool with global/profile, workspace/all-profile, and workspace/profile selectors.
Read back exact attachment and input ownership, verify a secret never appears in another cell,
and reject cross-workspace/profile agent writes before acquisition. Include the local-path variant
and explicit `COMPOZY_PROFILE` selection. Task09/10 owns the remaining live runtime/restart walks;
focused daemon SQLite/vault and transport tests do not settle this scenario.

Also cover an omitted selector with a workspace default in trusted workspace context, rejection without
that context, explicit global override, and explicit selection for mixed server defaults. Failed default
resolution leaves no managed package or installation row. Existing attachments survive updates.

QA impact 2026-09-13 (marketplace-catalog task03; final walk owned by tasks09/10): repeat the same
published install with identical inputs and verify unchanged package/attachment/input/secret state and
no runtime reload. Reinstall a new package or changed inputs in an existing workspace/profile and
verify other input cells remain unchanged; a publication failure restores the previous package and
credentials. Concurrent catalog aliases of one origin return the same installed result; two origins
claiming one name produce one success and one `409 extension_name_conflict` with `installed_origin`.
A display-name change must not conflict. Only an operator may associate an unclassified installation
with a listing. Exercise CLI, HTTP/UDS and native error details; scoped fixture race checks are slice
evidence, not completion of this live journey.


Task03 update/rollback follow-up (final task10): update a direct GitHub package while the curated
catalog is unavailable. Its source/provenance must remain GitHub. Add a required input: refusal
keeps the installed package unchanged; supplying the input permits update. Dropped declarations
retain inactive values, and re-declaring an input restores its old value. A failed publication after
input commit restores both package and values, and the restored MCP still launches with them.

Task04 package rollback follow-up (final task10): attach one package to multiple workspaces and
profiles. Fail publication of an update that introduces a server; candidate-only runtime names
must disappear in every attachment, while prior allocations/overrides, credentials and another
package remain unchanged. Repeat through published reinstall. Concurrent operations on another
workspace of the same package wait until update/rollback completes; unrelated packages can proceed.
The focused lifecycle suite owns injected storage/publication failure and lock cancellation.

Task04 public addressing follow-up (final task10): with a manual github and extension-owned github
allocated as github.github, Settings GET/auth and native diagnostics accept owner=extension:github
with name github. The same owner with name github.github must return not found. Discovered tools
still execute through the extension's resource/runtime identity and retain its credential owner.
