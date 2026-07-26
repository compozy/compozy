# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[0.0.9]: https://github.com/compozy/agh/compare/v0.0.8...v0.0.9
[0.0.8]: https://github.com/compozy/agh/compare/v0.0.7...v0.0.8
[0.0.7]: https://github.com/compozy/agh/compare/v0.0.6...v0.0.7
[0.0.6]: https://github.com/compozy/agh/compare/v0.0.5...v0.0.6
[0.0.5]: https://github.com/compozy/agh/compare/v0.0.4...v0.0.5
[0.0.4]: https://github.com/compozy/agh/compare/v0.0.3...v0.0.4
[0.0.3]: https://github.com/compozy/agh/compare/v0.0.2...v0.0.3
[0.0.2]: https://github.com/compozy/agh/releases/tag/v0.0.2

---

_Generated by [git-cliff](https://git-cliff.org)_
