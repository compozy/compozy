# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## 0.3.0 - 2026-07-28

### 🎉 Features

- Introducing CompozyOS beta
- **BREAKING:** introducing CompozyOS beta

### 🐛 Bug Fixes

- _(cli)_ Reap leaked test daemons and artifacts (#253)
- Archive without tasks
- Resolve inherited cross-runtime models against runtime defaults (#259)

### 📚 Documentation

- Update readme

### 📦 Build System

- Auto-publish beta releases on release PR merge
- Push release branch updates with RELEASE_TOKEN
- Publish site changelog receipts from the release job
- Delete not need things
- Fix tests
- Align release body heading with the published version

### 🔧 CI/CD

- Fix tests

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

### 📚 Documentation

- Add v0.2.15 release highlights

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

### 📚 Documentation

- Update skills
- Add v0.2.11 release notes

## 0.2.10 - 2026-06-18

### ♻️ Refactoring

- Tui redesign (#201)

### 🎉 Features

- Worktree-backed parallel multi-run for tasks run --multiple (#200)
- Add Devin CLI agent support (#204)

### 🐛 Bug Fixes

- Reviews watch bug

### 📚 Documentation

- Release notes

### 📦 Build System

- Skeeper config (#206)
- Converge skeeper sidecar lock to main branch

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

## 0.2.7 - 2026-05-27

### 🔧 CI/CD

- Support forced release version via workflow_dispatch (#175)

## 0.2.6 - 2026-05-27

### 🎉 Features

- Add multi-task run support (#162)

### 🐛 Bug Fixes

- Add Windows daemon support (#163)

### 📚 Documentation

- Release notes

## 0.2.5 - 2026-05-25

### 🎉 Features

- Add zsh task completion plugin docs and script (#149)
- Add kiro-cli as supported ACP execution runtime (#160)
- Discover task files recursively in nested subdirectories (#153)

### 🐛 Bug Fixes

- Homebrew formula
- Emit one task slug per compozy completion candidate (#159)
- Run managed upgrade commands (#158)

### 📚 Documentation

- Add star history on readme
- Release notes

### 🔧 CI/CD

- Release fix

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

### 📚 Documentation

- Update
- Release notes

### 📦 Build System

- Release tool

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

### 📚 Documentation

- Release notes
- Daemon prd
- New prds
- Update
- Add release notes

### 🔧 CI/CD

- Fix auto-docs
- Add release notes
- Fix windows

### 🧪 Testing

- Release config

## 0.1.12 - 2026-04-14

### 🎉 Features

- Add shared layout package for run artifact filenames (#95)

### 🐛 Bug Fixes

- Execution order
- Fetch reviews parsing

### 🔧 CI/CD

- _(release)_ Prepare release v0.1.12 (#100)

### 🧪 Testing

- Fix suite

## 0.1.11 - 2026-04-14

### 🎉 Features

- Agents spec (#78)
- Add extensability (#80)
- Add compozy skill
- Extension improvements (#83)
- Migrate core extension (#93)

### 📚 Documentation

- New prds
- Add auto-docs workflow (claude code on merge)
- Update
- Update docs path

### 📦 Build System

- Auto-docs workflow

### 🔧 CI/CD

- _(release)_ Prepare release v0.1.11 (#94)

## 0.1.10 - 2026-04-10

### ♻️ Refactoring

- Improve packages (#70)
- Add nitpicks for coderabbit (#75)

### 🎉 Features

- Kernel refactoring (#68)

### 🐛 Bug Fixes

- Stop rewriting all _meta.md files when listing workflows (#73)

### 🔧 CI/CD

- _(release)_ Prepare release v0.1.10 (#76)

## 0.1.9 - 2026-04-06

### 🎉 Features

- Exec command (#60)

### 🐛 Bug Fixes

- Close issue #61 (#63)
- Fail for unsupported --add-dir (#66)

### 📚 Documentation

- Context7 and exa skills

### 🔧 CI/CD

- _(release)_ Prepare release v0.1.9 (#67)

## 0.1.8 - 2026-04-05

### ♻️ Refactoring

- Rename idea-factory artifacts from issue to idea (#56)

### 🎉 Features

- Add GitHub Copilot CLI as ACP runtime (#57)

### 🔧 CI/CD

- _(release)_ Prepare release v0.1.8 (#59)

## 0.1.7 - 2026-04-05

### ♻️ Refactoring

- Tool calls (#48)
- Task artifacts changes (#52)

### 🎉 Features

- _(build)_ Add AUR support and automation via GoReleaser (#49)

### 🐛 Bug Fixes

- Review round

### 📦 Build System

- Comment AUR release for now

### 🔧 CI/CD

- _(release)_ Prepare release v0.1.7 (#55)

## 0.1.6 - 2026-04-04

### 🐛 Bug Fixes

- Improve failures

### 📦 Build System

- Remove ai-docs folder

### 🔧 CI/CD

- _(release)_ Prepare release v0.1.6 (#47)

## 0.1.5 - 2026-04-03

### 🎉 Features

- Add config.toml (#40)

### 🐛 Bug Fixes

- Check skills shift before run
- Acp permission

### 🔧 CI/CD

- _(release)_ Prepare release v0.1.5 (#45)

## 0.1.4 - 2026-04-03

### 🎉 Features

- Add cy-idea-factory skill and improve planning skills DX (#35)

### 🐛 Bug Fixes

- Failed tool call crash
- Skills frontmatter

### 📦 Build System

- Fix skills symlink

### 🔧 CI/CD

- _(release)_ Prepare release v0.1.4 (#39)

## 0.1.3 - 2026-04-03

### 🎉 Features

- _(repo)_ Add archive command
- Use acp instead of stream raw json (#34)

### 📚 Documentation

- Archive old prds
- Update readme

### 🔧 CI/CD

- _(release)_ Prepare release v0.1.3 (#36)

## 0.1.2 - 2026-04-02

### 🐛 Bug Fixes

- _(repo)_ Close tui when finish
- Correct opencode run flags and add stdin support (#25)

### 📚 Documentation

- _(repo)_ Update readme

### 🔧 CI/CD

- _(release)_ Prepare release v0.1.2 (#28)

## 0.1.1 - 2026-04-02

### 🐛 Bug Fixes

- _(repo)_ Automatic completion

### 📚 Documentation

- _(repo)_ Remove installs

### 📦 Build System

- _(repo)_ Fix release

### 🔧 CI/CD

- _(release)_ Prepare release v0.1.1 (#24)

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

### 📚 Documentation

- _(repo)_ Improve readme
- _(repo)_ Remove old templates
- _(repo)_ Improve readme
- _(repo)_ Readme
- _(repo)_ Update readme

### 📦 Build System

- _(repo)_ Release
- _(repo)_ Gitignore
- _(repo)_ Rename to compozy
- _(repo)_ Bump tag

### 🔧 CI/CD

- _(release)_ Prepare release v0.0.1 (#4)
- _(release)_ Prepare release v0.0.2 (#5)
- _(release)_ Prepare release v0.0.3 (#11)
- _(release)_ Prepare release v0.1.0 (#21)
- _(repo)_ Fix tests

[0.3.0]: https://github.com/compozy/compozy/compare/v0.2.15...v0.3.0
[0.2.15]: https://github.com/compozy/compozy/compare/v0.2.14...v0.2.15
[0.2.14]: https://github.com/compozy/compozy/compare/v0.2.13...v0.2.14
[0.2.13]: https://github.com/compozy/compozy/compare/v0.2.12...v0.2.13
[0.2.12]: https://github.com/compozy/compozy/compare/v0.2.11...v0.2.12
[0.2.11]: https://github.com/compozy/compozy/compare/v0.2.10...v0.2.11
[0.2.10]: https://github.com/compozy/compozy/compare/v0.2.9...v0.2.10
[0.2.9]: https://github.com/compozy/compozy/compare/v0.2.8...v0.2.9
[0.2.8]: https://github.com/compozy/compozy/compare/v0.2.7...v0.2.8
[0.2.7]: https://github.com/compozy/compozy/compare/v0.2.6...v0.2.7
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
