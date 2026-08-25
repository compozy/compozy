# QA Run Report — 2026-08-25 — skill-sources

- **Scope:** Skill sources program (spec `.compozy/tasks/skill-sources`, tasks 01–07): four-layer `skills.sources` / `skills.custom_sources` config with live apply, eight-tier discovery with first-level symlink following, `skill_exposures` expose/unexpose lifecycle, provider-aware injection suppression, origin attribution across every public projection, web S1–S3 (Settings sources section, composer picker origin labels, marketplace expose panel), site docs + official skill.
- **Cadence tier:** targeted (five in-scope journeys + one adjacent canary), per `task_08.md`
- **Build:** `80f17b536` (`compozy v0.3.0-beta.20-16-g80f17b536`, built from `_worktrees/skills-source`) · **Environment:** isolated QA lab `compozy-devtool-oss-launch-20260825-165802-509267-lab`, daemon `http://127.0.0.1:55384`, UDS `…/compozyqa-dd797be7e54f/runtime/compozyd.sock`
- **Started:** 2026-08-25T16:58:59Z · **Status:** in-progress <!-- in-progress | closed -->

### Lab and parity notes

- Bootstrap manifest: `~/dev/qa-labs/compozy-devtool-oss-launch-20260825-165802-509267-lab/qa-artifacts/qa/bootstrap-manifest.json` (`reused_lab: false`, `health: fresh`, playbook `devtool-oss-launch`, browser mode `browser-use`, no blocker).
- Isolation: non-default `COMPOZY_HOME`, HTTP port 55384, lab-scoped UDS and `tmux-bridge` sockets, lab provider homes. `COMPOZY_WEB_API_PROXY_TARGET` derived from the manifest — never hardcoded. Five other worktrees are live on this machine, so machine-wide reaping is forbidden for this run; teardown uses the manifest `TEARDOWN_COMMAND` only.
- Every lab command runs against `_worktrees/skills-source/bin/compozy`, never the operator's installed binary.

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Dora — Runtime Administrator | Power User | desktop / wifi-fast / en-US | CH-skill-sources-live-apply, CH-skill-expose-lifecycle-trust, CH-skill-sources-diagnostics-truth, CH-skill-sources-settings-web |
| Bruno — Delivery Builder | Power User | desktop / wifi-fast / en-US | CH-skill-expose-web-repair, CH-skill-sources-repo-teammate |
| Théo — Returning Session User | Power User | desktop / wifi-fast / en-US | CH-skill-session-suppression-matrix |
| Ada — Autonomous Agent | Power User (native-tool) | desktop / wifi-fast / en-US | CH-skill-sources-agent-plane, CH-skill-sources-managed-session-canary |
| Mateo Rivera — Helix CLI founder (playbook operator persona) | — | desktop / wifi-fast / en-US | real-scenario kickoff (`devtool-oss-launch`) |

## Flows in Scope

