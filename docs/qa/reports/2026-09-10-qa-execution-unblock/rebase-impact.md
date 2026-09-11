# Main integration impact on QA

User requested the `git-rebase` workflow and inspection of each incoming change's PR. Source base: `ed7f2d7adc2d7ec38071677d28a2d6e87a019c28`; fetched target: `fec0e9b08631dd9e2d7eaa66fc7f76f5314ae5c0`. Local fixes before rebase: `4f5ae5298`, `f32f50a9c`, `ea8f4c3f3`, `e5a6b2cea`. Preserve the latest steering: ordinary managed tests use Cursor Grok 4.6 High Fast.

## Reviewed incoming changes

| Main change | Source and behavior | QA consequence |
| --- | --- | --- |
| `d3da98299` | Landing/marketplace design artifacts and orchestration instructions. GitHub commit association returned no PR. | No daemon or production Web runtime change. Preserve artifacts; no live acceptance inferred. |
| `d224f95ef` | [PR 607](https://github.com/compozy/compozy/pull/607): compacts skill catalogs at confirmed ACP delivery, resets uncertain delivery on failure/cancellation, excludes skills/situation from input-only roles. Frozen Bun installer for previews. | ET-049 and prompt/role/recovery expectations change. Receiver fixture and fresh real Cursor first/subsequent turns are relevant; historical prompt sizes are not current measurements. Our MCP text projection remains independently necessary. |
| `0a01305cc` | [PR 608](https://github.com/compozy/compozy/pull/608): private Tailscale proof uses provider-owned tsnet relay while retaining TLS hostname, nonce and tier restrictions. Additive verification_address SDK field. | RT-connectivity-provider-route changes; current local gateway is disabled. Real remote Tailnet/Linux credentials/device remain separate prerequisites. Preserve generated SDKs and exact proof; no false external pass. |
| `c1c06f835` | [PR 609](https://github.com/compozy/compozy/pull/609): Memory health reports the effective scoped Dream role, including degraded resolution failures, without invoking it. | MS-011 and memory/Settings/native diagnostics must use role truth. This lab has memory enabled and Dream disabled: health must report false. Shared native_tools_test.go must retain both upstream Dream cases and local Heartbeat profile cases. |
| `08d2c5bd8` | [PR 610](https://github.com/compozy/compozy/pull/610): mixed reasoning/tools share an ordered disclosure; deliberate terminals, visible decisions and narrative boundaries remain separate. Stable keys/reveal/folds. | RT-048/RT-055 and transcript interaction expectations change. Rebuild Web and inspect real retained Cursor mixed activity. Old rendered transcript evidence describes the old renderer only. |
| `29f4bee46` | [PR 611](https://github.com/compozy/compozy/pull/611): durable profile/actor acknowledgement receipts and exact unread snapshots, operator-only acknowledge routes, bell/Home Mark as read and Clear all, migration 109. Source work remains unchanged. | Reopen existing lab with additive migration 108→109 and retained state proof. Home/bell/title scenarios must distinguish unread occurrence from source status; receipts cannot consume later arrivals or cross scope. |
| `0e820dc0e` | Direct main integration, no associated PR: consolidates 607–611 reviews; routes/receipt cleanup, approval occurrence identity, Home ownership, exact same-turn prose concatenation, revision-pinned window opening. | Read the integration report and source diffs; retain all review fixes. Check session prose, existing-window reuse and notification boundaries; no wholesale conflict-side acceptance. |
| `0c02260ce` | Direct follow-up, no associated PR: unread polling continues in hidden tabs. | RT-web-attention-title-count must update without restoring focus. |
| `aecb5911b` | Direct follow-up, no associated PR: explicit encoding errors, named population, pointer use, formatter/lint corrections. | Preserve production handling and existing assertions; no weakened tests. |
| `fec0e9b08` | Direct follow-up, no associated PR: escalation occurrence uses NeedsAttention.At, not administrative activity time. | Acknowledgement survives neutral events; resolved-and-renewed escalation becomes unread. |

Read the complete PR bodies, commit lists, review summaries, representative production changes and the upstream `2026-09-10-pr-607-611-integration.md` report. Earlier PR bodies contain historical delivery-pending language; merge commits and fetched main establish inclusion, not those stale descriptions. REST commit-to-PR responses establish which direct commits have no PR.

## Preservation and verification

- Code backup ref and dirty archive: `rebase-preservation.json` in the run evidence directory. No untracked path overlaps incoming changes. Checkpoint only this task's QA documents; no stash or destructive Git.
- Owned daemon stopped through CLI, no active managed sessions. Read-only SQLite backups passed integrity checks before migration; `rebase-database-backup.json` records source paths and counts. Operator provider HOME/login untouched.
- Rebase directly onto fetched main, preserving all local fix commits. Inspect the final range-diff and any semantic conflicts.
- Rebuild Go/Web; run the required affected gate and targeted integration checks justified above. Restart the same isolated lab; verify schema 109, retained session/workspace state, Dream false, managed Cursor tools/terminal identity and the changed rendered transcript. Notification read behavior is checked against the new contract; full bell/title journeys remain in the 357-row execution inventory until walked.
- No push, merge, release or deployment. Full inventory is still 2 finalized / 355 pending at this checkpoint. Rebase integration is a prerequisite, not completion of the QA goal.

## Post-rebase results

Rebase completed without conflicts at `fa63698f478e0c942f6142f2cb194b6fc85bc7cd`; `rebase-range-diff.txt` reports all five local commits unchanged (`=`). Backup ref `backup-rebase-qa-execution-unblock-20260910-234235` preserves the checkpoint before rewriting. Local fix mappings are `4f5ae5298` → `609981b51`, `f32f50a9c` → `ad2571be9`, `ea8f4c3f3` → `790039e93`, and `e5a6b2cea` → `dcb9d25a4`; QA checkpoint `ffbfa1c82` → `fa63698f4`. Historical evidence retains its original commit IDs.

Go and root Turbo Web builds passed. The same isolated daemon restarted with schema 109. `rebase-migration-proof.json` verifies integrity and exact selected stable fields for the pre-existing 22 sessions, 2 workspaces, 1 profile, 2 tasks, and 1 task run. Public listing excludes 9 internal-role sessions by contract; they remain in storage. Backups comprise 23 SQLite databases and the separate bbolt client-state store.

The four targeted daemon integration tests passed with race detection: input-only role scoping, measured delivered catalogs, Dream opt-in combinations, and live scoped Dream overrides (`rebase-targeted-integration.txt`). These use protocol fixtures; the separate real Cursor canary completed two turns with `cursor/grok-4.6/high/fast`. Native Heartbeat matches the pre-rebase valid missing-policy state and digest. Native Memory health agrees with CLI/HTTP/UDS: memory enabled, Dream disabled. The terminal journal independently confirms the agent-owned identity command exited 0 with 81 output bytes; the transcript contains the expected session/agent values. Session stop was verified. `rebase-live-proof.json` records the evidence boundaries.

Web smoke on the rebuilt application showed the empty attention popover with disabled Clear all, preserved historical sessions and Vault state, the new Cursor runtime selection, and ordered thought/tool/thought content within an expanded turn, with reasoning disclosure operable. Screenshots were visually inspected. This is not full acknowledgement, hidden-tab title, or remote Tailnet acceptance.

### Canonical suite reconciliation

The initial required gate passed Go lanes but found six Web test failures in three existing suites. Inspection against the merged contract identified stale fixtures and assertions, without a production regression:

- `session-thread.test.tsx`: settled reasoning auto-collapses, so open its own disclosure before asserting rendered Markdown; keep DOM identity/order/focus assertions. The streaming isolation fixture now assigns the settled answer a prior turn ID because same-turn text chunks intentionally concatenate exactly after `0e820dc0e`.
- `session-timeline.logic.test.ts`: the outer interrupted turn already owns the summary; require no second nested summary while preserving ordered reasoning/tool entries, interrupted state, outer open state, and visible final text.
- `desktop-menubar.test.tsx`: provide the real QueryClient context required by the new notification-selection hook; retain the pending-scope aria-disabled assertion.

Owning layers are transcript rendering/derivation and desktop menubar wiring. No new suite or weakened product contract was introduced. Focused root Turbo execution passed all 188 tests (`rebase-web-focused.txt`). Required gate retry is recorded in `rebase-gate-retest.txt`; its final result is recorded below when complete.

Cross-surface impact of these follow-up edits: not applicable — test fixtures and evidence only; no production behavior, native tool, hook/config, data-isolation, official skill, or public contract changed.

Final local gate passed: all affected lanes green; Web 771 test files / 7205 tests, typecheck and lint passed (`rebase-gate-retest.txt`). No CI/push was requested or performed.

Pre-commit formatting/lint ran explicitly with `lint-staged --no-stash`, and commitlint passed. Automatic Husky re-execution is disabled solely because its default lint-staged invocation uses Git stash, which the user prohibits; the checks themselves were completed.
