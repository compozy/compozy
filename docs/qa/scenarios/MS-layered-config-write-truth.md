---
id: MS-layered-config-write-truth
area: MS
title: Write layered configuration with truthful winning-source feedback
persona: Dora
journey: J-layer-profile-resources
expected: User, personal-profile, and workspace writes reach only their selected files; effective reads follow user → profile → workspace → workspace-profile precedence; a lower-layer save names the winning layer instead of claiming it applied; and machine-only profile writes fail without residue.
entry_points: compozy config path|get|set|unset --scope user|profile|workspace -o json; Settings Persona, Hooks, and Command palette; GET/PATCH /api/settings; compozy__config_get|set|unset
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: MS-background-role-routing; MS-worktree-config-bootstrap
---

Flagged by Profiles task 08. Task 13 owns the isolated real-user walk and verdict.

Walk the default and one non-default profile across two workspaces. Prove the context-owned default
write target, every explicit scope, all four read layers, `ok_overridden` with `winning_layer`, and
fresh read parity across CLI, HTTP, UDS, Web, and the native config tools. Attempt every machine-only
root plus `window_manager.global_shortcuts` in a profile file and through `--scope profile`; require
`profile_config_key_denied`, allowed-prefix guidance, and no file or apply-record residue.

Expected evidence: before/after file hashes, structured mutation and effective-read payloads, Settings
provenance captures, apply-record rows, and denial transcripts.

2026-08-23 reconciliation (Profiles task 12): kept as the owner of the layered config write
contract; no reset needed (already `untested` and minted for this behavior). Two cross-links for
the walk: `[cmd_palette]` keys participate in these layers and their palette-side effects are owned
by `ET-agent-palette-config-parity`, and the retired `--scope global` value must be rejected rather
than accepted anywhere — `user` is its replacement.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

QA impact 2026-10-09 (Subagents tools slice): `[subagents] result_max_chars` defaults to 60000,
accepts 1000–1000000, and supports workspace overrides. `TestSubagentsConfig` verifies real file
loading and workspace isolation; an isolated CLI run verified the default, user override, and
999 rejection. The integrated finalize-time reload walk remains owned by the spec QA slice.
