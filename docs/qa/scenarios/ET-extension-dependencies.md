---
id: ET-extension-dependencies
area: ET
title: Install an extension with its dependencies, guard them against removal, and report unmet dependencies
persona: Bruno
journey: J-extension-kit-lifecycle
expected: install of a package declaring [[requires.extensions]] refuses with extension_dependencies_required until consent; a human TTY shows one consolidated preview and each dependency's own trust gates, a non-TTY needs --with-dependencies plus per-name consent; remove and disable of a needed dependency refuse with extension_dependency_in_use listing the dependents until --force; a forced removal never cascades and the dependent reports unmet_dependencies while it keeps running; enabling a dependent with unmet dependencies refuses with extension_dependencies_required.
entry_points: compozy extension install <source> [--with-dependencies] [--allow-unverified-dependency <name>] [--confirm-gateway-requirement <name>=<digest>]; compozy extension remove|disable <name> [--force]; compozy extension status <name>; POST /api/extensions and /api/extensions/preview-install; DELETE /api/extensions/{name}?force=true; PUT /api/extensions/{name}/enablement {force}; compozy__extensions_install|remove|disable
qa_status: pass
bug_ids: [BUG-20261122-dependency-install-automation-disabled, BUG-20261122-dependency-exists-code-collapsed, BUG-20261122-enable-dependency-guidance-flag]
fix_status: fixed
retest_status: pass
fix_commits: [08bd19a60, 0cb8b750d]
evidence: docs/qa/evidence/2026-11-22-gh-1845-pr-b; docs/qa/evidence/2026-11-22-gh-1845-pr-b-rewalk
last_report: docs/qa/reports/2026-11-22-gh-1845-pr-b-extension-dependencies-rewalk.md
overlaps: ET-extension-setup-inputs; ET-web-extension-union-install; ET-extension-profile-enablement
---

