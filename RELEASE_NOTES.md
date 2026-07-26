## 0.0.9 - 2026-07-04

### ♻️ Refactoring

- Tasks orchestration (#269)

### 🐛 Bug Fixes

- Network optimizations (#263)
- Native tools (#266)
- Align release-gated network integration tests with #263 behavior

### 📚 Documentation

- Update skills
- New skill
- Add feature stories
- Add v0.0.9 release notes

### 📦 Build System

- Update go deps
- Gitignore

### Release Notes

#### Features

##### Network delivery policies, subscriptions, and thread-to-task promotion

AGH network channels gain durable delivery control. Each channel now carries a fan-out policy — deliver to all members, route to a designated coordinator peer, or match by capability (the default) — managed with `agh network channels create` and `agh network channels update`. Peers and tasks can subscribe to channels: inspect subscriptions with `agh network subscriptions` and manage per-task subscriptions with `agh task subscribe <task-id>`. Executable thread messages can be promoted into durable workspace tasks straight from the CLI with `agh network promote`, or by an agent through the `agh__task_promote_from_thread` native tool, and designated sibling task runs can be fanned out as part of a workflow. Network and task views now surface task links, peer and coordination cost, and delivered size and token metrics, while mentions are normalized (trimmed, empties removed) and size and token limits are enforced as non-negative.

##### Task blocking, recovery, and needs-attention triage

Tasks are now first-class to block, triage, and recover. A task can be explicitly blocked and unblocked with `agh task block <id>` and `agh task unblock <id>`, and every block is kept as durable history you can inspect with `agh task blocks <id>`. A new `needs_attention` status surfaces tasks that stalled and need a human or coordinator to step in — task details now carry the blocked reason and the wake creator so it is clear why a task is waiting and who nudged it. Clear the state with `agh task recover <id>`, recover a specific stalled run with `agh task run recover <run-id>`, or let an agent do it through the `agh__task_recover` native tool and the HTTP/UDS API. Task completion now returns the IDs of any tasks it created, and stronger validation rejects invalid created-task references before they are persisted. Claim tokens are redacted across task outputs and hook payloads, and the new `needs_attention_after` setting controls how long a task may wait before it is flagged for attention.

#### Fixes

##### Clearer native tool errors and availability diagnostics

Native and hosted tool calls now fail legibly. Hosted MCP tool calls return richer, structured JSON error details — the tool ID, an error code, and any denial reasons — instead of opaque failures, and advertised tools carry improved metadata and descriptions. When runtime diagnostic data cannot be retrieved, native tool lookups report a clear "unavailable" diagnostic rather than a silent miss. Hosted and session MCP availability handling is safer overall, with clearer informational and warning logs and graceful fallbacks when a feature is disabled. Documentation and the runtime envelope now consistently direct agents to the canonical `agh__*` tool IDs and the harness-returned tool references.

## 0.0.8 - 2026-06-22

### 🐛 Bug Fixes

- Acp update and general fixes (#256)
- Make docs markdown copy retryable

### 📦 Build System

- Update deps
- Fix msw setup

### 🧪 Testing

- Remove not needed test

## 0.0.7 - 2026-06-04

### ♻️ Refactoring

- Improvements on create modals (#252)

### 🐛 Bug Fixes

- Repair daemon-served e2e flows

## 0.0.6 - 2026-06-03

### 🎉 Features

- Dependency-driven auto-enqueue (opt-in) (#232)

### 🐛 Bug Fixes

- Wake coordinator sessions reliably (#240)
- Enable runtime evidence profiles (#242)
- React doctor fixes (#245)
- Verify marketplace skill installs (#244)
- Persist active workspace and redirect on session/workspace mismatch (#238)
- Safe workspace delete and agh session remove command (#239)
- Unblock release CI (bootstrapRun complexity, stale gate tests) (#249)

### 📚 Documentation

- Release notes

### Release Notes

#### Features

##### Dependency-driven auto-enqueue

Tasks can now opt into dependency-driven auto-enqueue. When a task is marked `auto_enqueue_on_ready`, AGH enqueues its next task run automatically as soon as a blocking dependency completes and the task reconciles to `ready` — so a dependency graph advances without a manual `agh task enqueue` at each step. The behavior is conservative: only a successful completion satisfies a `blocks` edge, paused dependents are skipped, and the queued-run reservation keeps at most one open run per dependent under concurrent or retried completions. Enqueue happens only after the completion has durably committed and is best-effort — a failed enqueue is logged, never rolled back onto the completing run. It is off by default; enable it per task with `--auto-enqueue-on-ready` on `agh task create` / `agh task child create`, toggle it with `agh task update`, or set the `auto_enqueue_on_ready` field over HTTP/UDS and the extension SDK.

##### Runtime evidence mode for task execution profiles

Task execution profiles can now drive worker startup and opt into runtime evidence mode. A task with no pool owner but a `worker.mode = "select"` profile now starts the selected agent, provider, and model and propagates the profile's required capabilities into its claim command. Setting `runtime.mode = "evidence"` boots that worker with guidance to run browser, simulator, and local-app validation and to capture runtime evidence. AGH elevates the session to auto-approve permissions only when the profile also pins an explicit sandbox reference (`sandbox.mode = "ref"`); otherwise the configured permission policy stays in force, and evidence mode grants no extra task authority. The profile's new `runtime` block is surfaced through the execution-profile CLI, the HTTP/UDS endpoints, and the native task tools.

#### Fixes

##### Coordinator sessions wake reliably on new work

Coordinator sessions now wake reliably when new work arrives. When a task run is enqueued for a workspace that already has a running coordinator session, AGH delivers a synthetic wake to that session — interrupting the agent's current turn if it is idle and waiting — so queued runs are picked up promptly instead of stalling. The same interrupt-if-waiting delivery now applies to harness re-entry and heartbeat wakes, force-retried and force-recovered runs re-trigger a coordinator wake, and heartbeat wakes skipped for non-transient reasons are recorded as dropped rather than silently lost. Wake delivery is de-duplicated per session and run and drained safely across daemon shutdown.

##### Marketplace skill installs are verified end-to-end

Marketplace skill installs are now verified end-to-end. After downloading and writing a skill, AGH confirms the runtime can actually discover it as an enabled marketplace skill with matching provenance, instead of reporting success for an install that would never resolve. Installs that are disabled, shadowed by a higher-precedence skill of the same name, missing provenance, or resolved to a different slug now fail with a clear, terminal error and remediation guidance across `agh skill install` and the daemon API. Use `agh skill where <name>` to inspect the winning source before retrying.

##### Safe workspace deletion, plus agh session remove and agh open

Workspace deletion is now safe: AGH refuses to delete a workspace while any of its sessions are still active — returning a 409 that names the blocking sessions — and cleans up the workspace's stopped session history transactionally when deletion proceeds. Two agent-manageable CLI commands ship alongside it: `agh session remove <id>` deletes a single session and its persisted history, and `agh open` opens the AGH web UI in your default browser. The CLI reference also gains documentation for `agh open`, `agh session remove`, and the existing `agh onboarding` command group.

##### Web UI remembers your active workspace

The web UI now remembers your active workspace across reloads and browser restarts, so a refresh no longer snaps you back to the first workspace. Direct and shared session links resolve a session's owning workspace from its ID and load reliably regardless of which workspace is selected, and the UI redirects to the agent page when an opened session belongs to a different workspace than the active one.

## 0.0.5 - 2026-05-29

### ♻️ Refactoring

- Optimize prompt consumption (#222)
- Orchestration improvements (#230)

### 🎉 Features

- Decouple worker concurrency from coordinator uniqueness (#229)

### 🐛 Bug Fixes

- Handle provider overlay subtables (#228)

### 📚 Documentation

- Add contributors

### 🔧 CI/CD

- Web asset release

## 0.0.4 - 2026-05-27

### 🐛 Bug Fixes

- Default workspace

### 📚 Documentation

- Remove old changelog from website

### 📦 Build System

- Sync web assets module (#217)

## 0.0.3 - 2026-05-27

### ♻️ Refactoring

- Memory optimization (#215)

### 📦 Build System

- Sync web assets module (#210)

## 0.0.2 - 2026-05-27

### Other Changes

- Lessons learned

### ♻️ Refactoring

- Project structure (#7)
- Kb improvements (#12)
- Rename spaces to channels (#17)
- Add extensions gaps (#21)
- Improve tool calls ui (#22)
- Remove web app header
- Module improvements (#29)
- Memory improvements (#35)
- Storybook for web and ui (#38)
- Enable AGH network by default for new installs (#57)
- Hermes adjustments (#69)
- Badges design (#84)
- Storybook scenario and logos gallery
- Migrate typescript tests (#114)
- Internal go packages (#120)
- Ui patterns (#127)
- Improve e2e tests (#130)
- Ui redesign
- Workspace isolation across runtime surfaces (#145)
- Prod ready applies (#162)
- Tool card ui (#164)
- Alpha on logo
- Prod ready features (#167)
- Thread sheet (#202)

### 🎉 Features

- Implement config foundation packages
- Implement sqlite store package
- Add ACP client package
- Add session lifecycle manager
- Implement observe package
- Add daemon composition root
- Add uds api server
- Implement cli package
- Add http api server
- Add system design
- Add foundation types, schemas, and layout shell for web client
- Add daemon health polling and agent sidebar systems for web client
- Add session system CRUD, streaming core, and session store for web client
- Add chat view, messages, and composer tests for web client
- Add tool cards and renderers for web client
- Add file-backed memory store core
- Scaffold memory session seams
- Add memory dream consolidation service
- Wire memory assembler into daemon
- Add memory api and cli
- New skills system (#1)
- Add workspace entity (#5)
- Add new skill capabilities (#8)
- Web ui v2 (#9)
- Improve hooks system (#10)
- Session resilience (#11)
- Add extensability (#13)
- Add automation (#16)
- Add channels (#14)
- Add network implementation (#15)
- Add network, bridges and automations web pages (#18)
- Ext registry (#20)
- Add core tasks (#19)
- Bridge adapters (#23)
- Add site (#26)
- Add ext refac and sandbox (#25)
- Settings ui (#37)
- Tasks ui (#36)
- Harness improvements (#44)
- Agent capabilities (#49)
- Redesign ui (#48)
- Unify capability (#53)
- Redesign network workspace (#59)
- Add task deletion and split session delete from stop (#58)
- Session provider selection (#60)
- Production grade adjustments (#66)
- Autonomous system (#75)
- Add agent session route (#80)
- Tools registry (#85)
- Agents soul (#88)
- Add network threads (#105)
- Orchestration improvements (#106)
- Memory v2 (#108)
- Agent categories (#113)
- Providers model (#118)
- Add canonical AGH bundled skill (#143)
- Onboarding and improvements (#198)
- Onboarding and improvements (#201)

### 🐛 Bug Fixes

- Review round
- Review rounds
- Resolve memory extensibility review batch
- Embed web into daemon
- Defaults agents
- Acp integration (#4)
- Lint errors
- Prd folder
- Remove orphan web actions and dead surfaces (#55)
- Qa testing and fixes (#73)
- New review rounds (#82)
- Security audit (#90)
- Release qa round (#95)
- Add missing tools (#141)
- New qa round (#147)
- Advanced qa round (#149)
- Homebrew tap
- Final review round (#151)
- Daemon healthy
- Reasoning models (#158)
- Lint errors (#160)
- Review round (#168)
- Release adjustments (#171)
- Stabilize release ci fixtures
- Stabilize release integration gate
- Stabilize release verify gates
- Stabilize release integration flows
- Stabilize release verify gates
- Stabilize main verify shutdown
- Ignore stale acpmock cancel
- Marketplace search focus and filtering (#193)
- Website video
- Workspace command select

### 📚 Documentation

- Update agents.md
- Update prd
- Update skills
- Update compozy tasks
- Update compozy
- Update compozy
- Add new skills
- Archive prd
- Update prds
- Update rfc
- Update prds
- Update prds
- Add automation prd
- Channels prd
- Update prd
- Update prd
- New prds
- Archive prds
- Bridges adapters prd
- Sandbox prd
- Update
- Archive prd
- Update
- Add new prd
- New design
- Update prd
- Archive prds
- Update prds
- Tasks-ui prd tasks
- Update prd
- Update design docs
- Agent capabilities prd
- Improve site docs
- Remove old design references
- Udpate
- Autonomous prd
- Update skills
- Blog design
- Agent sould prd
- Final qa plan
- Update
- Remove codex ledgers from gitignore
- Remove not needed files
- Udpate ledger
- Update cy-codex-loop skill
- Orchestration improves prd
- Update prds
- Orch improvs prd
- Memv2 prd
- Providers model prd
- Update refacs prd
- New design proposal
- Update rules
- Update skills
- New blog posts (#173)
- Format docs
- Remove old design files
- Remove old
- Skeeper update

### 📦 Build System

- Initial structure
- Commitlint
- Frontend base structure
- Update vscode settings
- Add subagents
- Coderabbit
- Prd and tooling
- Bun lock
- Lint tooling
- Copy.md and tooling adjusts
- Add repoclone rc
- Upgrade skeeper to v0.2.0
- Update go.mod
- Adopt task artifacts into skeeper
- Sync codex plans with skeeper
- Skeeper lock
- Skeeper lock
- New skills
- Skeeper lock
- Skeeper lock
- Skeeper lock
- Update deps and go
- Regenerate daytona sidecar assets for go 1.26.3
- Fix cliff
- Ignore docs on fmt
- Build web assets before goreleaser
- Extend release dry-run timeout
- Fix release dry-run token contract

### 🔧 CI/CD

- Lint errors
- Fint release pr
- Fix goreleaser
- Fix release
- Fix release process
- Fix release sync
- Decouple release dry-run npm auth
- Persist web assets git auth
- Require npm auth before release merge

### 🧪 Testing

- Add e2e tests (#27)
- Qa rounds (#78)
- Improve test suite (#138)
- Harden daemon-served restart reloads
- Harden daemon-served readiness waits
- Stabilize dashboard focus assertion
- Stabilize release integration gates
- Stabilize release e2e markers
- Stabilize release e2e flows
- Improve suite speed

## 0.0.1 - 2026-05-26

### Other Changes

- Lessons learned

### ♻️ Refactoring

- Project structure (#7)
- Kb improvements (#12)
- Rename spaces to channels (#17)
- Add extensions gaps (#21)
- Improve tool calls ui (#22)
- Remove web app header
- Module improvements (#29)
- Memory improvements (#35)
- Storybook for web and ui (#38)
- Enable AGH network by default for new installs (#57)
- Hermes adjustments (#69)
- Badges design (#84)
- Storybook scenario and logos gallery
- Migrate typescript tests (#114)
- Internal go packages (#120)
- Ui patterns (#127)
- Improve e2e tests (#130)
- Ui redesign
- Workspace isolation across runtime surfaces (#145)
- Prod ready applies (#162)
- Tool card ui (#164)
- Alpha on logo
- Prod ready features (#167)
- Thread sheet (#202)

### 🎉 Features

- Implement config foundation packages
- Implement sqlite store package
- Add ACP client package
- Add session lifecycle manager
- Implement observe package
- Add daemon composition root
- Add uds api server
- Implement cli package
- Add http api server
- Add system design
- Add foundation types, schemas, and layout shell for web client
- Add daemon health polling and agent sidebar systems for web client
- Add session system CRUD, streaming core, and session store for web client
- Add chat view, messages, and composer tests for web client
- Add tool cards and renderers for web client
- Add file-backed memory store core
- Scaffold memory session seams
- Add memory dream consolidation service
- Wire memory assembler into daemon
- Add memory api and cli
- New skills system (#1)
- Add workspace entity (#5)
- Add new skill capabilities (#8)
- Web ui v2 (#9)
- Improve hooks system (#10)
- Session resilience (#11)
- Add extensability (#13)
- Add automation (#16)
- Add channels (#14)
- Add network implementation (#15)
- Add network, bridges and automations web pages (#18)
- Ext registry (#20)
- Add core tasks (#19)
- Bridge adapters (#23)
- Add site (#26)
- Add ext refac and sandbox (#25)
- Settings ui (#37)
- Tasks ui (#36)
- Harness improvements (#44)
- Agent capabilities (#49)
- Redesign ui (#48)
- Unify capability (#53)
- Redesign network workspace (#59)
- Add task deletion and split session delete from stop (#58)
- Session provider selection (#60)
- Production grade adjustments (#66)
- Autonomous system (#75)
- Add agent session route (#80)
- Tools registry (#85)
- Agents soul (#88)
- Add network threads (#105)
- Orchestration improvements (#106)
- Memory v2 (#108)
- Agent categories (#113)
- Providers model (#118)
- Add canonical AGH bundled skill (#143)
- Onboarding and improvements (#198)
- Onboarding and improvements (#201)

### 🐛 Bug Fixes

- Review round
- Review rounds
- Resolve memory extensibility review batch
- Embed web into daemon
- Defaults agents
- Acp integration (#4)
- Lint errors
- Prd folder
- Remove orphan web actions and dead surfaces (#55)
- Qa testing and fixes (#73)
- New review rounds (#82)
- Security audit (#90)
- Release qa round (#95)
- Add missing tools (#141)
- New qa round (#147)
- Advanced qa round (#149)
- Homebrew tap
- Final review round (#151)
- Daemon healthy
- Reasoning models (#158)
- Lint errors (#160)
- Review round (#168)
- Release adjustments (#171)
- Stabilize release ci fixtures
- Stabilize release integration gate
- Stabilize release verify gates
- Stabilize release integration flows
- Stabilize release verify gates
- Stabilize main verify shutdown
- Ignore stale acpmock cancel
- Marketplace search focus and filtering (#193)
- Website video
- Workspace command select

### 📚 Documentation

- Update agents.md
- Update prd
- Update skills
- Update compozy tasks
- Update compozy
- Update compozy
- Add new skills
- Archive prd
- Update prds
- Update rfc
- Update prds
- Update prds
- Add automation prd
- Channels prd
- Update prd
- Update prd
- New prds
- Archive prds
- Bridges adapters prd
- Sandbox prd
- Update
- Archive prd
- Update
- Add new prd
- New design
- Update prd
- Archive prds
- Update prds
- Tasks-ui prd tasks
- Update prd
- Update design docs
- Agent capabilities prd
- Improve site docs
- Remove old design references
- Udpate
- Autonomous prd
- Update skills
- Blog design
- Agent sould prd
- Final qa plan
- Update
- Remove codex ledgers from gitignore
- Remove not needed files
- Udpate ledger
- Update cy-codex-loop skill
- Orchestration improves prd
- Update prds
- Orch improvs prd
- Memv2 prd
- Providers model prd
- Update refacs prd
- New design proposal
- Update rules
- Update skills
- New blog posts (#173)
- Format docs
- Remove old design files
- Remove old
- Skeeper update

### 📦 Build System

- Initial structure
- Commitlint
- Frontend base structure
- Update vscode settings
- Add subagents
- Coderabbit
- Prd and tooling
- Bun lock
- Lint tooling
- Copy.md and tooling adjusts
- Add repoclone rc
- Upgrade skeeper to v0.2.0
- Update go.mod
- Adopt task artifacts into skeeper
- Sync codex plans with skeeper
- Skeeper lock
- Skeeper lock
- New skills
- Skeeper lock
- Skeeper lock
- Skeeper lock
- Update deps and go
- Regenerate daytona sidecar assets for go 1.26.3
- Fix cliff
- Ignore docs on fmt
- Build web assets before goreleaser
- Extend release dry-run timeout

### 🔧 CI/CD

- Lint errors
- Fint release pr
- Fix goreleaser
- Fix release

### 🧪 Testing

- Add e2e tests (#27)
- Qa rounds (#78)
- Improve test suite (#138)
- Harden daemon-served restart reloads
- Harden daemon-served readiness waits
- Stabilize dashboard focus assertion
- Stabilize release integration gates
- Stabilize release e2e markers
- Stabilize release e2e flows
