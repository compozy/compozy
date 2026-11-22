---
id: ET-extension-dependencies
area: ET
title: Install an extension with its dependencies, guard them against removal, and report unmet dependencies
persona: Bruno
journey: J-extension-kit-lifecycle
expected: install of a package declaring [[requires.extensions]] refuses with extension_dependencies_required until consent; a human TTY shows one consolidated preview and each dependency's own trust gates, a non-TTY needs --with-dependencies plus per-name consent; remove and disable of a needed dependency refuse with extension_dependency_in_use listing the dependents until --force; a forced removal never cascades and the dependent reports unmet_dependencies while it keeps running; enabling a dependent with unmet dependencies refuses with extension_dependencies_required.
entry_points: compozy extension install <source> [--with-dependencies] [--allow-unverified-dependency <name>] [--confirm-gateway-requirement <name>=<digest>]; compozy extension remove|disable <name> [--force]; compozy extension status <name>; POST /api/extensions and /api/extensions/preview-install; DELETE /api/extensions/{name}?force=true; PUT /api/extensions/{name}/enablement {force}; compozy__extensions_install|remove|disable
qa_status: untested
bug_ids: []
fix_status: n/a
retest_status: n/a
fix_commits: []
evidence: ""
last_report: ""
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
   stays enabled and running; the daemon log has WARN `extension dependency unmet`.
7. `compozy extension disable acme-tool`, then `compozy extension enable acme-tool`: exits 2 with
   `extension_dependencies_required` listing `acme-core` as `disabled`. Enable `acme-core`, then
   `acme-tool` enables.
8. `compozy extension remove acme-core --global --force`: removes only `acme-core`; `acme-tool` is
   still installed and reports `acme-core` as `missing` in `unmet_dependencies`.
9. Raise `acme-tool`'s `min_version` for `acme-base` above the installed 1.0.0 and reinstall:
   exits 2 with `Update first: compozy extension update acme-base`, even with `--with-dependencies`;
   nothing is updated.
10. From an agent session: `compozy__extensions_remove {name: "acme-base"}` in the agent's
    workspace installation returns the `extension_dependency_in_use` tool error with `dependents`;
    with `force: true` it proceeds.

Pass: no dependency is installed, enabled or updated without consent for that exact artifact;
removal never cascades; the dependent's runtime state never changes because of the guard.

src: .compozy/tasks/extension-inputs-setup-deps/_tests.md (T-16 to T-19)
