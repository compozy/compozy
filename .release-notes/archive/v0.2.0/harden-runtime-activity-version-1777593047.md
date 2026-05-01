---
title: Harden runtime activity tracking and version handling
type: fix
---

A bundle of reliability fixes across the update notifier, native Codex ACP runtime, ACP activity tracking, the extension SDK, and the build toolchain.

### What's fixed

- **Update notifier no longer prompts a "downgrade"** on git-describe builds. Pre-release suffixes like `-15-g834fec6` are stripped before semver comparison, so a binary built ahead of the latest tag stops nagging users to install the older release. Identifiers like `1.2.3-1-gamma` are preserved unless the suffix is a plausible short SHA.
- **Native Codex ACP runtime accepts `codex/<model>` aliases.** The provider prefix is stripped before `SetSessionModel`, fixing rejections for ChatGPT-account Codex sessions.
- **ACP activity stays "active" for the full lifecycle of a session update**, including nested or concurrent submissions. Previously the tracker could mark a session idle while in-flight work was still being submitted, dropping events on the floor.
- **Extension SDK publishes `initialized` state before sending the initialize response**, fixing a host-side race where `runs.start` could be rejected as "extension not initialized" immediately after handshake.
- **`BUN_VERSION` is now a minimum supported version, not an exact pin.** Error messaging updated to "or newer / at least", so contributors with a newer Bun release stop seeing spurious version errors.

### Before / after

```
# Before: a binary built between releases prompted a "downgrade" install
$ compozy --version
v0.1.12-15-g834fec6
$ compozy ...
Update available: 0.1.12 (you have v0.1.12-15-g834fec6)

# After: git-describe suffix is stripped; no spurious prompt
$ compozy ...
(no update notice)
```