- `J-absorb-skills-from-other-tools` — a machine's existing cross-tool skill folders become Compozy skills without copying anything (`../journeys/J-absorb-skills-from-other-tools.md`)
- `J-diagnose-skill-sources` — when a skill is missing, the product itself explains why (`../journeys/J-diagnose-skill-sources.md`)
- `J-share-skills-with-other-tools` — a Compozy skill becomes visible to other tools through an owned symlink, reversibly (`../journeys/J-share-skills-with-other-tools.md`)
- `J-use-absorbed-skills-in-a-session` — absorbed skills are choosable and invocable in a session without being loaded twice (`../journeys/J-use-absorbed-skills-in-a-session.md`)
- `J-operate-skill-sources-headless` — the whole feature is operable with no screen (`../journeys/J-operate-skill-sources-headless.md`)
- `J-load-skill-in-managed-session` — **adjacent canary**: an agent still reaches a skill the prompt omitted (`../journeys/J-load-skill-in-managed-session.md`)
- `J-use-session-slash-commands` — ride-along: the `/` catalog and composer chip (`../journeys/J-use-session-slash-commands.md`)
- `J-marketplace-acquisition` — ride-along: installed card + detail changed by this diff (`../journeys/J-marketplace-acquisition.md`)
- `J-validate-compozy-hard-cut` — ride-along: native tool descriptors and the official skill's references changed (`../journeys/J-validate-compozy-hard-cut.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-skill-sources-live-apply | J-absorb-skills-from-other-tools / ET-manage-skill-source-policy | Dora | Interrupt | Pending | | |
| 2 | CH-skill-sources-live-apply | J-absorb-skills-from-other-tools / ET-live-skill-source-reload | Dora | Interrupt | Pending | | |
| 3 | CH-skill-sources-live-apply | J-absorb-skills-from-other-tools / ET-skill-origin-attribution | Dora | Interrupt | Pending | | |
| 4 | CH-skill-expose-lifecycle-trust | J-share-skills-with-other-tools / ET-skill-exposure-lifecycle | Dora | Interrupt | Pending | | |
| 5 | CH-skill-session-suppression-matrix | J-use-absorbed-skills-in-a-session / ET-skill-session-source-injection | Théo | Multi-Tab | Pending | | |
| 6 | CH-skill-session-suppression-matrix | J-use-session-slash-commands / ET-session-command-catalog-parity | Théo | Multi-Tab | Pending | | |
| 7 | CH-skill-session-suppression-matrix | J-use-session-slash-commands / ET-session-composer-skill-chip | Théo | Multi-Tab | Pending | | |
| 8 | CH-skill-sources-diagnostics-truth | J-diagnose-skill-sources / ET-skill-source-diagnostics-cli | Dora | Garbage | Pending | | |
| 9 | CH-skill-sources-diagnostics-truth | J-diagnose-skill-sources / ET-skill-source-symlink-containment | Dora | Garbage | Pending | | |
| 10 | CH-skill-sources-diagnostics-truth | J-diagnose-skill-sources / ET-skill-ecosystem-frontmatter-quiet | Dora | Garbage | Pending | | |
| 11 | CH-skill-sources-settings-web | J-absorb-skills-from-other-tools / ET-web-skill-sources-settings | Dora | Garbage | Pending | | |
| 12 | CH-skill-expose-web-repair | J-share-skills-with-other-tools / ET-web-skill-expose-panel | Bruno | Back-Button | Pending | | |
| 13 | CH-skill-expose-web-repair | J-marketplace-acquisition / ET-web-marketplace-installed-management | Bruno | Back-Button | Pending | | |
| 14 | CH-skill-expose-web-repair | J-marketplace-acquisition / ET-web-marketplace-skill-install | Bruno | Back-Button | Pending | | |
| 15 | CH-skill-sources-agent-plane | J-operate-skill-sources-headless / ET-skill-source-agent-parity | Ada | Feature | Pending | | |
| 16 | CH-skill-sources-agent-plane | J-operate-skill-sources-headless / ET-skill-source-observe-ledger | Ada | Feature | Pending | | |
| 17 | CH-skill-sources-agent-plane | J-validate-compozy-hard-cut / ET-compozy-native-tool-invocation | Ada | Feature | Pending | | |
| 18 | CH-skill-sources-agent-plane | J-validate-compozy-hard-cut / ET-compozy-official-skill-discovery | Ada | Feature | Pending | | |
| 19 | CH-skill-sources-repo-teammate | J-absorb-skills-from-other-tools / ET-workspace-skill-source-teammate | Bruno | Feature | Pending | | |
| 20 | CH-skill-sources-managed-session-canary | J-load-skill-in-managed-session / ET-managed-session-skill-loading | Ada | Feature | Pending | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

### Real-scenario runtime lane (`eng-real-scenario-qa`, playbook `devtool-oss-launch`)

Observation of the Helix CLI v1.0 release-week runtime that hosts these walks. One in-persona operator
kickoff, then read-only observation; no agent under test is prompted again.

| Lane | Check | Status | Evidence |
|---|---|---|---|
| R1 | Single in-persona kickoff posted and confirmed | Pending | |
| R2 | Task activation released behind the scheduler barrier (11 declared tasks) | Pending | |
| R3 | Observation window completes without an unexplained stall | Pending | |
| R4 | Required deliverables produced by the runtime (8 types) | Pending | |
| R5 | Required collaboration (12 peer messages, 3 review cycles, 1 disagreement, 4 channels) | Pending | |
| R6 | Strict evidence audit (`audit-qa-evidence.py --strict`) | Pending | |

### Automated lanes

| Lane | Command | Status | Result |
|---|---|---|---|
| E1 | `make test-e2e-runtime` | Pending | |
| E2 | `make test-e2e-web` | Pending | |
| E3 | `make gate-full` (single workstream close) | Pending | |

## Session Debriefs

<!-- One block per charter run, written within 5 minutes of the box ending. Charter files stay untouched. -->

