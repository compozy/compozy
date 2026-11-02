# QA: spec-cycle simplification final walk

Date: 2026-11-02. Build: `feat/scs-qa` from `ce8cdcc37`, with the fixes below. Provider: Claude Code (`claude-sonnet-5-5`, `claude-haiku-5-5`), real runs on a two-file fixture repository. Lab: isolated home, ports and socket; teardown clean. Host load stayed between 20 and 30 for the whole walk.

## Result

| Scenario | Status | Summary |
| --- | --- | --- |
| `RT-upgrade-spec-cycle-hard-cut` | partial | Boot was broken for real v0.4.8 homes and is fixed. Steps 1-4 and 6-9 pass. Step 5 leaves one open finding. |
| `ET-spec-authoring-slices-review` | partial | Slices, declined review and one clean round pass. Blocker, second round and interruption were not run. |
| `TA-spec-conductor-delivery` | partial | Delivered end to end. Cases 2-6 pass, case 1 ran one send-back, case 7 was not run. |
| `TA-orchestrator-small-work` | blocked-decision | Works only with an operator approving tools. Needs a decision. Triage passes after a fix. |
| `TA-merge-train-gate` | skipped | No authorized sandbox. See below. |
| Web evidence | pass | See screenshots. |

The strict evidence audit reports FAIL with 3 blockers, all bookkeeping: `journey-log.jsonl` was not populated (C7), no lab verification file (C12), and no local gate evidence (C14), because gates are CI-only. This report is the evidence record.

## Defects fixed

| Commit | Defect | Proof |
| --- | --- | --- |
| `2fa8edf3c` | Blocker: migration 00172 dropped a named unique index that homes bootstrapped from definitions never had. Every such home, including a real v0.4.8 install, failed at boot with `no such index`. The defect came in with the stays-on work and is already on `main`, unreleased. | Real v0.4.8 home failed, then booted to version 172 on the same files. `TestGlobalAgentSnapshotProfileMigrationConvergesUniqueKeyShapes` covers both shapes. |
| `5ed24fec1` | Blocker: children of a stays-on instance inherited a ceiling of the workspace root only, so a child in a managed worktree failed with `instance execution path exceeds permission ceiling`. Managed worktrees sit outside the root by default, so every orchestrator dispatch failed. | The same dispatch then ran and committed. `TestInstanceCeilingManagedWorktree`. |
| `d732bffc3` | The triage prompt did not state the 120 and 500 character limits its parser enforces. A real 1,251 character proposal failed with `triage_output_invalid`. | The next real check returned `reported`. `TestTriagePromptStatesProposalLimits`. |

`2fa8edf3c` edits migration 00172 and refreshes `atlas.sum`, which the migration rules normally forbid. An appended migration cannot repair this, because 00172 fails first and no release contains it. A home that already applied 00172 keeps its version, and its shape matches the rewritten result. The migration runs without a transaction, like every `foreign_keys = off` rebuild here. Separately, `TestMarketplaceCatalogSourceMigration` still pinned 19 catalog entries from before the Batuta removal, so that commit also moves it to 18.

## Open findings

1. **Decision needed: stays-on children cannot write unattended.** `applyInstanceCeiling` lowers `approve-all` to `approve-reads` for an instance and its children. The orchestrator stopped at its first compound Bash and at `skill_view`, and its executor stopped at its first file write with `waiting-for-auth`. Nothing tells the operator through the orchestrator, because it is waiting too. The golden path in `_dx.md` ("I'll report when it is verified") holds only if someone approves each step: this walk approved at least 17 requests by script. The ceiling is deliberate, so it was left alone. `merge-train` declares `permissions: approve-all` and would be lowered the same way.
2. **Step 5: a rerun that hits a removed step never settles.** A run active at upgrade continued from its snapshot and ended `done`, because no removed step remained on its path. Forcing the path with `loop rerun --from-node load_tasks` failed the node with `unknown_action_kind`, in isolation, while other runs and sessions continued. Both forced runs then stayed `running` after three quarantined generations, one of them for over 20 minutes. An active run that reaches the step on its own was not produced, so this may be specific to rerun.
3. **Errors are less clear than the spec asks.** `Clear error` holds for `loop definition not found` and `agent not available in workspace`. These are weaker: a removed-step attempt reports `The action could not complete its runtime contract`, `batuta-deliver` reports `unknown_action_kind` without the tool name, prompting a `review_fixer` session returns HTTP 500 with `skills: agent not found`, and `session resume` says only `session not attachable`.
4. **Agent memory is unverified.** The orchestrator kept a stated preference out of the work log, but no memory file or memory tool call was found.
5. **Load sensitivity, unrelated to this branch.** At load above 20, a 30-second hosted MCP bind nonce expired for sessions starting right after boot, and a provider `session/load` hit its deadline once. The lab raised the nonce to 180 seconds. The stuck session recovered on the next prompt.
6. **Minor.** `/agents/orchestrator/check-ins` without `?workspace=` under All projects shows `Couldn't load this page`. An agent dropped on disk needs a daemon restart before `agent instance create`, otherwise it fails with `agent memory definition unavailable`. `daemon stop` left the lab daemon running at teardown, and the sweep stopped it. `catalog/artifacts/batuta-v0.1.0-beta.3.tar.gz` is still tracked though the spec lists `catalog/artifacts` for Batuta removal.

## Walk detail

### RT-upgrade-spec-cycle-hard-cut

The home was prepared by the real v0.4.8 release binary on `darwin/arm64`.

