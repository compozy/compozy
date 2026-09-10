---
title: Previous-release sessions remain readable after upgrade
type: fix
---

Sessions created by 0.3.0-beta.21 remain readable after upgrading. The persisted
creation-profile codec accepts versions 4 and 5, validates their fields, and keeps
historical profile references, policy digests, and creation digests unchanged.
New runtime profiles continue to use version 5.

Unreadable session metadata now produces a bounded scan summary with at most five
examples. An unchanged set repeats at most once every five minutes per scanner;
changed failures are reported immediately. `compozy doctor --only
runtime.session_metadata -o json` reports live counts and a bounded sample,
including missing or invalid catalog creation witnesses.

Migration notes: the version 4 to 5 change added optional ACP selections. Version
4 profiles upgrade through the persistence codec with no selections, preserving
their original bytes and hashes instead of rewriting immutable witnesses. No SQL
migration, config change, session deletion, or manual reset is required. Unknown
versions and corrupted witnesses remain rejected and unchanged. This does not
change dreaming defaults or claim to reproduce the reported CPU/disk measurements.
