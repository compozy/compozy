---
id: ET-web-extension-union-install
area: ET
title: Install an extension from the source union in the web
persona: Bruno
journey: J-extension-distribution
expected: The Extensions-only "Install extension" entry point submits the generated `InstallExtensionRequest` union shape (`{source, ref, version?, asset?, allow_unverified?}`) for local_path, github, and git; per-source ref grammar is validated inline before the request; unverified archives route through the daemon's explicit consent dialog (including the daemon's own 422 reason); daemon validation errors are surfaced verbatim; and the curated one-click card install stays unchanged.
entry_points: /marketplace/extensions (Install extension); `POST /api/extensions`
qa_status: pass
bug_ids:
fix_status:
retest_status: untested
fix_commits:
evidence: docs/qa/evidence/2026-11-22-gh-1845-pr-a/screenshots; docs/qa/evidence/2026-11-22-gh-1845-pr-b/screenshots
last_report: docs/qa/reports/2026-11-22-gh-1845-pr-b-extension-dependencies.md
overlaps: ET-web-extensions-manage; ET-extension-published-source-installs
---

Added by ext-improvs Task 08. Cover all three sources against one real daemon: a relative local path
rejected before the request, an absolute path installed after consent, `owner/repo[@tag]` grammar,
and a credential-bearing git URL rejected inline.

2026-08-20 targeted review walk: passed the changed source-guidance contract against a live daemon. The Git URL choice exposed the repository HelpTip on hover with public HTTPS/version guidance, and the modal retained its install controls and explicit consent boundary without browser errors. The request union remains covered by the existing canonical browser/API evidence listed above.

QA impact 2026-07-29: new surface. Never verified against a real daemon.

Visual recapture was explicitly waived by the operator on 2026-07-29. Functional browser coverage
remains the implementation evidence; this scenario stays `untested` for the next QA cycle.

QA impact 2026-08-03: the Git source form now accepts only credential-free HTTPS repository URLs,
routes branch/tag/commit input to Version, and rejects query or fragment syntax inline. Re-walk the
real daemon flow and capture the hint plus each recovery message before completion.

QA 2026-08-04: passed through the normal Marketplace entry point against the isolated daemon. The
dialog exposed local, GitHub, and Git source members; Git kept Version separate and rejected HTTP,
SSH, credentials, query, and fragment syntax inline with specific recovery text. A local unverified
extension required explicit consent, became active, appeared in Installed, and was removed after the
canary. The official bundled `compozy` skill remained visible. Desktop and narrow captures plus every
Git recovery state are linked above.

QA impact 2026-09-28 (ui-normie-pass): Version, Asset and "Allow an unverified package" now sit under a
closed "More options" fold in the install dialog; open it before setting them. Request shape is unchanged.
The description reads "Install from GitHub, a git link, or a folder on this computer." Reset to untested.

QA impact 2026-11-22 (GH-1845 subprocess inputs): inputs bound to the extension subprocess now arrive
in preview `inputs` and in the `extension_inputs_required` refusal's `input_definitions` next to MCP
inputs, and the same input form renders them (secret masked, boolean as a switch). No form change
ships. Walk an Add of an extension that declares one subprocess-only secret and one MCP input: both
appear, Add stays disabled until the required ones are filled, the install succeeds, and the
subprocess input is bound (Installed shows no "Needs setup" notice). Reset to untested.

QA 2026-11-22 (GH-1845 subprocess inputs): the Add of a local extension with one subprocess-only
secret and MCP inputs rendered both masked secrets and a boolean switch; Install stayed disabled
until both required secrets were filled, consent ran, the install succeeded and Installed showed
no Needs setup. Dependency parts are out of scope; source-guidance parts were not re-walked.

QA impact 2026-11-22 (GH-1845 dependencies, experimental): the catalog Add review now lists an
**Also installs** section built from preview `dependencies` (missing and disabled rows; met rows
hidden), each with version, source, ref, trust word ("Not verified" or "Checked by CompozyOS"),
"Turns on" for a disabled dependency, and the gateway digest when one is required. A `version_unmet`
row reads "Update {name} to {min_version} or later first" and Add stays disabled. Add then shows each
dependency's own gate in preview order, titled with its name: the unverified trust dialog for
`consent_required`, then the remote-access digest dialog for a gateway requirement. Canceling any
gate closes the Add and sends no install. Confirming every gate sends one `POST /api/extensions` with
`with_dependencies: true` and `dependencies.<name>` carrying only that dependency's
`expected_digest`, `allow_unverified` and `confirm_gateway_digest`; the dependent's own
`allow_unverified` is unchanged. The success toast names `dependencies_installed`. The installed
rail shows "Missing dependency: {name}" per `unmet_dependencies` entry, with Install (browse), Turn
on or Update (Installed) routing to that dependency. Walk against a real daemon: a kit whose
dependency is a local unverified extension (consent gate, joint install, both listed in Installed),
a cancel at the dependency gate (nothing installed), a dependency installed at a lower version than
`min_version` (Add disabled), then remove the dependency with force and confirm the dependent's rail
notice. The source-union dialog (Install from GitHub, Git URL or local path) does not list
dependencies; a source-union Add with unmet dependencies surfaces the daemon's
`extension_dependencies_required` message. Reset to untested.

QA 2026-11-22 (GH-1845 PR B dependencies, head `08bd19a60`; daemon with `[automation] enabled = true`,
see BUG-20261122-dependency-install-automation-disabled): a local v3 catalog feed served `acme-kit`,
which requires the local unverified `acme-webdep` (gateway permission) and the installed but disabled
`acme-base`. Add showed **Also installs** with `acme-webdep v0.3.0` (Not verified, Local path, Remote
access digest) and `acme-base v1.0.0` (Turns on, Not verified). It hid met rows (01). Add opened the
`acme-webdep` trust gate titled with its name (02). Cancel closed the Add with no
`POST /api/extensions`, and nothing was installed. Confirming the trust gate and the remote-access gate
(03) sent one POST with `with_dependencies: true`, root `allow_unverified: false`, and
`dependencies.acme-webdep` carrying only `expected_digest`, `allow_unverified` and
`confirm_gateway_digest`. The toast read "acme-kit added with acme-webdep, acme-base" (04), and both
dependencies were active. `acme-kit-new` (needs acme-base 2.0.0) showed "Update acme-base to 2.0.0 or
later first" with Add disabled (05). After `remove acme-webdep --force`, the acme-kit rail showed
"Missing dependency: acme-webdep" with Install (06). The source-union local-path Add of `acme-solo`
surfaced "extension: dependencies required: acme-webdep (missing)" (07). The source-guidance parts
were not re-walked.
