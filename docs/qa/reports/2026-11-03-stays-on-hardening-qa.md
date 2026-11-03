# Stays-on hardening QA walk (integrated F1–F3 + backend)

- Build: branch `soh/qa` at `583415ef5` (integrated Web F1–F3 plus review fixes on the backend with `origin/main` merged).
- Lab: isolated `eng-qa-bootstrap` targeted lab (`cli,api,web,runtime`), its own `COMPOZY_HOME`, HTTP port and `/tmp` socket.
  The daemon served the built `web/dist`. Data came from `scripts/demo-seed`.
- Provider: acpmock driver routed through `[providers.claude] command`; fixture in `artifacts/2026-11-03-stays-on-hardening-qa/acpmock-fixture.json`.
  Lab config: `agents.stays_on.unattended_turn_limit = 3`, `approval_timeout = "2m"`.
- State was driven through the public CLI, HTTP, UDS and native tools: monitor-timer chains (brake), `compozy__agent_skill_propose`,
  a signed workspace webhook to a `--target-instance` trigger, a held peer message, permission expiry, and daemon restarts.
- Web screens were captured with Playwright in light theme at 1440 wide (R2 `compozy-pr-evidence/stays-on-hardening/583415ef5/`).
- Durable evidence: `docs/qa/reports/artifacts/2026-11-03-stays-on-hardening-qa/`.

## Matrix

| Scenario | Verdict | Notes |
| --- | --- | --- |
| ET-native-tool-approval-grants | Partial | Native, HTTP and UDS grant mutation from stays-on mains is denied (403 `tool_approval_grants_denied`), and the operator is unaffected. The nested-child and on-demand callers were not walked. |
| ET-skill-activation-gates | Pass | Pending drafts are not offered, an agent decision returns `skill_review_operator_only`, a stale digest is rejected, an approved skill is offered in a fresh session, and a rejection persists. |
| ET-web-agent-detail-tab-parity | Partial | Only the five-level vocabulary was checked (S5/S15). |
| ET-web-session-message-card | Partial | The held row on the target passes. The Sent card was not verified with the acpmock sender. |
| ET-web-session-permission-dock | Partial | Expiry gives `timeout`, and an identical repeat gives `repeat_denied` immediately. The hosted-tool denial text was not walked. |
| MS-web-agent-create-simple-advanced | Skipped | Not walked this pass. |
| MS-web-agent-permissions | Partial | Levels, approval reach and exceptions save and read back. Auto unavailable is never shown (BUG-20261103-reviewer-mode-omitted, fixed by 2f5647d5b and pending a re-walk). |
| MS-web-stays-on-placement | Fail | The brake (3, 0 and -1) and the S1/S7/S11 copy pass. Resume all resumes only one agent (BUG-20261103-resume-all-rate-limited). |
| MS-web-stays-on-reports | Fail | S2, S3 and S9 pass. The S8 reason leaks `session/request_permission`, and the S13 proposal prints raw untrusted webhook JSON. |
| RT-session-message-origin | Partial | A held peer message survives a restart and is delivered once on resume. Rotation was not walked. |
| RT-session-monitor-wake | Partial | Pause and restart, next-wake projection, the doctrine section, and UT-028 pass. Other B3 legs were not walked. |
| TA-automation-last-run-agent | Partial | Event delivery, proposal sources and checks-disabled pass. An instance-ID target is accepted at create time and then 404s on delivery (BUG-20261103-target-instance-id-accepted). |
| TA-web-automation-editor | Partial | The S10 copy and the schedule lock pass. Agent selection was not confirmed in the automated walk. |

## Live receipts

- **IT-008:** `compozy agent instance show --json` and `GET /api/agent-instances/:id` return the same 26 fields for a paused instance (`it008-cli.json` and `it008-http.json` diff empty).
- **UT-028:** triage-desk had HEARTBEAT `quiet_windows` set to 05:00–07:30 UTC and a timer due at 06:02:47. At 06:02:47 it ran as a `[COMPOZY MONITOR - NOT USER INPUT]` turn with `held_wakes: 0`, so the quiet window did not hold the wake (`ut028-triage-events.txt`, `ut028-heartbeat-status.json`).

## Screens (R2 `https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/stays-on-hardening/583415ef5/`)

Every object returned 200 `image/png`.

| Screen | File | Verdict | Difference from the draft |
| --- | --- | --- | --- |
| S1/S2 paused | `01-s1-s2-pause-report-paused.png`, `01b-s1-hover-paused.png` | Pass | Live adds the waiting-count line. |
| S2 resumed | `02-s2-pause-report-resumed.png` | Pass | "Delivering N things that were waiting." is missing. |
| S3 away digest | `03-s3-away-digest.png` | Pass | Matches. |
| S4 Resume all | `04-s4-resume-all.png` | Fail (action) | The popover structure matches. |
| S5 permissions | `05-s5-permissions-levels.png` | Partial | The "has no AI reviewer" alert never shows. |
| S6 approval reach | `06-s6-approval-reach.png` | Pass | — |
| S7 next wake | `07-s7-hover-next-wake.png` | Pass | Captured on Inbox. |
| S8 expired approval | `08-s8-expired-approval.png` | Fail (copy) | The draft names the tool and action; live shows `session/request_permission`. |
| S9 card / Read / Source | `09a-…`, `09b-…`, `09c-…` | Pass | Matches. |
| S10 trigger target | `10-s10-trigger-target.png`, `10b-…-locked.png` | Partial | The picked agent did not stick, so the scope picker is not shown. |
| S11 usage | `11-s11-usage.png` | Pass | Paper cut: clipped project name. |
| S12 held message | `12-s12-held-message.png`, `12b-s12-sent-card.png` | Pass (target) | Paper cuts: provider avatar for the sender, raw "SY synthetic" label. |
| S13 event proposal | `13-s13-event-proposal.png` | Fail (copy) | Raw JSON summary; the draft footer "Nothing happens until you pick one." is missing. |
| S15 Settings levels | `15-s15-settings-levels.png` | Pass | Settings has no icons and uses a check mark. |

## Findings

- BUG-20261103-resume-all-rate-limited (Blocks-Completion): parallel resumes get 429 with no UI error. A fix is in progress on F1.
- BUG-20261103-reviewer-mode-omitted (Trust-Damage): fixed by 2f5647d5b after this base; pending a re-walk.
- BUG-20261103-target-instance-id-accepted (Blocks-Completion): a fix is in progress on F2.
- S8 copy leaks the ACP method `session/request_permission` and shows the session title as the subtitle. The repeat refusal is labelled "Not allowed by you".
- S13 prints the raw untrusted webhook payload as the event summary.
- Paper cuts:
  - A missing peer name returns `agent_instance_not_found` with the message "Internal Server Error".
  - The automation picker shows agent names instead of labels.
  - A paused agent can sort under Paused in the overflow instead of Needs you.

## Debrief

The integrated build holds the backend contract: brake trips, held wakes and messages surviving a restart, skill review authority, grant denial,
event-to-proposal delivery, and repeat-expiry refusal all behaved as specified on real SQLite with a real ACP process. Web regressions cluster on
action wiring (Resume all) and on report copy that passes raw backend values through (S8 action, S13 summary). The re-walk on the final head covers S4, S5 and
the trigger scenario.

## Final status

13 scenarios: 1 pass, 2 fail, 9 partial and 1 skipped. Three bugs were filed (one fixed and pending a re-walk, two in progress), plus two copy failures
and the paper cuts listed above. No CI or full suites were run locally, per repository policy.
