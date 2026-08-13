# QA Run Report — 2026-08-13 — PR #372 extension-agent session skills, native CLI

- **Scope:** Fresh targeted evidence for PR #372: extension-published `reviewer` skill resolution and the missing-config reason through public runtime, provider, CLI, HTTP, UDS, hosted MCP, and built Web surfaces.
- **Build:** `compozy v0.3.0-beta.13-23-gc19508ca-dirty` from `c19508ca0a3df92d27f4574c2edc52097decdb36`; SHA-256 `41e81184eabdc24a83425fa661def583740504efed43f2b57d649df66cf4d163`.
- **Dirty diff under test:** `qa/provenance/e2e-test-diff-under-test.patch` under manifest `/Users/pedronauck/dev/qa-labs/compozy-pr372-extension-agent-session-skills-native-cli-20260813-181110-157690-lab/qa-artifacts/qa/bootstrap-manifest.json`.
- **Provider:** real operator-home `codex` native CLI; no secret was copied or configured.
- **Status:** BLOCKED — substantive behavior is evidenced, but the current automatic full lane failed because GoLint exceeded its time budget. The rendered reviewer-session Web route remains unavailable in this build.

## Fresh results

| Scenario | Public evidence | Verdict |
|---|---|---|
| `ET-managed-session-skill-loading` | The real `reviewer` prompt and hosted skill calls returned exactly ten names: built-in `compozy` plus every canonical `dev-cycle` skill. The extension-provenance subset was exactly the nine `dev-cycle` names. | PASS (persona walk) |
| `ET-compozy-native-tool-invocation` | Hosted `compozy__config_get` returned `config_path_not_found` for `loops.inputs.batuta-deliver.auto_commit`. | PASS (persona walk) |
| `ET-session-command-catalog-parity` | CLI, HTTP, and direct UDS compact payloads matched exactly; a foreign workspace returned 404 without commands. Built Web rendered, but its reviewer-session route was not found. | BLOCKED |

The former dated report is invalid as PR #372 evidence: its `36bd8156` build predates this PR head. It is retained only as historical context and is not the current scenario evidence.

## Checks and limitations

- Focused `TestDaemonE2EExtensionPublishedAgentSessionCommandsAndPrompt` with integration tags, CGO, and race detection: PASS (19.062 s).
- Test-convention checker for the changed daemon E2E file: PASS.
- One scoped `./internal/daemon` integration package run: FAIL after 601.519 s in an unrelated existing native-tools/SQLite wait; it was not repeated and is not counted as green.
- `make build` could not resolve Bun's cached `typescript` package for OpenAPI generation; `make build-go` produced the tested binary.
- The first automatic full lane failed only because Bun lacked the installable `typescript` worktree prerequisite. After `rtk bun install --frozen-lockfile` left `package.json` and `bun.lock` unchanged, the next automatic full lane reached GoLint, which printed `0 issues` and then exceeded its 10-minute timeout under machine contention. Neither result identifies a PR #372 product defect.
- A final frozen-tree `make gate` rerun is the remaining validation action. No explicit `make gate-full` command will be run.

## QA tracker impact

PR #372 changes observable extension-agent prompt, catalog, and hosted native-tool resolution. The first two scenarios are passed at the persona-walk level; the QA report remains blocked on C14 until a successful final gate exists. Catalog parity remains `blocked-verify` for its rendered Web evidence. The adjacent extension-marketplace canary was not walked after the reviewer-session Web route proved unavailable and the lab was torn down, so it is an explicit remaining evidence gap.

## Web and configuration impact

- **Web:** The built Web root rendered at the manifest HTTP endpoint, but the reviewer-session route was absent. This is recorded as a limitation rather than attributed to PR #372; it blocks rendered-menu parity only.
- **Configuration lifecycle:** No configuration key changed. Public `compozy__config_get` now has fresh evidence for the existing `config_path_not_found` result. The lab set `defaults.provider=codex` sequentially in its isolated `COMPOZY_HOME`, restarted the daemon as instructed by the command response, and preserved the operator's `HOME` and native login.

## Compozy Impact Audit

- **Native tools:** No ToolID, descriptor, schema, digest, or capability gate changed. The daemon E2E now proves exact catalog parity for `compozy__skill_list`, empty `compozy__skill_search`, and all command-ID `compozy__skill_view` reads, plus the existing missing-path reason assertion for `compozy__config_get`.
- **Extensibility and hooks:** The changed E2E owner is extension publication through the daemon session resolver: prompt augmentation, command projection, and hosted skill calls must expose the same complete catalog. No extension manifest, hook, registry, bridge SDK, sidecar, or configuration lifecycle behavior changed.
- **Workspace data isolation:** The fresh HTTP and direct-UDS reads rejected the reviewer session under foreign workspace `ws_5058192ea7eace90` with 404 and no commands. No ownership model or store schema changed.
- **Official Compozy skill:** No `skills/compozy/` change. The built-in `compozy` skill is deliberately included in the exact ten-item public catalog, while extension provenance remains exactly the nine `dev-cycle` entries.

## Teardown and audit

- The literal manifest teardown ran before this report was written. `qa/teardown.json` records `clean: true` with no survivors.
- The first strict audit failed because its journey log and final verification report had not yet been populated. After those records were filled from real commands, the script returned exit 0 because it found an existing verification log. That is not a passing QA verdict: the project contract requires a successful final gate, and the current automatic full lane timed out in GoLint. The last source-tree action is a frozen-tree `make gate` rerun; no status will change afterward unless it produces a current successful full record.
