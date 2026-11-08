# Always-on check-in tools QA walk (#1357, S4 phase B)

Walk of `MS-always-on-check-in-sources` §8 ("Check-in tools and actions") and
`MS-web-stays-on-reports` ("Check-in picks, tool calls and activity") on `aocit/qa`, starting at
the integrated head `88adc0207` and ending at `2a49860ec`. Spec:
`.compozy/tasks/always-on-check-in-tools/`.

## Lab

- Isolated lab from `eng-qa-bootstrap` (`--profile targeted`, surfaces cli/api/web/runtime/provider):
  own `COMPOZY_HOME`, HTTP port 50744, UDS socket and copied Codex login; Claude ran through its
  native sign-in (`home_policy = operator`). The operator's daemon, `~/.compozy` and desktop app
  were not touched.
- Daemon built from the worktree and rebuilt after every fix. Web: Vite from the worktree on
  127.0.0.1:50746 proxied to the lab daemon only, driven with Playwright (Chromium).
- Fixture MCP server (Go SDK, streamable HTTP, 127.0.0.1:50745) declared in the agent's
  operator-authored `AGENT.md`: `list_items` (`readOnlyHint`), `get_item` (no annotations),
  `add_comment` (no annotations), `delete_item` (`readOnlyHint` + `destructiveHint`). It records
  every call, so "the fixture received exactly one call" is checked against its counter.
- Always-on agent `issue-watch` with an approved script source `ticks` that returns fresh items on
  every poll, so each real `instance check` passes the gate.
- Lab configuration: `[tools.policy] external_default = "enabled"`. With the default `disabled`,
  MCP sources are `source_disabled` for every session, so an agent's MCP read tools are invisible to
  check-ins too. The scenario now states this prerequisite.
- Providers: Claude (`claude-opus-5-5`, check-ins in `plan`) for journeys 1–5 and the Web walk;
  Codex (check-ins in `read-only`) for the approved-write and kill-switch re-walks.
- Evidence (command output, JSON reads, screenshots, Playwright logs) is in the worktree's
  gitignored `.compozy/tasks/always-on-check-in-tools/qa-walk/`.

## Results

| Journey | Scenario steps | Result |
| --- | --- | --- |
| 1. Reads through the fixture read tool, then proposes | §8.1 | Pass after fixes 1, 2, 3, 4 |
| 2. Unannotated and destructive tools | §8.2 | Pass (not offered to the check-in; see notes) |
| 3. Approved `allow_writes` write and activity report | §8.4, §8.5, §8.9 | Pass on Codex; Claude `plan` declines approved writes (finding F-1) |
| 4. Authority unapproved after an agent edit, then approve | §8.10 | Pass |
| 5. Auto-accept, waiting proposals, operator picks, repeat 409 | §8.11–§8.13 | Pass after fixes 5, 6 |
| 6. Kill switch | §8.14 | Pass |
| 7. Web report states | Web U1–U5 | Pass after fixes 7, 8, 9, 10 |
| Live Linear search | §8 closing note | Blocked: needs the operator's Linear MCP sign-in |

Key observations:

- Journey 1: occurrence row `mcp__fixture__list_items` `access: read`, `decision: allowed`,
  `outcome: ok`, `args_preview: {}`, no result body. Proposals quote fixture data, and the report
  reads `tool_calls.total: 1`. `tool info` reads `access: read`. A check with no new item is
  `skipped` (`digest_empty`) with no tool calls.
- Journey 2: `get_item` and `delete_item` read `access: write` (`delete_item` with
  `destructive: true`). The check-in's projection marks both `check_in_write_not_allowed` and
  hides them. The model reported that only `list_items` was offered, so no audit row exists. The
  main session lists all three writes as `approval_required`. Denial rows for a direct dispatch
  remain covered by IT-006.
- Journey 3: with the main session at `approve-edits`, `add_comment` is not offered (the
  `RunsUnattended` check runs at listing) and `session interactions` stays empty. After an
  agent-scope "always allow" grant, Codex check-ins called it: `allowed`, `outcome: ok`,
  `effect: done`, exactly one fixture call per check-in. The result is `quiet` with
  `quiet_streak: 0`, plus an `activity` report (`writes_done: 1`).
- Journey 4: an agent-credential `PUT …/heartbeat` adding actions leaves `authority_status:
  unapproved`. One "Check-in changes need your approval" notification fires, not again on the next
  check-in. Check-ins run reads only, offer no actions, and every proposal waits. Agent approval is
  refused over CLI and HTTP (`heartbeat_authority_operator_only`, 403). An operator save answered
  `n` stays unapproved, a stale `--sha256` fails `heartbeat_authority_changed`, and the operator
  approve succeeds.