Covers US-010 to US-014 (#1845). Dependencies are experimental.

Lab: `eng-qa-bootstrap` isolated lab (unique `COMPOZY_HOME`, ports and socket) built from the
integrated `feat/extension-dependencies` head. Capture every command with timestamp, output and exit
code.

Fixture: local packages with `min_compozy_version` at the dependencies floor:
- `acme-core` 0.5.1, no dependencies;
- `acme-base` 1.0.0, no dependencies, with a `[gateway]` permission so it needs a gateway digest;
- `acme-tool` 1.2.0 requiring `acme-core` (`min_version = "0.4.0"`) and `acme-base`, both
  `source = "local_path"`.

Steps:

1. `compozy extension install <acme-tool dir> --allow-unverified --yes -o json`: exits 2 with
   `extension_dependencies_required` listing `acme-core` and `acme-base` as `missing`, each with
   `required_by: ["acme-tool"]`. Nothing is installed. `POST /api/extensions/preview-install` lists
   both under `dependencies` with versions, digests, `consent_required: true` and the gateway digest
   of `acme-base`, without installing anything.
2. Add `--with-dependencies` only: the refusal is `extension_checksum_unverified` with
   `dependency: "acme-core"` and the message names `--allow-unverified-dependency acme-core`; bare
   `--allow-unverified` covered only `acme-tool`.
3. Add `--allow-unverified-dependency acme-core --allow-unverified-dependency acme-base`: the
   refusal is the gateway confirmation for `acme-base` (`dependency: "acme-base"`) and
   `dependencies_installed: ["acme-core"]`; `acme-core` stays installed.
4. Uninstall nothing; in a real terminal run `compozy extension install <acme-tool dir>
   --allow-unverified`: the CLI prints `Dependencies (experimental):` with one line per missing
   dependency (version, source, state, trust, gateway digest, required by), asks once, then asks
   the `acme-base` unverified consent and its gateway digest. Answering yes installs `acme-base`
   then `acme-tool` and prints `Installed dependencies first: acme-base.` Answering `n` to the first
   question installs nothing and exits non-zero.
5. `compozy extension remove acme-core --global`: exits 1, prints `extension is still required by:
   acme-tool (workspace global, profile all)` and `--force`; `acme-core` stays installed.
   `compozy extension disable acme-core`: same refusal naming profile `default`. The HTTP DELETE
   answers 409 with `dependents[]`.
6. `compozy extension disable acme-core --force`: succeeds. `compozy extension status acme-tool -o json`
   shows `unmet_dependencies: [{"name": "acme-core", "state": "disabled", ...}]` while `acme-tool`
   stays enabled and running; the daemon log has WARN `extension dependency unmet`. Human
   `compozy extension status acme-tool` shows `Unmet Dependencies  acme-core (disabled)`, and
   `compozy extension list -o toon` has the `unmet_dependencies` column; the compact
   `compozy__extensions_info` result carries `unmet_dependencies`.
7. `compozy extension disable acme-tool`, then `compozy extension enable acme-tool`: exits 2 with
   `extension_dependencies_required` listing `acme-core` as `disabled`. Enable `acme-core`, then
   `acme-tool` enables.
8. `compozy extension remove acme-core --global --force`: removes only `acme-core`; `acme-tool` is
   still installed and reports `acme-core` as `missing` in `unmet_dependencies`.
9. Raise `acme-tool`'s `min_version` for `acme-base` above the installed 1.0.0 and reinstall:
   exits 2, even with `--with-dependencies`; nothing is updated. A local-path `acme-base` is not
   marketplace-installed, so the CLI prints `No update of acme-base 1.0.0 reaches min_version ...;
   reinstall it from a source that provides it first.` and no `retry_command`; a marketplace-installed
   one whose release reaches `min_version` prints `Run first: compozy extension update acme-base ...`.
10. From an agent session: `compozy__extensions_remove {name: "acme-base"}` in the agent's
    workspace installation returns the `extension_dependency_in_use` tool error with `dependents`;
    with `force: true` it proceeds.
11. Marketplace dependency: a root requiring a plugin-source package (`source = "marketplace"`,
    `ref = "<source>/<entry>"`) previews it as `missing` with the listed `digest_sha256` and its
    `registry_tier`, without any consent. Installing with `--with-dependencies` pins that digest;
    a direct API call without `dependencies.<name>.expected_digest` refuses naming the dependency.
12. A dependency installed only for another profile, whose source has since published new bytes:
    the preview reports it as `installed_elsewhere` with a `retry_command` scoped to that profile
    (`--scope <scope> --profile <profile>`): `compozy extension update <dep>` for a marketplace-installed
    package whose release is the dependency's version, otherwise
    `compozy extension install <dependency source> [--replace]`. Running it unblocks the root. Installing
    the root with `--with-dependencies` refuses with `extension_dependencies_required` and that
    command before it installs any dependency, and the other profile keeps the installed version. A
    dependency that a concurrent install commits first still refuses with `extension_exists` and
    `dependency`.
13. With a disabled unverified dependency installed, the preview lists it with its
    `installed_version`, `registry_tier: "unverified"` and, if its gateway confirmation no longer
    covers it, its gateway digest; the CLI calls it `unverified` and the Web shows `Not verified`.

Pass: no dependency is installed, enabled or updated without consent for that exact artifact;
removal never cascades; the dependent's runtime state never changes because of the guard.

src: .compozy/tasks/extension-inputs-setup-deps/_tests.md (T-16 to T-19)

QA 2026-11-22 (GH-1845 PR B, head `08bd19a60`, isolated lab, CLI/API/Web): steps 1, 2, 4–11 and 13
pass. Step 3 fails while `[automation] enabled = false`: every dependency install aborts with a
masked 500 (BUG-20261122-dependency-install-automation-disabled, the trigger of #1959). With
automation enabled it passes as written. Step 12 refuses and names the dependency and
`compozy extension update`, but the code is `extension_install_failed`, not `extension_exists`
(BUG-20261122-dependency-exists-code-collapsed). Step 7's human hint names an unknown
`--with-dependencies` flag on `enable` (BUG-20261122-enable-dependency-guidance-flag). Step 10 ran
as the operator through `compozy tool invoke` because the lab has no provider. Step 11 used a custom
folder plugin source, so the dependency was `unverified` and needed `consent_required: true`. The
"without any consent" wording assumes a verified tier. Step 13's gateway-digest sub-case (the
confirmation no longer covers it) was not reachable without an update. A transitive chain
(`acme-chain` → `acme-mid` → `acme-leaf`) and a diamond (`acme-top` → `acme-left`/`acme-right` →
`acme-core`, merged `min_version` 0.5.0, both parents in `required_by`) were also verified.

QA re-walk 2026-11-22 (head `0cb8b750d`, fresh isolated lab, `[automation] enabled = false` throughout,
report `docs/qa/reports/2026-11-22-gh-1845-pr-b-extension-dependencies-rewalk.md`):
- Step 3 passes. The full `--with-dependencies` install ends with acme-tool, acme-core and acme-base
  installed and active, and so do a fresh chain install and the step 11 marketplace dependency.
- Step 7 now prints `Enable first: compozy extension enable acme-core.`, or install guidance for a
  missing dependency, with no flag suggested on `enable`.
- Step 12 previews `installed_elsewhere` with
  `compozy extension update acme-p12dep --scope global --profile other`. It refuses before installing
  the first-listed missing dependency, and the other profile keeps 0.1.0. The step text was updated
  to this contract.
- A dependency-attributed commit collision keeps `extension_exists`. The step 11 missing
  `expected_digest` response is a 400 validation response with `field: expected_digest`.
- A source offering only a version below `min_version` is refused without dependency consent prompts
  or a `--with-dependencies` hint.
- A relative `local_path` ref (`../acme-reldep`) resolves against the dependent's folder.

All four defects are verified. Steps 1, 2, 4–6, 8–10 and 13 were not re-walked.
