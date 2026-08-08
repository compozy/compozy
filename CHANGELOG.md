# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## 0.3.0 - 2026-08-08

### ♻️ Refactoring

- State management with xstate-store (#268)
- Site improvements (#277)
- Replace bundles with extension kits (#291)
- Modernize Go runtime packages (#293)

### 🎉 Features

- Introducing CompozyOS beta
- **BREAKING:** introducing CompozyOS beta
- Add provider-neutral ACP speed control (#267)
- Support grouped skill directories (#270)
- Session transcript calm-surface redesign (#271)
- Add permission-aware cross-workspace access (#275)
- Extensions improvements (#278)
- Select and switch runtime per session prompt (#283)
- Adopt MCP 2026 and expand the curated catalog (#284)
- **BREAKING:** MCP catalogs now require manifest_version 2 and public
  MCP transport no longer accepts SSE.
- Add support for window tabs (#287)
- Adopt feedback semantics for durable Loops (#290)
- Add complete Loop node lifecycle (#305)
- Rewind sessions to durable conversation checkpoints (#310)
- Add session-aware slash skill commands (#311)
- Add reversible session archiving and list actions (#309)
- Replace software-delivery with implement-tasks (#325)
- **BREAKING:** remove software-delivery; use implement-tasks without gate inputs.
- Parent and child sessions (#327)

### 🐛 Bug Fixes

- Align release and process test contracts
- Workspace params (#266)
- Onboarding workspace selection
- Restore Windows daemon support (PR #163 regression + new fixes) (#274)
- Untested cases from qa (#276)
- Remaining untested qa (#279)
- Durable session messaging (#288)
- Assistant-ui version
- Changelog generation (#292)
- Make busy-session inputs durable (#304)
- Restore durable ACP session continuity (#307)
- Preserve Loop template manifests across hydration (#317)
- Preserve ACP stream disconnect recovery (#319)
- Discover Cursor model catalog before sessions (#320)
- Keep managed skill loading on native seam (#323)
- Loop run bugs (#324)
- Enforce bundled agent and Loop ownership (#326)
- Session native tools and extensions details (#330)

### 🧪 Testing

- Guard ACP initialize protocol version (#318)

## 0.2.15 - 2026-07-17

### 🎉 Features

- Cy-capture-decisions — skill-only extension for durable decision capture (#237)

### 🐛 Bug Fixes

- Recover stalled and wedged multi-runs (#230)
- Share parallel task status enum (#241)
- Surface progress and bound the reviews-fix daemon start (#236)
- Package cy-qa-workflow as a module and make host.tasks.create v2-aware (#234)
- Correct Kiro CLI ACP model handling (#226)
- Isolate sync tests and clarify ignore checks (#248)
- Isolate task artifacts and add complexity runtime defaults (#250)

## 0.2.14 - 2026-07-15

### 🐛 Bug Fixes

- Acp integratoin

## 0.2.13 - 2026-07-10

### 🐛 Bug Fixes

- Codex acp

## 0.2.12 - 2026-07-10

### 🐛 Bug Fixes

- Parallel tasks (#231)

## 0.2.11 - 2026-07-03

### 🎉 Features

- Agentic runs (#212)
- Simplify repo-level default setup overrides (#90)
- Support COMPOZY_HOME env override for home directory (#216)

### 🐛 Bug Fixes

- Parallel execution (#217)
- Specifying the model on ACP (#215)
- Worktree management (#223)
- Restore run TUI elapsed timer across retry, failure, cancel, and remote paths (#221)

## 0.2.10 - 2026-06-18

### ♻️ Refactoring

- Tui redesign (#201)

### 🎉 Features

- Worktree-backed parallel multi-run for tasks run --multiple (#200)
- Add Devin CLI agent support (#204)

### 🐛 Bug Fixes

- Reviews watch bug

## 0.2.9 - 2026-06-14

### 🐛 Bug Fixes

- Record ACP token usage and adapt to acp-go-sdk v0.13.5 (#198)

## 0.2.8 - 2026-06-12

### 🎉 Features

- Warn when tasks run starts beside active runs in other workspaces (#190)

### 🐛 Bug Fixes

- Keep multi-run task timers ticking (#179)
- Treat model auto as runtime default (#181)
- Pin claude model via ANTHROPIC_MODEL instead of unsupported session/set_model (#187)
- Restart stale daemon when CLI and daemon versions mismatch (#191)
- Surface ACP session setup errors in job logs and fail runs fast (#192)
- Reviews watch (#196)

## 0.2.6 - 2026-05-27

### 🎉 Features

- Add multi-task run support (#162)

### 🐛 Bug Fixes

- Add Windows daemon support (#163)

## 0.2.5 - 2026-05-25

### 🎉 Features

- Add zsh task completion plugin docs and script (#149)
- Add kiro-cli as supported ACP execution runtime (#160)
- Discover task files recursively in nested subdirectories (#153)

### 🐛 Bug Fixes

- Homebrew formula
- Emit one task slug per compozy completion candidate (#159)
- Run managed upgrade commands (#158)

### 🧪 Testing

- Internal test fix

## 0.2.4 - 2026-05-14

### 🐛 Bug Fixes

- Codex acp integration (#151)

## 0.2.3 - 2026-05-09

### 🐛 Bug Fixes

- Cwd path

## 0.2.2 - 2026-05-09

### 🎉 Features

- Add qa extension (#138)

### 🐛 Bug Fixes

- Workspace register (#140)
- Workspace discover path
- Prevent false task completion via prompt kickoff + worktree diff-check (#144) (#145)

## 0.2.1 - 2026-05-01

### 🐛 Bug Fixes

- Binary release

## 0.2.0 - 2026-05-01

### ♻️ Refactoring

- Daemon improvs (#121)

### 🎉 Features

- Add optional sound notifications on run lifecycle events (#96)
- Global config defaults (#106)
- Add per task prop selection (#109)
- Migrate to daemon (#112)
- **BREAKING:** migrate to daemon (#112)
- Daemon web UI (#122)
- Web ui polish (#125)
- Review watch (#133)

### 🐛 Bug Fixes

- Daemons adjustments (#116)
- Harden runtime activity and version handling (#127)
- Release adjustments (#131)
- Infer task type during migrate (#129)
- Watch adjustments
- Lint errors

### 🧪 Testing

- Release config

## 0.1.12 - 2026-04-14

### 🎉 Features

- Add shared layout package for run artifact filenames (#95)

### 🐛 Bug Fixes

- Execution order
- Fetch reviews parsing

### 🧪 Testing

- Fix suite

## 0.1.11 - 2026-04-14

### 🎉 Features

- Agents spec (#78)
- Add extensability (#80)
- Add compozy skill
- Extension improvements (#83)
- Migrate core extension (#93)

## 0.1.10 - 2026-04-10

### ♻️ Refactoring

- Improve packages (#70)
- Add nitpicks for coderabbit (#75)

### 🎉 Features

- Kernel refactoring (#68)

### 🐛 Bug Fixes

- Stop rewriting all _meta.md files when listing workflows (#73)

## 0.1.9 - 2026-04-06

### 🎉 Features

- Exec command (#60)

### 🐛 Bug Fixes

- Close issue #61 (#63)
- Fail for unsupported --add-dir (#66)

## 0.1.8 - 2026-04-05

### ♻️ Refactoring

- Rename idea-factory artifacts from issue to idea (#56)

### 🎉 Features

- Add GitHub Copilot CLI as ACP runtime (#57)

## 0.1.7 - 2026-04-05

### ♻️ Refactoring

- Tool calls (#48)
- Task artifacts changes (#52)

### 🎉 Features

- _(build)_ Add AUR support and automation via GoReleaser (#49)

### 🐛 Bug Fixes

- Review round

## 0.1.6 - 2026-04-04

### 🐛 Bug Fixes

- Improve failures

## 0.1.5 - 2026-04-03

### 🎉 Features

- Add config.toml (#40)

### 🐛 Bug Fixes

- Check skills shift before run
- Acp permission

## 0.1.4 - 2026-04-03

### 🎉 Features

- Add cy-idea-factory skill and improve planning skills DX (#35)

### 🐛 Bug Fixes

- Failed tool call crash
- Skills frontmatter

## 0.1.3 - 2026-04-03

### 🎉 Features

- _(repo)_ Add archive command
- Use acp instead of stream raw json (#34)

## 0.1.2 - 2026-04-02

### 🐛 Bug Fixes

- _(repo)_ Close tui when finish
- Correct opencode run flags and add stdin support (#25)

## 0.1.1 - 2026-04-02

### 🐛 Bug Fixes

- _(repo)_ Automatic completion

## 0.1.0 - 2026-04-01

### ♻️ Refactoring

- _(repo)_ Improve commands
- _(repo)_ Remove not needed flags
- _(repo)_ Remove PR as required for fix-reviews
- _(repo)_ Improve setup command
- _(repo)_ Remove prd- tasks folder prefix
- _(repo)_ Many improvements
- _(repo)_ Add cy prefix for skills and memory system

### 🎉 Features

- _(repo)_ Add build and release
- _(repo)_ Add adr support
- _(repo)_ Add fetch reviews
- _(repo)_ Add review-round skill
- _(repo)_ Add setup command
- _(repo)_ Add _meta.md for tasks
- Main structure

### 🐛 Bug Fixes

- _(repo)_ Release
- _(repo)_ Color bugs

[0.3.0]: https://github.com/compozy/compozy/compare/v0.2.15...v0.3.0
[0.2.15]: https://github.com/compozy/compozy/compare/v0.2.14...v0.2.15
[0.2.14]: https://github.com/compozy/compozy/compare/v0.2.13...v0.2.14
[0.2.13]: https://github.com/compozy/compozy/compare/v0.2.12...v0.2.13
[0.2.12]: https://github.com/compozy/compozy/compare/v0.2.11...v0.2.12
[0.2.11]: https://github.com/compozy/compozy/compare/v0.2.10...v0.2.11
[0.2.10]: https://github.com/compozy/compozy/compare/v0.2.9...v0.2.10
[0.2.9]: https://github.com/compozy/compozy/compare/v0.2.8...v0.2.9
[0.2.8]: https://github.com/compozy/compozy/compare/v0.2.7...v0.2.8
[0.2.6]: https://github.com/compozy/compozy/compare/v0.2.5...v0.2.6
[0.2.5]: https://github.com/compozy/compozy/compare/v0.2.4...v0.2.5
[0.2.4]: https://github.com/compozy/compozy/compare/v0.2.3...v0.2.4
[0.2.3]: https://github.com/compozy/compozy/compare/v0.2.2...v0.2.3
[0.2.2]: https://github.com/compozy/compozy/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/compozy/compozy/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/compozy/compozy/compare/v0.1.12...v0.2.0
[0.1.12]: https://github.com/compozy/compozy/compare/v0.1.11...v0.1.12
[0.1.11]: https://github.com/compozy/compozy/compare/v0.1.10...v0.1.11
[0.1.10]: https://github.com/compozy/compozy/compare/v0.1.9...v0.1.10
[0.1.9]: https://github.com/compozy/compozy/compare/v0.1.8...v0.1.9
[0.1.8]: https://github.com/compozy/compozy/compare/v0.1.7...v0.1.8
[0.1.7]: https://github.com/compozy/compozy/compare/v0.1.6...v0.1.7
[0.1.6]: https://github.com/compozy/compozy/compare/v0.1.5...v0.1.6
[0.1.5]: https://github.com/compozy/compozy/compare/v0.1.4...v0.1.5
[0.1.4]: https://github.com/compozy/compozy/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/compozy/compozy/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/compozy/compozy/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/compozy/compozy/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/compozy/compozy/releases/tag/v0.1.0

---

_Generated by [git-cliff](https://git-cliff.org)_