### Probe 0 — the documented-versus-shipped config question (Ada, headless, pre-session)

Ordered first because a wrong answer here poisons everything an agent builds on it. Run entirely
through public surfaces: `compozy tool invoke`, `compozy config`, the HTTP route, and the UDS socket.

- **Ran:** 2026-08-25T16:59Z → 17:20Z
- **The answer:** the shipped reference is wrong and the runtime is right. `compozy__config_set` and
  `compozy__config_unset` write both keys at **user** and **workspace** scope — `applied: true`,
  `lifecycle: live`, `next_action: none`, with an apply record and a bumped generation. **Agent** and
  **profile** scope refuse, but with `tool_denied` / `config_scope_not_allowed`.
  `config_trust_root_forbidden` is never emitted for either key at any scope. That matches
  `_spec.md:445` exactly, so the documentation was the defect
  (→ BUG-20260825-skill-source-agent-write-doc-mismatch, fixed).
- **Second finding, unplanned:** the *operator* half of the same documented sentence was broken.
  `compozy config set|unset --scope profile --profile <name>` on either source key failed with
  `decode skills settings request: unknown field "override"`, making the profile layer of the
  four-layer contract unreachable from every transport
  (→ BUG-20260825-skill-source-profile-write-rejected, fixed).
- **Dismissed with evidence (not a bug):** a profile-scope PATCH first reported
  `restart_required: true`. Cause is correct lifecycle classification — that body also introduced
  `poll_interval` into a profile layer that had none, and `poll_interval` is restart-required. A
  repeat, and a body changing only `sources`, both reported no restart.
- **Dismissed with evidence (not a bug):** `compozy config get skills.sources` returned `["agents"]`
  while the user layer held `["agents","claude"]`. Cause is correct four-layer resolution —
  `profile create helix` had made `helix` current and the helix layer set `["agents"]`. Explicit
  `--scope user` returned the user value; HTTP and UDS agreed.
- **Carried forward to charter 1/7:** `compozy skill sources` with no flags reports `scope: user`
  even while a named profile is current and that profile's layer narrows the source list. Whether the
  default view should follow the exact profile is a truthfulness question worth a deliberate walk,
  not a snap verdict.
- **Paper cuts:** the profile-write failure surfaced a raw decoder message (`unknown field
  "override"`) naming a field the operator never typed — recorded below; it disappears with the fix.
- **Bugs filed/updated:** BUG-20260825-skill-source-agent-write-doc-mismatch (fixed),
  BUG-20260825-skill-source-profile-write-rejected (fixed)
- **Scenarios settled:** none yet — both scenarios this probe touches
  (`ET-skill-source-agent-parity`, `ET-compozy-official-skill-discovery`) belong to charter 7 and are
  settled there after their full walk.

### Lab setup note — a blocking discovery outside this cycle's scope

Standing up the runtime lane surfaced BUG-20260825-workspace-agent-unusable-for-sessions: an agent
created with `compozy agent create --workspace <ws>` is listed by `agent list`, fully readable
through `agent info` with a resolved provider and model, and then refused by `session new` with
`agent not available in workspace`. It is not a skill-sources defect and it fails the fix-loop
governor (core session-resolution path, and the correct behavior is a product decision), so it is
escalated in Decisions for a Human. The runtime lane proceeds with the eight playbook agents
registered at the global layer, which resolves normally.

## What Was Fixed

### BUG-20260825-skill-source-profile-write-rejected: Setting a skill source for one profile fails with an internal decoder error

- **Symptom:** `compozy config set|unset --scope profile --profile <name> skills.sources` (and
  `skills.custom_sources`) exits 65 with `decode skills settings request: unknown field "override"`.
  User and workspace scope work; another skills key at profile scope works.
- **Root cause:** the shared API decoder, not the CLI. `internal/settings.updateSkillsSection`
  already accepts a `SkillSourcesOverride` at `ScopeProfile` and the CLI already sends it — the
  presence-aware override is what gives `unset` its clear-and-inherit semantics. But
  `decodeSettingsSkillsUpdate` offered the override shape only when `scope == ScopeWorkspace`, so
  profile bodies fell through to the config-only branch and were rejected. HTTP and UDS both.
- **Fix:** `07ab1d985` — one logical fix: offer the override shape to the exact-profile lane,
  workspace stays override-only, user stays config-only.
