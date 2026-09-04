# L-040 — Real users end the zero-legacy posture; compatibility is tiered by contract owner

**Class:** Project posture / Release / Process
**Date discovered:** 2026-08-22 (first real users on constant releases); decision codified 2026-09-04
**Evidence sources:** v0.3.0 release notes under `.release-notes/archive/v0.3.0/`; the resident `CLAUDE.md` "Greenfield Alpha — Zero Legacy Tolerance" section and SD-002 as they stood on 2026-09-04; git history of `CLAUDE.md` across every ref, stash, worktree, and reflog entry on 2026-09-04.

## Context

The greenfield posture (`CLAUDE.md` §Greenfield, SD-002, L-006, L-025) was written when nobody outside the team ran Compozy. It said "no production deployments to preserve" and told every agent to hard-cut instead of migrating. The rule worked exactly as written during the v0.3.0 beta line while real users were already installing each release:

- `loop-feedback-semantics-*.md` — "a greenfield hard cut that discards existing Loop run history … Export anything you need before upgrading."
- `window-tabs-in-the-os-shell-*.md` — persisted window layouts moved to v3 "as a hard cut — v2 layout compatibility paths … removed".
- `runtime-speed-is-part-of-the-runtime-*.md` — session creation profile "moves to v3 as a hard cut, with no v2 branch".
- `implement-tasks-replaces-software-delivery-*.md` — loop rename "with no alias"; every config, CLI call, automation binding, and doc link broke at once.
- `the-desktop-app-is-now-electron-*.md` — desktop runtime replaced "with no compatibility bridge", and the installed-app update walk "was not verified for this build".

On 2026-08-22 the owner decided the posture had to change and a tiered compatibility proposal was drafted. On 2026-09-04 the owner reread `CLAUDE.md` expecting the section to be gone. It was not: no commit on any ref, no stash, no worktree, and no reflog entry ever removed it. For two weeks agents kept obeying a rule the owner believed retired.

## Root cause

Two independent causes:

1. **The rule's premise had a shelf life and nothing tied it to the event that would falsify it.** "No production deployments" was a fact about April 2026 hardened into a perpetual directive. When users arrived, no gate, review prompt, or release step re-checked the premise — the release notes even reused the word "greenfield" to describe destroying their data.
2. **A policy change that lives only in a conversation is not a policy.** Agents read the resident instruction file and the standing directives, not the chat where the decision was made. Until the edit lands in those files in the same session as the decision, the old rule is still the rule.

## Rule

> Compatibility is a contract tiered by who owns the surface, not a binary. **User state** (SQLite streams, `config.toml`, workspace files, persisted layouts) never breaks: every shape change ships a lossless migration. **Public scripted surfaces** (CLI, HTTP/UDS, hooks, SDKs, config keys, tool IDs) auto-migrate when possible and otherwise deprecate for one release before deletion. **Internal code** stays hard-cut. Compat is translation at the boundary, one shim generation at a time, each naming its removal release. And a posture change exists only once it lands in `CLAUDE.md` and `standing_directives.md` in the same commit as the decision.

## Operationalization

- Before approving a spec, tag every delete target with its regime and ladder outcome (auto-migrate / deprecate N→N+1 / `experimental` break). SD-013 carries the full ladder; L-006 still owns the enumeration.
- A migration that drops or truncates rows is a spec decision with the user's sign-off recorded in an ADR and a release-note `Migration notes` block — never a default the migration author picks.
- A shim is a loader/decoder/alias-table entry at the edge, never an `if oldShape` branch in domain code; its comment and release note name the release that removes it.
- When a standing rule rests on a fact about the world ("no users", "no published peers"), write the falsifying event next to it and re-check on every release.
- When the owner decides a posture change in conversation, edit the resident files and the SD in that session and confirm the commit hash back; "we agreed" without a commit is the failure mode this lesson records.

## Anti-pattern

- Release notes that describe destroying user data as a "greenfield hard cut" and ask users to export before upgrading.
- Renaming a CLI verb, config key, or tool ID with no alias window because "internal code is hard-cut" — the internal rule applied to a public surface.
- Stacked shims (N-2 still accepted) or a shim with no removal release: eternal compat by neglect.
- `if oldShape` branches inside domain code instead of one boundary translation.
- Believing a rule changed because a proposal was written; the resident file still carries the old rule.

## Source

- `.release-notes/archive/v0.3.0/loop-feedback-semantics-1785947603.md:13`, `window-tabs-in-the-os-shell-1785600988.md:12`, `runtime-speed-is-part-of-the-runtime-1787249316.md:12`, `implement-tasks-replaces-software-delivery-1786035360.md:12`, `the-desktop-app-is-now-electron-1787003301.md:12` — the hard cuts shipped to real users.
- `CLAUDE.md` §Greenfield and SD-002 as of commit `8eeb8a381` (`feat!: introducing CompozyOS beta`) — the only commit that ever touched the section before 2026-09-04.
- Superseded posture: [L-006](L-006-greenfield-delete-not-adapt.md) (enumeration rule kept, scope narrowed to internal code), [L-025](L-025-greenfield-hardcut-current-protocol-version.md) (versioning rule kept, "no published peers" condition now explicit).
- Replacement directive: SD-013 in `docs/_memory/standing_directives.md`.
