---
id: ET-agent-palette-config-parity
area: ET
title: Script palette bindings, aliases, and pins with UI parity
persona: Ada
journey: J-operate-command-palette
expected: Every palette configuration mutation available in Settings is scriptable with identical semantics: bind and unbind (in-app and --global), alias set and clear, pin and unpin, and personalization show and reset apply atomically through the daemon, return the same structured conflict and validation errors HTTP returns (shortcut_conflict and alias_conflict naming the owner, invalid_alias with the grammar rule), transfer ownership only with explicit overwrite, reflect live in connected shells without restart, and read back consistently through bindings, list, the settings sections, and config.toml.
entry_points: compozy cmd-palette bind|unbind (+ --global)|alias set|alias clear|bindings|pin|unpin|personalization show|reset; GET|PATCH /api/settings/window-manager (HTTP + UDS); GET|PATCH /api/settings/cmd-palette (HTTP + UDS); PUT|DELETE /api/cmd-palette/pins/{id} (HTTP + UDS); compozy config get|set cmd_palette.*; [cmd_palette] in config.toml
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-agent-command-invoke; ET-web-command-palette-shortcuts; ET-palette-personalization-lifecycle; ET-desktop-global-summon
---

Minted by command-palette task 11 (planning): tasks 04–05 and 09 shipped the mutation verb set
(US-034) with atomic settings-PATCH semantics, but `ET-agent-command-invoke` owns only the
discover → invoke → approval flow — the configuration-parity behavior had no owner. Task 12 owns
the first walk.

2026-08-23 qa-impact (Profiles): `[cmd_palette]` fallback targets, personalization, and aliases are
now profile-layerable, so the same scripted mutation can resolve differently per profile, while
`[window_manager.global_shortcuts]` stays machine-only and is rejected on a profile layer with the
typed denylist error. Pins and personalization are partitioned by profile lens. Already `untested`,
so no reset was needed. Extend step 6 to write one `cmd_palette` key under `--scope profile` and
confirm the effective value differs between two profiles, and that `--global` binding into
`[window_manager.global_shortcuts]` is refused from a profile layer with allowed-prefix guidance.
The per-lens partitioning of pins and ranking is owned by `ET-profile-palette-lens-isolation`; the
layered write mechanics are owned by `MS-layered-config-write-truth`.

Walk (task_11 plan):

1. `bind` a chord owned by another command — exit 1 `shortcut_conflict` naming the owner; re-run
   with `--overwrite` — the loser is unbound atomically and flagged; `bindings -o json` reports
   effective bindings, aliases, dormant extension defaults with their conflict owners, and
   conflicts.
2. `alias set` with invalid grammar (whitespace, 33 chars) — exit 2 `invalid_alias` stating the
   1–32/no-whitespace rule; set a valid alias, then try to give it to a second command — exit 1
   `alias_conflict` naming the owner; `--overwrite` transfers and clears the loser.
3. `unbind` and `alias clear` — both return ok and the effective map drops the entries; an open
   shell reflects every mutation live (rows, cheatsheet) without reload.
4. Repeat one bind and one alias through `PATCH /api/settings/window-manager` — the 409/422 bodies
   carry the same error codes and owner names the CLI printed.
5. `pin` / `unpin` — idempotent status round-trip matching `PUT|DELETE /api/cmd-palette/pins/{id}`;
   `personalization show` / `reset` matches the Settings > Palette reset semantics.
6. `bind <id> <chord> --global` — the intended binding lands in `[window_manager.global_shortcuts]`
   and is readable back; `config get cmd_palette.personalization` and the settings sections agree
   after `config set`.

Expected evidence: CLI transcripts for every error class beside the matching HTTP response bodies,
the `bindings -o json` snapshot showing effective + dormant + conflicts, and a screenshot of the
open shell reflecting a scripted rebind without reload.