- **Regression test:** `internal/api/core/settings_test.go`,
  `TestUpdateSettingsSkillsSourcePolicyShapes` — three profile-scope override subtests (set,
  `null`-clear, empty), plus pins for the config body and the forbidden-field refusal at profile
  scope. Failed before with the exact production message; passes after.
- **Retested:** rebuilt the binary, restarted the lab daemon, re-walked as Dora — set both keys at
  profile scope (live, applied), unset (deleted, live), and a fresh `config get` returned the
  inherited user value, proving the key cleared rather than blanked.

### BUG-20260825-skill-source-agent-write-doc-mismatch: The official skill tells agents a config write is denied that actually succeeds

- **Symptom:** `skills/compozy/references/configuration.md` and `references/tools-and-skills.md` both
  told agents `compozy__config_set` denies the two source keys with `config_trust_root_forbidden`.
  It does not — it writes them.
- **Root cause:** documentation, not code. Both paths sit in `agentMutableConfigKinds` and
  `ClassifyToolConfigPath` returns on that lookup before the trust-root branch. The keys *are* listed
  in `skillsConfigPathIsTrustRoot`, which is the unreachable branch the original sentence was written
  from.
- **Fix:** `2d63c7fe4` — both sentences corrected to the observed policy.
- **Regression test:** documented replay (prose has no red-before test) — the Probe 0 transcript is
  the before/after evidence. A guard was added to the canonical suite
  `internal/config/tool_surface_test.go` pinning both paths as agent-mutable string slices so the
  classifier cannot drift back and re-falsify the corrected text. The missing doc-vs-runtime gate is
  recorded at `docs/qa/automation-backlog/official-skill-doc-runtime-agreement.md`.
- **Retested:** re-read both references against the recorded probe matrix — no remaining claim the
  runtime contradicts; `config_trust_root_forbidden` no longer appears for either key anywhere in
  `skills/`.

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|
| Dora | J-absorb-skills-from-other-tools, set a source for one profile | "It's telling me a field called `override` is unknown. I never typed that. What am I supposed to do with this?" | sharp | fixed (`07ab1d985`) — the path now succeeds, so the message is unreachable |

## Runtime Errors Observed

_None yet beyond the filed bugs._

## Human Verifications Needed

_None recorded yet._

## Decisions for a Human

### An agent the catalog lists cannot start a session (BUG-20260825-workspace-agent-unusable-for-sessions)

- **What's broken:** `compozy agent create <name> --workspace <ws>` reports success and the agent is
  then listed by `agent list` and fully readable through `agent info` — with a resolved provider and
  model — but `compozy session new --agent <name> --workspace <ws>` refuses it with `agent not
  available in workspace`. Every read surface advertises an agent the one verb that uses it rejects.
  Evidence and the isolating control matrix are in the bug file; reproduced on a lab daemon after
  `compozy install`, under two different profiles.
- **Why not auto-fixed:** fails three governor bounds. The fault is in core session agent resolution
  (`resolveWorkspaceAgent` over `ResolvedWorkspace.Agents`, `internal/session/manager_workspace.go:206-224`)
  versus where `agent create --workspace` writes (`<ws>/.compozy/profiles/<profile>/agents/`), so the
  blast radius reaches every session start; the root cause is suspected but not confirmed by a fix;
  and the correct behavior is a genuine product choice.
- **Options:**
  1. Surface workspace-profile agents to session resolution — makes the catalog honest and matches
     what `agent create --workspace` already implies, but widens what a session may bind to and needs
     a decision about profile isolation between sessions.
  2. Make `agent create --workspace` write to a layer sessions already reach — smaller change, but it
     contradicts ADR-017's profile-aware resource scope for agent definitions.
  3. Keep both behaviors and make the catalog truthful — mark unusable agents in `agent list`/`info`
     and refuse `agent create --workspace` with a clear reason. Cheapest, but leaves the capability
     missing rather than fixing it.
- **Recommendation:** option 1. The product already lets an operator create a workspace agent and
  already advertises it as ready; the honest reading is that session resolution is missing a layer,
  not that the operator did something unsupported. Option 3 should ship regardless as the guard —
  whatever the layer rules end up being, a listed agent must never be one `session new` rejects.
- **Out of this cycle's scope:** this is an agent-definition/profile-layer defect, not a skill-sources
  one. It is recorded here because it was found by this run and it blocked the runtime lane until the
  playbook agents were re-registered at the global layer.

## Learnings

_Not run yet._

## Final Status

_Written last, after the exit gate._
