# GH-1845 PR B — experimental extension dependencies, re-walk after fixes

This walk repeats the failed and defective steps of
[docs/qa/reports/2026-11-22-gh-1845-pr-b-extension-dependencies.md](2026-11-22-gh-1845-pr-b-extension-dependencies.md)
on integration head `0cb8b750d`. It ran in an isolated `eng-qa-bootstrap` lab (targeted profile; CLI,
API, Web).

- **Build:** `go build` of `./cmd/compozy` from `0cb8b750d`, stamped
  `-X …/internal/version.Version=v0.7.9`. The Web ran under `vite` from the same checkout (after
  `bun install`), proxying the lab daemon.
- **Lab:** a fresh, empty home `/tmp/compozyqa-bac707423a42/runtime`, HTTP 58807, its own UDS, Web
  58808, and a loopback catalog feed on 58809. Provider usage was off and the lab had no provider. A
  wrapper unset every `COMPOZY_*` variable and then set only the lab home, port and socket, and every
  CLI call went through it. **The daemon ran with `[automation] enabled = false` for the whole walk.**
  Onboarding was completed through `POST /api/onboarding/complete`.
- **Fixtures** (`docs/qa/evidence/2026-11-22-gh-1845-pr-b-rewalk/fixture/`): the fixtures from the
  previous walk, with their paths rewritten to the new lab, plus:
  - `acme-plain`: no dependencies (for #1959).
  - `acme-rel`: requires `acme-reldep` with `ref = "../acme-reldep"`.
  - `acme-high`: requires `acme-lowdep >= 9.0.0`, but the `acme-lowdep` source provides 0.1.0.
  - `acme-p12`: requires `acme-p12a` (missing) and then `acme-p12dep` (installed only for profile
    `other`, with source bytes changed).
  - `acme-race1` and `acme-race2`: both require the missing `acme-racedep`.
  - The catalog feed adds `acme-kit-high`, which requires `acme-lowdep >= 9.0.0`.
- **Transcript:** `cli/transcript.md` records every CLI and HTTP call with its timestamp, command,
  output and exit or HTTP code. `cli/low-tty.pty` holds the TTY run, and
  `cli/daemon-automation-disabled.log` is the daemon log.
- **Screenshots:** stored locally under `screenshots/` (01–03). The R2 upload was blocked: `wrangler`
  is not installed (`npx --no-install wrangler` reports a missing package), and no Cloudflare token
  is present.

| Check | Result | Evidence |
| --- | --- | --- |
| #1959 `install <acme-plain> --enable` (automation off) | **Pass** | JSON: `enabled: true, state: active`, no `enable_error`. Human: `Installed acme-plain 0.1.0 and turned it on.`, `State: active`. The daemon log has no `previewer is required` line and no enable error. |
| Step 3 (automation off) | **Pass** | `extension_gateway_confirmation_required`, `dependency: "acme-base"`, `dependencies_installed: ["acme-core"]`, exit 65. `acme-core` is `active`. |
| `--with-dependencies` happy path (automation off) | **Pass** | Adding `--confirm-gateway-requirement acme-base=631b…` gives exit 0, `dependencies_installed: ["acme-base"]`, and acme-tool `enabled: true, active`. A fresh chain `acme-chain` printed `Installed dependencies first: acme-leaf, acme-mid.`, and all three are `active`. |
| Step 11 (automation off) | **Pass** | The preview lists `acme-design` `missing` with the listed digest. The CLI `--with-dependencies` install exits 0, and `acme-design` and `acme-mkt` are `active`. |
| Step 11 error shape | **Pass** | A direct API install without `expected_digest` returns 400 with `issues[{field: "expected_digest"}]` and `dependency: "acme-design"`. It is no longer `extension_install_failed`. |
| Step 12 | **Pass** | The preview gives `acme-p12dep` `state: installed_elsewhere`, `installed_version: 0.1.0`, and `retry_command: compozy extension update acme-p12dep --scope global --profile other`. The install exits 2 with `extension_dependencies_required` and the same top-level `retry_command`. Human output: `Update first: compozy extension update acme-p12dep --scope global --profile other`. `acme-p12a`, which comes first, was **not** installed. The `other` profile keeps 0.1.0 (old checksum `cca97…`). `extension update` accepts `--scope` and `--profile`. |
| Step 7 | **Pass** | Disabled dependency: `Enable first: compozy extension enable acme-core.` Missing dependency: `Install acme-reldep first, or reinstall this extension with compozy extension install <source> --with-dependencies`. Neither suggests `enable --with-dependencies`. |
| Source offers only a version below `min_version` (CLI) | **Pass** | `-o json`: exit 2, `acme-lowdep (version_unmet)`, `version: 0.1.0`. The result is the same with `--with-dependencies` and the consent flags. Human: `The source of acme-lowdep provides 0.1.0; no version satisfies min_version 9.0.0.` with no `--with-dependencies` hint. TTY (`low-tty.pty`): only the root's own unverified question appears, then the refusal, with no dependency consent prompt. Nothing was installed. |
| Same case (Web Add) | **Pass** | `acme-kit-high` review: `acme-lowdep v0.1.0` with "Its source has no version of acme-lowdep at 9.0.0 or later", and Add is disabled (03). |
| Relative `local_path` ref | **Pass** | The preview resolves `../acme-reldep` to `<lab>/fixtures/acme-reldep` (`state: missing`, version 0.1.0). The install was run from `/tmp`, so the CLI cwd is not the base. It exits 0 with `dependencies_installed: ["acme-reldep"]`, and its provenance `source_url` is the fixture folder. |
| Commit-time `extension_exists` keeps its code | **Pass** | Concurrent `install acme-race1` and `install acme-race2`, both needing the missing `acme-racedep`: one succeeded. The other returned `code: "extension_exists"`, `diagnostic.id: extension.exists`, `dependency: "acme-racedep"`. |
| Web Add with dependencies (automation off) | **Pass** | `acme-kit` review: **Also installs** `acme-webdep v0.3.0` (Not verified, Local path, Remote access digest) (01). Then "Install acme-webdep?", then "Allow remote access for acme-webdep?" with its fingerprint. Requests: preview 200 and one `POST /api/extensions` 201. Toast: "acme-kit added with acme-webdep" (02). `acme-kit` and `acme-webdep` are `active`. |

## New defects

| Bug | Severity | Pre-existing? |
| --- | --- | --- |
| BUG-20261122-preview-malformed-manifest-500 | Low | Yes. `internal/api/core/extensions_install_preview.go` is unchanged against `origin/main`. |

A fixture typo, a literal `\n` in `acme-high/extension.toml`, exposed it. `POST
/api/extensions/preview-install` returned a masked `500 Internal Server Error` with no cause in the
daemon log. `extension install -o json` printed a human `error: … decode manifest … toml: line 14`
with exit 1 instead of JSON.

## Paper cuts

| Where | Felt | Sharpness |
| --- | --- | --- |
| Web review, installed dependency whose source also lacks a satisfying version | `acme-kit-high`, in its first form requiring the installed `acme-core >= 9.0.0`, showed "Update acme-core to 9.0.0 or later first". The acme-core source has only 0.5.1, so no update can satisfy it. The CLI step 9 wording is the same | Low |
| `version_unmet` entries | They carry `consent_required: true` even though nothing can be consented to | Cosmetic |
| TTY install with `--allow-unverified` | The root's own unverified question still appears (carried over) | Low |
| Web gateway gate for a new dependency | "Updating acme-webdep lets it use remote access" on a first install (carried over) | Low |
| `compozy marketplace refresh` | Timed out on the UDS client while presets were fetched from GitHub. The sources were refreshed anyway | Low |

## Not re-walked

- Steps 1, 2, 4–6, 8–10 and 13, and the Web parts other than the dependency joint install and the
  unsatisfiable dependency. They passed in the previous walk, and the fixes do not touch them.
- An agent session (step 10), because the lab has no provider.
- The R2 upload, because `wrangler` is unavailable.

## Teardown

The manifest `TEARDOWN_COMMAND` ran. `teardown.json` (copied to the evidence folder) reports
`clean: true` with no survivors, and nothing listens on 58807–58809 any more. The teardown also ended
the agent shell that launched it, because that shell's command line contained the lab path. Its
exit code was 144.

## Second re-walk: steps 9 and 12 on `43f54e4c6`

`43f54e4c6` changed the expected output of steps 9 and 12. A retry command is offered only when it
can satisfy the dependency. Otherwise the CLI and Web give reinstall guidance. This pass also checked
native `compozy__extensions_install` validation payloads.

- **Build:** `go build` of `./cmd/compozy` from `43f54e4c6`, stamped
  `-X …/internal/version.Version=v0.7.9`. The Web ran under `vite` from the same checkout (after
  `bun install`), proxying the lab daemon.
- **Lab:** a fresh, empty `eng-qa-bootstrap` lab (targeted profile; CLI, API, Web): home
  `/tmp/compozyqa-1549c156b51e/runtime`, HTTP 47691, its own UDS, Web 47692, and a loopback catalog
  feed on 47693. `[automation] enabled = false`, provider usage was off, and the lab had no
  provider. Every CLI call went through a wrapper that unset every `COMPOZY_*` variable and then set
  only the lab home, port and socket. Onboarding was completed through
  `POST /api/onboarding/complete`.
- **Fixtures** (`docs/qa/evidence/2026-11-22-gh-1845-pr-b-s9-s12/fixture/`, built by `mkfix.py`):
  - `acme-s9` requires the local `acme-s9base >= 2.0.0`. The installed copy and the source are both
    at 1.0.0.
  - `acme-s9m` and `acme-s9mlow` require the catalog package `acme-mdep` (`source = "curated"`,
    `ref = "compozy/acme-mdep"`) at `>= 2.0.0` and `>= 3.0.0`. `acme-mdep` 1.0.0 was installed from
    the catalog, and 2.0.0 was published later.
  - `acme-p12` and `acme-p12n` require the local `acme-p12dep`, with no bound and with `>= 0.2.0`.
    `acme-p12dep` 0.1.0 was installed only for profile `other`, and its source bytes were changed
    afterwards.
  - `acme-p12m` requires the catalog `acme-mdep2`, installed at 1.0.0 only for `other`. 1.1.0 was
    published later.
  - The catalog kits `acme-kit-s9m`, `acme-kit-s9l`, `acme-kit-p12` and `acme-kit-p12n` mirror these
    cases for Web Add. `acme-kit-rel` is a catalog package with `ref = "../acme-reldep"`. `acme-mkt`
    requires the plugin-source entry `team/acme-design`.
- **Transcript:** `cli/transcript.md` records every call with its timestamp, command, output and exit
  or HTTP code. `cli/daemon.log` is the daemon log.
- **Screenshots:** stored locally under `screenshots/` (01–04). There was no R2 upload.

| Check | Result | Evidence |
| --- | --- | --- |
| Step 9, local-path dependency | **Pass** | The preview gives `acme-s9base` `version_unmet`, `installed_version: 1.0.0`, and no `retry_command`. The install exits 2 with `extension_dependencies_required`, with and without `--with-dependencies`. Human output: `No update of acme-s9base 1.0.0 reaches min_version 2.0.0; reinstall it from a source that provides it first.` `acme-s9base` stays at 1.0.0. |
| Step 9, marketplace-installed dependency | **Pass** | Before the catalog bump (catalog 1.0.0), the output is the same "No update of acme-mdep 1.0.0 reaches min_version 2.0.0" text with no command. After 2.0.0 is published and refreshed, the preview and refusal carry `retry_command: compozy extension update acme-mdep --scope global --profile default`, and the human output is `Run first: compozy extension update acme-mdep --scope global --profile default`. `acme-s9mlow` (`>= 3.0.0`) gets no command. Running the command updates `acme-mdep` to 2.0.0, and then `acme-s9m` installs and is `active`. |
| Step 9, Web Add | **Pass** | `acme-kit-s9m`: "Update acme-mdep to 2.0.0 or later first: run compozy extension update acme-mdep --scope global --profile default" (01). `acme-kit-s9l`: "No update of acme-s9base reaches 2.0.0. Reinstall it from a source with 2.0.0 or later first" (02). Add is disabled in both. |
| Step 12, previews and refusals | **Pass** | `acme-p12`: `installed_elsewhere` with `retry_command: compozy extension install <lab>/fixtures/acme-p12dep --replace --scope global --profile other`, and human `Run first: …`. `acme-p12n`: `installed_elsewhere` with no command, and human `Another scope or profile uses a different copy of acme-p12dep 0.1.0; reinstall it there from a source that provides 0.2.0 or later first.` `acme-p12m`: `retry_command: compozy extension update acme-mdep2 --scope global --profile other`. Each refusal exits 2 with `extension_dependencies_required` before any dependency installs, and `other` keeps its copy. |
| Step 12, Web Add | **Pass** | `acme-kit-p12`: "Another profile or workspace uses a different copy of acme-p12dep. Run compozy extension install … --replace --scope global --profile other first" (03). `acme-kit-p12n`: "… Reinstall it there from a source with 0.2.0 or later first" (04). Add is disabled in both. |
| Step 12, "running it unblocks the root" | **Fail** | Marketplace: after `extension update acme-mdep2 --scope global --profile other`, `acme-p12m` installs and `acme-mdep2` 1.1.0 is attached. Local: the `install … --replace` command succeeds (`other` now has the source checksum `b9042f…`). The preview then calls the dependency `missing`, but `install acme-p12 --with-dependencies --allow-unverified-dependency acme-p12dep` exits 65 with `extension_exists`, `dependency: "acme-p12dep"`. A direct local install into `default` also fails, with `extension_exists`, or with `extension not found` when `--replace` is passed (BUG-20261122-local-dependency-elsewhere-retry-no-unblock). |
| Native install, relative ref from a non-local dependent | **Pass** | `compozy tool invoke compozy__extensions_install {"source":"curated","ref":"compozy/acme-kit-rel","with_dependencies":true}` returns `tool_invalid_input`, `reason_codes: ["extension_validation_failed"]`, and `structured` `{error, issues[{field: "requires.extensions.ref"}], dependency: "acme-reldep"}`. That `structured` payload is identical to the HTTP 400 body for the same request. |
| Native install, missing `expected_digest` | **Pass** | `acme-mkt` with `dependencies.acme-design {allow_unverified: true}` and no digest returns the same tool error with `issues[{field: "expected_digest"}]` and `dependency: "acme-design"`, identical to HTTP 400. Nothing is installed. A `curated` catalog dependency (`acme-s11` → `acme-mdep3`) installs without a digest pin, which is correct: rule 13 covers only `marketplace` plugin sources. |

### New defects

| Bug | Severity | Pre-existing? |
| --- | --- | --- |
| BUG-20261122-local-dependency-elsewhere-retry-no-unblock | Medium | No. The local `install --replace` retry command and the same-bytes attach claim are new in PR B |

### Paper cuts

| Where | Felt | Sharpness |
| --- | --- | --- |
| Local `install --replace` retry command | Run as offered outside a TTY, it fails with "Extension checksum is unverified … The marketplace did not provide a verifiable checksum" because it has no `--allow-unverified --yes`. The message mentions a marketplace for a local folder | Low |
| `compozy profile create other` | Makes the new profile active for every workspace, so the walk had to switch back with `profile use default` | Low |
| Web review after a catalog republish | The first preview after the feed changed answered 409 and the Web retried with 200. After a page reload there was no 409 | Cosmetic |

### Not re-walked

Steps 1–8, 10, 11 and 13. They passed in the earlier walks, and `43f54e4c6` changes only retry
commands, the CLI and Web guidance, and native validation payloads.

### Teardown

The manifest `TEARDOWN_COMMAND` ran from a script whose command line did not name the lab, so the
agent shell survived this time. `teardown.json` (copied to the evidence folder) reports `clean: true`
with no survivors. The daemon survived its graceful stop and the signal sweep ended it. Nothing listens
on 47691–47693 any more, and no lab process remains.

## Third re-walk: step 12 local retry on `f0b712302`

`f0b712302` fixed BUG-20261122-local-dependency-elsewhere-retry-no-unblock. This pass re-walked step
12 (local and marketplace) and, as a quick regression, step 9 (local and marketplace) and the G-9
workspace hint.

- **Build:** `go build` of `./cmd/compozy` from `f0b712302`, stamped
  `-X …/internal/version.Version=v0.7.9`. No Web.
- **Lab:** a fresh, empty `eng-qa-bootstrap` lab (targeted profile; CLI, API): home
  `/tmp/compozyqa-536ba50716bb/runtime`, HTTP 42623, its own UDS, and a loopback catalog feed on
  42625. `[automation] enabled = false`, provider usage was off, and the lab had no provider. Every
  CLI call went through a wrapper that unset every `COMPOZY_*` variable and then set only the lab
  home, port and socket. Onboarding was completed through `POST /api/onboarding/complete`.
- **Terminal runs:** the offered retry commands were typed exactly as shown in a PTY (`script`), with
  a `compozy` on `PATH` that pointed to the lab wrapper. The first attempt used zsh. Its startup
  files put the operator's `~/.compozy/bin` first, so the command reached the operator's v0.7.8
  daemon. That daemon refused the fixture (`min_compozy_version "0.7.9"`) before it changed
  anything (`s12-local-retry-tty-wrong-binary.pty`). The command was then rerun under bash, which
  resolved the lab wrapper.
- **Fixtures** (`docs/qa/evidence/2026-11-22-gh-1845-pr-b-s12-local/fixture/`, built by `mkfix.py`):
  `acme-p12` → local `acme-p12dep`; `acme-p12m` → catalog `acme-mdep2`; `acme-s9` → local
  `acme-s9base >= 2.0.0`; `acme-s9m` → catalog `acme-mdep >= 2.0.0`; `acme-g9` → catalog
  `acme-mdep3 >= 2.0.0`.
- **Transcript:** `cli/transcript.md` records every call with its timestamp, command, output and exit
  or HTTP code. `cli/*.pty` are the terminal runs, and `cli/daemon.log` is the daemon log. It has no
  `ERROR` line.

| Check | Result | Evidence |
| --- | --- | --- |
| Step 12 local, preview and refusal | **Pass** | `acme-p12dep` 0.1.0 is installed for `other` only, and then its source bytes changed. The preview of `acme-p12` gives `installed_elsewhere`, `installed_version: 0.1.0`, digest `084282be…`, and `retry_command: compozy extension install <lab>/fixtures/acme-p12dep --replace --allow-unverified --scope global --profile other`. The `--with-dependencies` install exits 2 with `extension_dependencies_required` and the same top-level `retry_command`. Human output: `Run first: compozy extension install … --replace --allow-unverified --scope global --profile other`. |
| Step 12 local, running the command in a terminal | **Pass** | Typed exactly as shown (`s12-local-retry-tty.pty`), it asks `This extension checksum is unverified. Continue with allow_unverified? [y/N]`. After `y` it exits 0, and `acme-p12dep` is `active`. The `other` copy now has checksum `084282be…`. |
| Step 12 local, root unblocked | **Pass** | The preview now gives `acme-p12dep` `missing`, digest `084282be…`. `install acme-p12 --allow-unverified --with-dependencies --allow-unverified-dependency acme-p12dep --enable --yes` exits 0 with `dependencies_installed: ["acme-p12dep"]`, and `acme-p12` is `enabled: true, active`. `extension list --profile default` and `--profile other` both show `acme-p12dep` 0.1.0 `active`, checksum `084282be…`. In `other` it keeps `installation_profile: other`. |
| Step 12 marketplace | **Pass** | `acme-mdep2` 1.0.0 is installed for `other`, and then 1.1.0 is published. The preview of `acme-p12m` gives `installed_elsewhere` with `retry_command: compozy extension update acme-mdep2 --scope global --profile other`, and the install exits 2 with `Run first: …`. The command, typed exactly in a PTY (`s12-market-update-tty.pty`), updates 1.0.0 to 1.1.0 and exits 0. Then `acme-p12m` installs and is `active`, and `acme-mdep2` 1.1.0 is `active` in `default` and `other`. |
| Step 9 local | **Pass** | `acme-s9base` 1.0.0 is installed. The preview of `acme-s9` gives `version_unmet` with no `retry_command`. The install exits 2 with and without `--with-dependencies`, and the human output is `No update of acme-s9base 1.0.0 reaches min_version 2.0.0; reinstall it from a source that provides it first.` |
| Step 9 marketplace | **Pass** | With only 1.0.0 in the catalog, the output is the same "No update of acme-mdep 1.0.0 reaches min_version 2.0.0" text with no command. After 2.0.0 is published and refreshed, the preview and refusal give `retry_command: compozy extension update acme-mdep --scope global`, and the human output is `Run first: …`. The update was run with an extra `--profile default` and reached 2.0.0. Then `acme-s9m` installs and is `active`. The offered form without `--profile` was run as written in the G-9 case. |
| G-9: dependent in a workspace, dependency global and too old | **Pass** | `acme-mdep3` 1.0.0 is installed with `--scope global`, and then 2.0.0 is published. `install acme-g9 --scope workspace --workspace <lab>/product --with-dependencies` exits 2 with `retry_command: compozy extension update acme-mdep3 --scope global` (no `--workspace`), and the human output is `Run first: …`. Running that command exactly updates the package to 2.0.0. Then `acme-g9` installs into workspace `ws_2e1ee5ae38dd87b1` and is `active`. |

### New defects

None.

### Paper cuts

| Where | Felt | Sharpness |
| --- | --- | --- |
| Local `install --replace --allow-unverified` retry command | In a terminal it still asks to allow the unverified checksum, even though `--allow-unverified` is on the command line. This is the carried-over TTY paper cut from the first re-walk, and the fix deliberately leaves out `--yes` | Low |

### Not re-walked

Steps 1–8, 10, 11 and 13, and the Web. `f0b712302` changes only the dependency attach and reinstall
path, the local retry command text, and the checksum diagnostic copy.

### Teardown

The manifest `TEARDOWN_COMMAND` ran from a `/tmp` script whose command line did not name the lab.
`teardown.json` (copied to the evidence folder) reports `clean: true` with no survivors. The daemon
survived its graceful stop and the signal sweep ended it. Nothing listens on 42623 or 42625 any more,
and no lab process remains.

## Fourth re-walk: Web source-union dependency parity (D12) on `4ce0671ff`

D12 (`a21255f33`) gives the source-union Install dialog the same dependency flow as catalog Add.
This pass walked the "QA impact 2026-11-22 (GH-1845 D16)" steps of
`ET-web-extension-union-install` through the local-path source.

- **Build:** `go build` of `./cmd/compozy` from `4ce0671ff`, stamped
  `-X …/internal/version.Version=v0.7.9`. The Web ran under `vite` from the same checkout (after
  `bun install`) on 50461, proxying the lab daemon, and was started from a script that unset every
  `COMPOZY_*` variable.
- **Lab:** a fresh, empty `eng-qa-bootstrap` lab (targeted profile; CLI, API, Web): home
  `/tmp/compozyqa-b9a1516d4147/runtime`, HTTP 50459, its own UDS. `[automation] enabled = false`,
  provider usage was off, and the lab had no provider. Every CLI call went through a bash wrapper
  that unset every `COMPOZY_*` variable, set only the lab home, port and socket, and ran the lab binary
  by absolute path. Onboarding was completed through `POST /api/onboarding/complete`.
- **Fixtures** (`docs/qa/evidence/2026-11-22-gh-1845-pr-b-d12/fixture/`, built by `mkfix.py`), all
  local and unverified:
  - `acme-solo` requires `acme-webdep` (missing, gateway permission), `acme-base` (installed,
    active) and `acme-off` (installed, disabled).
  - `acme-high` requires `acme-lowdep >= 9.0.0`, and the source has 0.1.0.
  - `acme-s9` requires `acme-s9base >= 2.0.0`, and the installed copy and source are 1.0.0.
  - `acme-p12` requires `acme-p12dep`, installed only for profile `other`, whose source bytes changed
    afterwards.
  - `acme-two` requires `acme-depa` and `acme-depb`.
  - `acme-three` requires `acme-depa` and `acme-depx`, whose subprocess exits at once.
- **Transcript:** `cli/transcript.md` records every CLI and HTTP call. `cli/daemon.log` is the daemon
  log. `cli/*.json` hold the captured install request and response bodies.
- **Screenshots:** stored locally under `screenshots/` and published at
  `https://raw.githubusercontent.com/jillesmc/compozy/pr-evidence/pr-1961/4ce0671/<file>`
  (pr-evidence commit `5597093`).

| Check | Result | Evidence |
| --- | --- | --- |
| Also installs listed, met rows hidden | **Pass** | `acme-solo` review: `acme-webdep v0.3.0` (Not verified, Local path · path, Remote access `631b94e8…`) and `acme-off v1.0.0` (Turns on, Not verified). `acme-base` (met) is hidden (01). |
| Per-dependency gates in order, before root consent | **Pass** | Install hides the form and opens "Install acme-webdep?" ("acme-solo needs acme-webdep, which isn't verified…") (02). Then "Allow remote access for acme-webdep?" with the fingerprint (04). Only then does the root's own "Install <path>/acme-solo?" consent appear (05). |
| Cancel sends nothing | **Pass** | A cancel at the trust gate, and separately at the remote-access gate, closes every dialog (03). The requests show only `preview-install`, with no `POST /api/extensions`. `extension list` is unchanged. |
| Joint install request | **Pass** | One `POST /api/extensions` (201): `with_dependencies: true` and `dependencies: {"acme-webdep": {expected_digest, allow_unverified: true, confirm_gateway_digest}}`. The root's own `allow_unverified: true` and `expected_digest` sit at the top level. `acme-off` (disabled, nothing to consent) gets no entry (`cli/solo-install-request.json`). |
| Toast and Installed | **Pass** | "…/acme-solo installed with acme-webdep, acme-off" (06). The response has `dependencies_installed: ["acme-webdep","acme-off"]`. `acme-solo`, `acme-webdep` and `acme-off` are active (13). |
| Blocked: source too old | **Pass** | `acme-high`: "Its source has no version of acme-lowdep at 9.0.0 or later". Install is disabled (09). |
| Blocked: `version_unmet`, installed copy | **Pass** | `acme-s9`: "No update of acme-s9base reaches 2.0.0. Reinstall it from a source with 2.0.0 or later first". Install is disabled (10). |
| Blocked: `installed_elsewhere` | **Pass** | `acme-p12`: "Another profile or workspace uses a different copy of acme-p12dep. Run compozy extension install …/acme-p12dep --replace --allow-unverified --scope global --profile other first". Install is disabled (11). |
| Partial failure shows "Installed first" | **Pass** (masked message) | `acme-three`: `acme-depa` commits and `acme-depx` fails to initialize. The response is 500 with `dependencies_installed: ["acme-depa"]`, and the dialog shows "Internal Server Error · Installed first: acme-depa" (12). `acme-depa` stays active, and `acme-depx` is rolled back. The message is masked (BUG-20261122-install-activation-failure-masked-500). The error shows in the root consent dialog, because a local root always passes it. |
| Source changed between preview and install | **Pass** (no partial install) | Changing `acme-depb` or the root `acme-two` while a gate was open gives 409 `extension_source_changed` (with `dependency: "acme-depb"` for the dependency) before anything installs (12b). |
| GitHub / Git URL sources | Not walked | No offline GitHub release or HTTPS repository was available in the lab. |

### New defects

| Bug | Severity | Pre-existing? |
| --- | --- | --- |
| BUG-20261122-source-install-stale-preview-after-success | Low | Partly. `main` keeps the preview after success too. D12 adds the stale **Also installs** rows and repeats the dependency gates (07, 08) |
| BUG-20261122-install-activation-failure-masked-500 | Low | Likely. A plain install of the failing package returns the same bare 500 |

### Paper cuts

| Where | Felt | Sharpness |
| --- | --- | --- |
| Review install of a local folder without "Allow an unverified package" | The form shows the raw daemon text "extension: checksum is unverified: acme-solo", and the user has to find the option under More options | Low |
| Reopened dialog | The previous path and the checked "Allow an unverified package" stay in the form, so clicking the checkbox again unchecks it | Cosmetic |
| `compozy extension remove <name>` from an unregistered cwd | It fails on workspace resolution until `--global` is passed | Low |
| Web gateway gate for a new dependency | "Updating acme-webdep lets it use remote access" on a first install (carried over) | Low |

### Teardown

The manifest `TEARDOWN_COMMAND` ran from `/tmp/d12-teardown.sh`. `teardown.json` (copied to the
evidence folder) reports `clean: true` with no survivors. The daemon survived its graceful stop and
the signal sweep ended it, along with the registered `vite` and the lab extension subprocesses.
Nothing listens on 50459 or 50461 any more, and no lab process remains. The script's own command line
did not name the lab, but the tool shell that launched it did, because the command text defined the lab
path. The sweep ended that shell as well (exit 144).