| Step | Result |
| --- | --- |
| 1 Daemon starts | Fail, then pass after `2fa8edf3c`. No manual step. `doctor` reports only unrelated provider auth entries. |
| 2 Removed resources gone | Pass. 0 `ext__spec_cycle` tools, `cy-create-spec` is the one `cy-` skill listed, `spec-cycle` 0.6.0 is `resource`, `review_fixer` and `implement-tasks` are gone. |
| 3 `review_fixer` history | Pass with finding 3. History reads; resume and prompt fail. |
| 4 Automations | Pass. Records are identical. The Loop job fails with `loop definition not found`, the agent job with `agent not available in workspace`, and the control job completes. v0.4.8 refuses a trigger on `implement-tasks`, so the trigger targets `review_fixer`. |
| 5 Override and active run | Override row stays (`iteration_cap = 7`). Run: see finding 2. |
| 6 Same-name workspace Loop | Pass. `review-and-fix` runs to `done` as `workspace`. |
| 7 Stays-on instance | Pass, with a substitute. v0.4.8 has no instances, so the instance was made on this build for a global agent whose definition was removed. State `paused`, reason `definition_missing`. |
| 8 Batuta | Pass. It stays `active`, its agent loads, and only `batuta-deliver` fails. |
| 9 Task hashes | Pass at first boot. They changed later only because the active run's own worker marked its tasks complete. |

The walk restored the pre-upgrade database once, to repeat step 5 after the nonce failure ended the first attempt.

### ET-spec-authoring-slices-review

- `_slices.md` has slice, layer, dependencies, scope and spec references.
- Declined review: no `qa/` directory.
- One reviewer, 1 round: `peer-review-findings-round1-claude.md` has all 5 sections, readiness READY, no later round.
- Answers were supplied up front, so "asks once" was not observed. Not run: blocker with two rounds, operator escalation, interrupted reviewer. Each needs a staged reviewer, and the time went to the blockers above.

### TA-spec-conductor-delivery

Fixture: two slices in a gitignored spec directory, read by absolute path.

| Case | Result |
| --- | --- |
| Delivery | Pass. `6f00b08` and `855abc3` carry `Spec-Slice` trailers; `./test.sh` passes; reviewer approved at the integration head. |
| 1 Send-backs | Partial. One send-back from a monitor wake, no hop error. Five were not forced. |
| 2 Restart before checkpoint | Pass. With the S2 lines removed from `_run.md`, a new conductor took Git history as truth and dispatched nothing. |
| 3 Restart after cleanup | Pass. Slice worktrees removed; nothing was redispatched. |
| 4 Replacement while active | Pass. It reported the owner and took no action. |
| 5 No `_slices.md` | Pass. It proposed slices and waited for acceptance. |
| 6 Reviewer asked to edit | Pass. It refused, and wrote only its findings file. |
| 7 Replacement with a running child | Not run. |

Send-back count: 1, requested by the brief; no worker stopped early. Both implementers and the reviewer made 0 `session_prompt` calls, so they obeyed their prompt over the header hint. The conductor stayed at 0 permission requests. The runtime was confirmed by a child report only for the orchestrator's dispatch (`claude / claude-sonnet-5-5 / low`).

### TA-orchestrator-small-work

| Check | Result |
| --- | --- |
| `[ ]` before child | Pass. The entry preceded the child session. |
| `[~]` with session and worktree | Pass after `5ed24fec1`. |
| `[x]` with evidence | Pass, with operator approvals (finding 1). Commit `a38abed`. |
| Duplicate request | Pass. Reports the closed entry, creates no second executor. |
| Spec-sized request | Pass. Recommends a spec and `spec_conductor`; dispatches nothing. |
| Learned preference | Partial. Stays out of the work log; see finding 4. |
| Kill before `[~]` | Not run as stated. The retry after the ceiling failure reused the entry's worktree and session. |
| Stale `[ ]` entry | Pass after `d732bffc3`. The digest carried `work:wl-20261102-02`, and triage reported a proposal. |
| Unchanged digest | Pass. `quiet`, `duplicate_suppressed`, no model call. |

Because of finding 1, this scenario should run again before `cy-orquestrate-issue` is removed.

### TA-merge-train-gate

Skipped. The agent names `compozy/compozy` in its commands and the lab inherits live `gh` credentials, so a run risks the real remote. A safe run needs a sandbox repository, whose creation is an outward action nobody authorized, plus a copy of the agent pointing at it.

## Screenshots

Hosted in the evidence bucket, each confirmed as `200` and `image/png`:

- Agents list with kinds: https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/spec-cycle-simplify/d732bffc3/01-agents-list.png
- Check-ins with "Work that never got started": https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/spec-cycle-simplify/d732bffc3/02-check-ins.png
- Marketplace without Batuta: https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/spec-cycle-simplify/d732bffc3/03-marketplace-no-batuta.png

The Agents capture is a lab home that also installed Batuta, so it lists `batuta` and `general` beside the 4 spec-cycle agents. The Marketplace capture is a fresh home on this branch's catalog: 18 entries, no Batuta. Until this merges, the live catalog on `main` still lists it, so a home using the default source shows it.

## Not covered here

CI owns `make gate`, E2E and full suites. Local runs were focused: the migration suites, `internal/session` tests for the ceiling and triage, `internal/heartbeat` triage, and `TestMarketplaceCatalogSourceMigration`. Focused lint on the changed lines of `internal/session`, `internal/heartbeat` and `internal/store` reports 0 issues. `TestAgentPreStartHookOpenCodeLaunchOrigin` fails locally on the macOS `/private` path prefix; its assertions concern hook paths outside these changes, though it was not run against the base commit.
