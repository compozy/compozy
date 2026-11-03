---
title: Extension upgrades recover from retired and half-installed packages
type: fix
---

Upgrading no longer leaves the old `dev-cycle` extension in the way. CompozyOS moves its installs and profile enablement to `spec-cycle` in one step and keeps the old package files. If the old extension holds inputs, credentials, development links, profile markers or conflicting enablement that can't move safely, it stays registered and `compozy doctor` names its replacement and the exact removal command. Review that state before you remove it.

One broken extension also no longer fails another extension's install, update, enablement, removal or rollback. Its failure stays visible in status and in `compozy doctor`. A package left with no install can now be cleared with `compozy extension remove <name> --global`.

Migration notes: the `dev-cycle` to `spec-cycle` move runs automatically on upgrade and is scheduled for removal in v0.6.0, after the v0.5 migration window. Retired `ext__dev_cycle__*` tools have no alias and no tool replacement, because `spec-cycle` no longer ships tools in this release: see "Spec delivery moves from Loops to agents" for what to use instead. `cy-create-prd` and `cy-create-techspec` are now `cy-create-spec`, so review its inputs when you update scripts and Loop definitions.