- Journey 5: `report-harness-bug` was auto-picked (`actor: auto_accept`,
  `auto_accept_status: picked`, delivered). The main session received exactly one
  `synthetic_reentry` (`proposal_auto_accept`) carrying the action instructions and the fenced
  proposal. A second proposal waited. The real model never emitted the undeclared `triage-docs`
  id, so `check_in_action_unknown` stays covered by `TestTriageReplyOutcome`. With
  `auto_accept_per_day = 1`, the next auto proposal waited with `limit`. CLI pick `2` succeeded,
  and a repeat returned `proposal_already_picked` (HTTP 409). An agent pick returned
  `proposal_pick_operator_only` over HTTP (403) and, after fix 6, over the CLI.
- Journey 6: `check_in_tools = "disabled"` started no hosted bind, wrote no audit rows, sent no
  fixture calls and recorded `tools_status: disabled`. The setting was then restored.
- Journey 7: the report shows the "Auto-accepted" chip with "Report a harness bug" and no Pick,
  "Daily auto-accept limit reached" with a working Pick, and the "Check-in used 1 tool"
  disclosure (`aria-expanded` false→true, listing "list items · fixture · Read · Allowed · 39 ms").
  A Web pick sent `POST …/pick` `{"proposal":1}` with no chat message, folded to "You picked …",
  and queued one prompt. A CLI pick refreshed the open tab within the same second, so the
  stale-tab 409 was not reachable by hand (`TestProposalPick*` owns it). Picking on a paused
  instance showed "The agent is paused, so it can't take this now. (instance_not_active)" and
  queued nothing. After resume, the same pick succeeded. The activity card read "The check-in made
  1 change / Here's what it did" with the open list ("add comment · fixture · Change · Allowed")
  and no Pick. The Heartbeat history keeps tool rows apart and reads "Tools off" for the
  kill-switch run.

## Bugs fixed (each with a focused regression test that failed before the fix)

1. `4da259aef`: an MCP tool annotated `readOnlyHint` + `destructiveHint` failed
   `Descriptor.Validate`, so every session projecting that server failed to start
   (`registry_policy_denied`). Test: `TestMCPCheckInAccess`.
2. `c3e755717`: always-on main and check-in sessions denied every agent-declared MCP tool
   (`session_denied`) because the instance ceiling never admitted a server's tools. Test: IT-006
   `TestCheckInToolAuthorityIntegration` now uses the ceiling shape that admission records.
3. `cab3cda15`: `CheckInOccurrence` omitted proposal counts. Test: `TestInstanceProposalPicks`.
4. `c15a927c1`: `GET …/check-ins?occurrence=` dropped proposal counts. Test:
   `TestAgentInstanceCheckIns`.
5. `ee9ff2851`: the heartbeat policy read always returned `actions: null`. Test:
   `TestHeartbeatAuthorityHTTP`.
6. `32628460c`: a CLI pick from an agent session succeeded as the operator. Test:
   `TestAgentInstanceCommands`.
7. `0f89127b3`: the Web history counted reads as changes ("1 tool · 1 change"). Test:
   `agent-editor-files.test.ts`.
8. `a0c7e5b44`: the away digest counted activity reports as "went wrong" and folded them as "Still
   in the same conversation". Test: `session-away.test.ts`.
9. `12400691f`: Heartbeat history called a check-in that wrote and notified "All quiet, so it
   didn't bother you". Test: `agent-editor-files.test.ts`.
10. `2a49860ec`: the `allow_writes` selector warning listed tools under the workspace identity ID,
    so every MCP selector read "matches no current write tool". Test:
    `TestHeartbeatAuthorityHTTP`.

## Open findings

- **F-1 (decision): Claude `plan` check-ins decline approved writes.** With plain guidance, all
  3 Claude check-ins refused the offered, approved `add_comment`, citing plan mode ("overrides
  other instructions"). That includes 2 runs with an extra check-in prompt sentence saying
  approved writes are not blocked; that change was not kept. When the guidance itself insisted, 2
  of 3 runs called it. Codex `read-only` ran approved writes 3 of 3 times. ADR-001 chose `plan` for
  Claude check-ins, so a fix needs an operator decision (for example, a Claude mode that keeps
  built-ins read-only but does not instruct the model to refuse every write).
- Copy: the stored activity detail reads "Made 1 changes; …". The notification and Web titles
  use the correct singular.
- Not walked in this pass: §8.3 (`read_only_tools` override), §8.6 (writable set and interaction
  tools), §8.7 (caps), §8.8 (no main session) and the `queue_full` / `no_main` / cancelled parts
  of §8.12, all covered by the S1/S2/F1 integration suites. Also not walked: the live Linear
  search, sections 1–7, and the older Web sections.
- The strict QA auditor reported FAIL on bookkeeping only: C7 (journey-log format not used), C12
  (lab verification report) and C14 (local gate evidence; the host-load rule leaves gates to CI).
