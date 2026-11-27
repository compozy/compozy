# GH-1845 PR B — experimental extension dependencies

An isolated `eng-qa-bootstrap` lab (targeted profile; CLI, API, Web) walked the changed scenarios for
[#1845](https://github.com/compozy/compozy/issues/1845) PR B on integration head `08bd19a60`.

- **Build:** `go build` of `./cmd/compozy` from `08bd19a60`, stamped
  `-X …/internal/version.Version=v0.7.9`, which is the dependencies floor. Web: `vite` from the same
  checkout (after `bun install`) proxying the lab daemon.
- **Lab:** fresh home `/tmp/compozyqa-7923e2c1da40/runtime`, HTTP 55551, its own UDS, Web 55552, and
  a loopback catalog feed on 55553. Provider usage was off and the lab had no provider. Every CLI call
  went through a wrapper that unsets all `COMPOZY_*` variables and sets only the lab home, port and
  socket. Onboarding was completed through `POST /api/onboarding/complete` so the Web was usable.
- **Deviation:** the walk started with `[automation] enabled = false`, as briefed. Every dependency
  install then failed (BUG-20261122-dependency-install-automation-disabled). The daemon was
  restarted with `[automation] enabled = true` for the rest of the walk. The failure is recorded
  under step 3.
- **Fixtures** (`docs/qa/evidence/2026-11-22-gh-1845-pr-b/fixture/`): one Go SDK subprocess binary
  is shared by every package. All packages set `min_compozy_version = "0.7.9"`.
  - `acme-core` 0.5.1.
  - `acme-base` 1.0.0 with `[gateway] permissions = ["gateway.private"]`.
  - `acme-tool` 1.2.0 requires `acme-core >= 0.4.0` and `acme-base`, all `local_path`.
  - A chain `acme-chain` → `acme-mid` → `acme-leaf`.
  - A diamond `acme-top` → `acme-left` (`acme-core >= 0.4.0`) and `acme-right` (`acme-core >= 0.5.0`).
  - `acme-mkt` requires the plugin-source entry `team/acme-design`, a renamed copy of the
    `open-design` client-plugin testdata in a folder marketplace.
  - The catalog feed (`catalog-v3-extensions.json`) serves `acme-kit` and `acme-kit-new`. `acme-kit`
    requires the local `acme-webdep` (gateway) and `acme-base`. `acme-kit-new` raises the
    `acme-base` floor to 2.0.0.
- **Transcript:** `docs/qa/evidence/2026-11-22-gh-1845-pr-b/cli/transcript.md` holds every CLI and
  HTTP call with timestamp, command, output and exit or HTTP code. The TTY runs are `cli/*.pty`, the
  daemon logs are `cli/daemon*.log`, and the Web POST body is `cli/web-add-post-body.json`.
- **Screenshots:** local under `docs/qa/evidence/2026-11-22-gh-1845-pr-b/screenshots/` (01–07).
  The R2 upload was blocked: `wrangler whoami` reports "You are not authenticated", and no Cloudflare
  token is present.
- **Teardown:** the manifest `TEARDOWN_COMMAND` ran, and `teardown.json` reports `clean: true` with
  no survivors. The daemon survived its graceful stop and the sweep killed it, as in the PR A walk.

| Scenario | Result | Observation |
| --- | --- | --- |
| ET-extension-dependencies | Fail | Steps 1, 2, 4–11 and 13 pass. Step 3 fails with automation disabled (High) and passes with it enabled. Step 12 has the wrong error code (Low). Step 7 suggests an unknown flag (Low). |
| ET-web-extension-union-install | Pass | Dependency part: Also installs, the per-dependency gates, cancel with no POST, the joint install POST shape, the toast, version_unmet with Add disabled, the rail notice, and the source-union refusal. |

## ET-extension-dependencies — per step

| Step | Result | Evidence |
| --- | --- | --- |
| 1 | Pass | Exit 2 `extension_dependencies_required`, with `acme-core` and `acme-base` `missing` and `required_by: ["acme-tool"]`. Nothing was installed. Preview 200 lists both with version, digest, `consent_required: true`, and `acme-base`'s `gateway_requirement_digest`. The preview needs root `allow_unverified: true`; without it, 422 `extension_checksum_unverified`. |
| 2 | Pass | `extension_checksum_unverified`, `dependency: "acme-core"`, exit 78. The human output says `pass --allow-unverified-dependency acme-core`. |
| 3 | **Fail** with automation disabled; Pass with it enabled | Disabled: `dependency installed but not enabled: Internal Server Error`, `dependencies_installed: ["acme-core"]`, and `acme-core` left disabled. A retry gives `automation previewer is required`. Enabled: `extension_gateway_confirmation_required`, `dependency: "acme-base"`, `dependencies_installed: ["acme-core"]`, and `acme-core` stays installed. |
| 4 | Pass | `s4-tty-decline.pty`, `s4-tty-decline-deps.pty`, `s4-tty-accept.pty`. Output: `Dependencies (experimental):` with one line (acme-base 1.0.0, local_path, missing, unverified, gateway digest, required by acme-tool), one question, then `acme-base is not registry-verified…` and `Confirm acme-base gateway requirement …`. `Installed dependencies first: acme-base.` `n` prints `dependency install declined; nothing was installed`, exit 1. |
| 5 | Pass | `remove --global` exits 1 with `acme-tool (workspace global, profile all)` and `--force`. `disable` names profile `default`. The HTTP DELETE returns 409 with `dependents[]`, and the HTTP enablement PUT (`{"profile":"default","enabled":false}`) returns 409. |
| 6 | Pass | `disable --force` succeeds. `status -o json` has `unmet_dependencies[acme-core, state disabled]`, and acme-tool is `enabled: true, state: active`. The WARN `extension dependency unmet … cause: forced` is logged. Human `Unmet Dependencies: acme-core (disabled)`. The toon `unmet_dependencies` column and the `compozy__extensions_info` compact result carry it. |
| 7 | Pass (Low defect) | Enable refuses with exit 2 `extension_dependencies_required`, listing `acme-core` `disabled` with `installed_version`. After enabling acme-core, acme-tool enables. The human hint names `--with-dependencies`, which `enable` lacks (BUG-20261122-enable-dependency-guidance-flag). |
| 8 | Pass | `remove acme-core --global --force` removes only acme-core. acme-tool stays installed and active and reports `acme-core (missing)`. |
| 9 | Pass | With `min_version = "2.0.0"`: exit 2, `Update first: compozy extension update acme-base`, `retry_command`, refused even with `--with-dependencies`. Nothing was installed or updated. |
| 10 | Pass (operator transport) | `compozy tool invoke compozy__extensions_remove {name: acme-base}` returns the tool error `code: extension_dependency_in_use` with `dependents` in `partial_result.structured`. With `force: true` it completes. No agent session ran because the lab has no provider. |
| 11 | Pass | The preview lists `acme-design` `missing`, `source: marketplace`, the listed `digest_sha256` and `registry_tier: unverified`. The custom folder source is unverified, so `consent_required: true`. The CLI `--with-dependencies` install pinned the digest: `archive_digest_sha256` equals the listed digest and `digest_matched: true`. A direct API call without `expected_digest` returns 400 naming the dependency. |
| 12 | Pass (Low defect) | The refusal names `acme-leaf` and `compozy extension update acme-leaf`, and the `other` profile keeps 0.1.0. The code is `extension_install_failed`, not `extension_exists` (BUG-20261122-dependency-exists-code-collapsed). |
| 13 | Pass (partial) | The preview lists the disabled `acme-base` with `installed_version: "1.0.0"` and `registry_tier: "unverified"`. The CLI line reads `disabled, unverified`. The Web shows "Not verified" with "Turns on" (01). Its gateway confirmation still covered the installed version, so no digest was listed. The "confirmation no longer covers it" sub-case needs an update and was not walked. |
| Extra | Pass | The chain preview orders `acme-leaf` (depth 2) before `acme-mid` (depth 1), and the install prints `Installed dependencies first: acme-leaf, acme-mid.` The diamond merges `acme-core` to `min_version 0.5.0`, with `required_by [acme-left, acme-right]`, in topological order. `remove acme-core` lists all three dependents. `remove acme-leaf` names `acme-mid`. |

## Session debrief — Bruno, Web

The catalog Add of `acme-kit` opened a review with **Also installs**. `acme-webdep v0.3.0` showed
Not verified, Local path and the Remote access digest. `acme-base v1.0.0` showed Turns on and Not
verified (01). Add showed "Install acme-webdep?" with the name of the dependent (02). Cancel closed
everything, the network log shows only the preview POST, and nothing was installed. Repeating the
Add and confirming led to "Allow remote access for acme-webdep?" with its fingerprint (03). One
`POST /api/extensions` followed, with `with_dependencies: true`, root `allow_unverified: false`, and
only `dependencies.acme-webdep` carrying `expected_digest`, `allow_unverified: true` and
`confirm_gateway_digest`. The toast read "acme-kit added with acme-webdep, acme-base" (04).
`acme-base` was turned on, and that cleared `acme-tool`'s unmet dependency. For `acme-kit-new`, Add
was disabled under "Update acme-base to 2.0.0 or later first", and the met `acme-webdep` row was
hidden (05). After `remove acme-webdep --global --force`, the acme-kit detail rail showed "Missing
dependency: acme-webdep" with Install (06). Install routes to `/marketplace?q=acme-webdep`. The
source-union "Install extension from a local build…" for `acme-solo` (needs the missing
`acme-webdep`) surfaced "extension: dependencies required: acme-webdep (missing)" after the trust
consent (07).

## Defects

| Bug | Step | Severity | Pre-existing? |
| --- | --- | --- | --- |
| BUG-20261122-dependency-install-automation-disabled | ET-extension-dependencies 3 | High | Trigger is on `origin/main` (`previewExtensionAutomation` nil-previewer error; the #1959 symptom). #1845 makes it block every dependency install. |
| BUG-20261122-dependency-exists-code-collapsed | 12 (and 11's 400 code) | Low | New with #1845 |
| BUG-20261122-enable-dependency-guidance-flag | 7 | Low | New with #1845 |

BUG-20261122-install-enable-500 (#1959) gained the trigger finding: `[automation] enabled = false`.

## Paper cuts

| Where | Felt | Sharpness |
| --- | --- | --- |
| TTY install with `--allow-unverified` | The root's own unverified prompt still appears before the dependency list | Low |
| Human install errors | `acme-tool will:` prints with nothing after it before the error line | Cosmetic |
| Native tool errors | The top-level `message` reads "Internal Server Error" for `extension_dependency_in_use` and `extension_dependencies_required`. The same happens for the pre-existing `extension_inputs_required`, so the masking is shared. The real text is only in `partial_result` | Low |
| Web gateway gate for a new dependency | "Updating acme-webdep lets it use remote access" on a first install | Low |
| Web rail Install for a local-path dependency | Routes to a Marketplace search that cannot find a local package ("No results") | Low |
| Web detail rail | "Verified by CompozyOS" next to "Community · not verified" | Low |
| Web source-union refusal | The dependency error renders inside the unverified-trust dialog, and "Install anyway" stays enabled | Low |
| Daemon log | Every status or list read logs a WARN `extension dependency unmet` per dependent | Low |
| Remove guard wording | A CLI install lists the dependent as "profile all", while a Web install of the same shape says "profile default" | Cosmetic |

## Not verified

- An agent session for step 10 and the agent authority on dependency installs, because the lab has
  no provider.
- The step 11 "without any consent" case for a verified-tier marketplace dependency. The only
  reachable plugin source was a custom (unverified) folder.
- Step 13's gateway-digest-in-preview case for a disabled dependency whose confirmation no longer
  covers it, which needs an update of the dependency.
- The `extension_dependency_graph_invalid` cases (cycle, depth > 5, conflicting source), which are
  not in the scenario.
- The Web "Turn on" and "Update" rail actions. Only Install (missing) was exercised.
- The R2 upload of the screenshots (wrangler unauthenticated).
