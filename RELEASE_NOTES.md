# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
## 0.2.0 - 2026-04-19

### 🎉 Features

- Add optional sound notifications on run lifecycle events (#96)- Global config defaults (#106)- Add per task prop selection (#109)- Migrate to daemon (#112)  - **BREAKING:** migrate to daemon (#112)

### 📚 Documentation

- Release notes- Daemon prd
### 🔧 CI/CD

- Fix auto-docs- Add release notes
### 🧪 Testing

- Release config
## 0.1.12 - 2026-04-14

### 🎉 Features

- Add shared layout package for run artifact filenames (#95)
### 🐛 Bug Fixes

- Execution order- Fetch reviews parsing
### 🔧 CI/CD

- *(release)* Prepare release v0.1.12 (#100)

### 🧪 Testing

- Fix suite
## 0.1.11 - 2026-04-14

### 🎉 Features

- Agents spec (#78)- Add extensability (#80)- Add compozy skill- Extension improvements (#83)- Migrate core extension (#93)
### 📚 Documentation

- New prds- Add auto-docs workflow (claude code on merge)- Update- Update docs path
### 📦 Build System

- Auto-docs workflow
### 🔧 CI/CD

- *(release)* Prepare release v0.1.11 (#94)

## 0.1.10 - 2026-04-10

### ♻️  Refactoring

- Improve packages (#70)- Add nitpicks for coderabbit (#75)
### 🎉 Features

- Kernel refactoring (#68)
### 🐛 Bug Fixes

- Stop rewriting all _meta.md files when listing workflows (#73)
### 🔧 CI/CD

- *(release)* Prepare release v0.1.10 (#76)

## 0.1.9 - 2026-04-06

### 🎉 Features

- Exec command (#60)
### 🐛 Bug Fixes

- Close issue #61 (#63)- Fail for unsupported --add-dir (#66)
### 📚 Documentation

- Context7 and exa skills
### 🔧 CI/CD

- *(release)* Prepare release v0.1.9 (#67)

## 0.1.8 - 2026-04-05

### ♻️  Refactoring

- Rename idea-factory artifacts from issue to idea (#56)
### 🎉 Features

- Add GitHub Copilot CLI as ACP runtime (#57)
### 🔧 CI/CD

- *(release)* Prepare release v0.1.8 (#59)

## 0.1.7 - 2026-04-05

### ♻️  Refactoring

- Tool calls (#48)- Task artifacts changes (#52)
### 🎉 Features

- *(build)* Add AUR support and automation via GoReleaser (#49)

### 🐛 Bug Fixes

- Review round
### 📦 Build System

- Comment AUR release for now
### 🔧 CI/CD

- *(release)* Prepare release v0.1.7 (#55)

## 0.1.6 - 2026-04-04

### 🐛 Bug Fixes

- Improve failures
### 📦 Build System

- Remove ai-docs folder
### 🔧 CI/CD

- *(release)* Prepare release v0.1.6 (#47)

## 0.1.5 - 2026-04-03

### 🎉 Features

- Add config.toml (#40)
### 🐛 Bug Fixes

- Check skills shift before run- Acp permission
### 🔧 CI/CD

- *(release)* Prepare release v0.1.5 (#45)

## 0.1.4 - 2026-04-03

### 🎉 Features

- Add cy-idea-factory skill and improve planning skills DX (#35)
### 🐛 Bug Fixes

- Failed tool call crash- Skills frontmatter
### 📦 Build System

- Fix skills symlink
### 🔧 CI/CD

- *(release)* Prepare release v0.1.4 (#39)

## 0.1.3 - 2026-04-03

### 🎉 Features

- *(repo)* Add archive command
- Use acp instead of stream raw json (#34)
### 📚 Documentation

- Archive old prds- Update readme
### 🔧 CI/CD

- *(release)* Prepare release v0.1.3 (#36)

## 0.1.2 - 2026-04-02

### 🐛 Bug Fixes

- *(repo)* Close tui when finish
- Correct opencode run flags and add stdin support (#25)
### 📚 Documentation

- *(repo)* Update readme

### 🔧 CI/CD

- *(release)* Prepare release v0.1.2 (#28)

## 0.1.1 - 2026-04-02

### 🐛 Bug Fixes

- *(repo)* Automatic completion

### 📚 Documentation

- *(repo)* Remove installs

### 📦 Build System

- *(repo)* Fix release

### 🔧 CI/CD

- *(release)* Prepare release v0.1.1 (#24)

## 0.1.0 - 2026-04-01

### ♻️  Refactoring

- *(repo)* Improve commands
- *(repo)* Remove not needed flags
- *(repo)* Remove PR as required for fix-reviews
- *(repo)* Improve setup command
- *(repo)* Remove prd- tasks folder prefix
- *(repo)* Many improvements
- *(repo)* Add cy prefix for skills and memory system

### 🎉 Features

- *(repo)* Add build and release
- *(repo)* Add adr support
- *(repo)* Add fetch reviews
- *(repo)* Add review-round skill
- *(repo)* Add setup command
- *(repo)* Add _meta.md for tasks
- Main structure
### 🐛 Bug Fixes

- *(repo)* Release
- *(repo)* Color bugs

### 📚 Documentation

- *(repo)* Improve readme
- *(repo)* Remove old templates
- *(repo)* Improve readme
- *(repo)* Readme
- *(repo)* Update readme

### 📦 Build System

- *(repo)* Release
- *(repo)* Gitignore
- *(repo)* Rename to compozy
- *(repo)* Bump tag

### 🔧 CI/CD

- *(release)* Prepare release v0.0.1 (#4)
- *(release)* Prepare release v0.0.2 (#5)
- *(release)* Prepare release v0.0.3 (#11)
- *(release)* Prepare release v0.1.0 (#21)
- *(repo)* Fix tests

[0.2.0]: https://github.com///compare/v0.1.12...v0.2.0
[0.1.12]: https://github.com///compare/v0.1.11...v0.1.12
[0.1.11]: https://github.com///compare/v0.1.10...v0.1.11
[0.1.10]: https://github.com///compare/v0.1.9...v0.1.10
[0.1.9]: https://github.com///compare/v0.1.8...v0.1.9
[0.1.8]: https://github.com///compare/v0.1.7...v0.1.8
[0.1.7]: https://github.com///compare/v0.1.6...v0.1.7
[0.1.6]: https://github.com///compare/v0.1.5...v0.1.6
[0.1.5]: https://github.com///compare/v0.1.4...v0.1.5
[0.1.4]: https://github.com///compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com///compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com///compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com///compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com///releases/tag/v0.1.0
---
*Generated by [git-cliff](https://git-cliff.org)*

### Release Notes

#### Features

##### Global config defaults
Set personal defaults once in `~/.compozy/config.toml` and have them apply across every project. Project-level `.compozy/config.toml` always takes precedence, so teams keep control while individuals stop repeating themselves.

### Example

```toml
# ~/.compozy/config.toml  (global — applies to all projects)
[defaults]
ide = "claude"
model = "sonnet"
access_mode = "default"
auto_commit = true

[sound]
enabled = true
on_completed = "glass"
on_failed = "basso"

[exec]
model = "gpt-5.4"
verbose = true
```

```toml
# .compozy/config.toml  (project — overrides global)
[defaults]
model = "o4-mini"

[start]
include_completed = true
```

With both files above the effective config resolves to:

| Field                     | Value       | Source       |
| ------------------------- | ----------- | ------------ |
| `defaults.ide`            | `"claude"`  | global       |
| `defaults.model`          | `"o4-mini"` | project wins |
| `defaults.auto_commit`    | `true`      | global       |
| `sound.enabled`           | `true`      | global       |
| `exec.model`              | `"gpt-5.4"` | global       |
| `start.include_completed` | `true`      | project      |

All sections supported in project config (`[defaults]`, `[start]`, `[exec]`, `[fix_reviews]`, `[fetch_reviews]`, `[tasks]`, `[sound]`) work in the global file with the same schema.

##### Optional sound notifications on run lifecycle events
Opt-in audio cues that play when a run completes or fails, so you can step away from long-running sessions without missing the result. Ships **disabled by default** — no sound unless you explicitly enable it.

### Setup

Add a `[sound]` section to `.compozy/config.toml` (project or global):

```toml
[sound]
enabled = true
on_completed = "glass"   # plays on successful completion
on_failed = "basso"      # plays on failure or cancellation
```

### Built-in presets

Seven presets work cross-platform out of the box:

| Preset      | macOS                                   | Linux                                        | Windows                             |
| ----------- | --------------------------------------- | -------------------------------------------- | ----------------------------------- |
| `glass`     | `/System/Library/Sounds/Glass.aiff`     | `freedesktop/stereo/complete.oga`            | `Media\Windows Notify Calendar.wav` |
| `basso`     | `/System/Library/Sounds/Basso.aiff`     | `freedesktop/stereo/dialog-error.oga`        | `Media\chord.wav`                   |
| `ping`      | `/System/Library/Sounds/Ping.aiff`      | `freedesktop/stereo/message.oga`             | `Media\notify.wav`                  |
| `hero`      | `/System/Library/Sounds/Hero.aiff`      | `freedesktop/stereo/bell.oga`                | `Media\tada.wav`                    |
| `funk`      | `/System/Library/Sounds/Funk.aiff`      | `freedesktop/stereo/bell.oga`                | `Media\Ring06.wav`                  |
| `tink`      | `/System/Library/Sounds/Tink.aiff`      | `freedesktop/stereo/message.oga`             | `Media\ding.wav`                    |
| `submarine` | `/System/Library/Sounds/Submarine.aiff` | `freedesktop/stereo/phone-incoming-call.oga` | `Media\ringin.wav`                  |

### Custom sounds

Pass an absolute path to use your own audio file:

```toml
[sound]
enabled = true
on_completed = "/Users/you/sounds/success.wav"
on_failed = "/Users/you/sounds/fail.wav"
```

### Lifecycle events

| Event         | Config field   | When it fires                                  |
| ------------- | -------------- | ---------------------------------------------- |
| Run completed | `on_completed` | Task finishes successfully                     |
| Run failed    | `on_failed`    | Task errors out                                |
| Run cancelled | `on_failed`    | Task is interrupted (reuses the failure sound) |

Playback is synchronous with a 3-second timeout — a missing or slow audio file never blocks shutdown. Errors are logged at debug level and never surface to the user.