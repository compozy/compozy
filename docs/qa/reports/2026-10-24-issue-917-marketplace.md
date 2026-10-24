# Linear catalog compatibility

Scope: issue #917, isolated CompozyOS 0.4.0 runtime and current unchanged Web UI.
Persona: operator choosing the Linear integration from Marketplace.

| Journey | Status | Evidence |
| --- | --- | --- |
| Discover incompatible Linear, inspect it, and observe disabled Install | Blocked (needs human verify) | Initial setup model selection was denied by automatic approval review. |
| Confirm the same entry through the public API | Pass | `installable=false`, required `0.5.0`, running `0.4.0`. |

The lab uses a published Linear package requiring 0.5.0. It has no operator credentials.
Existing integration evidence covers CLI/HTTP/UDS and installation of the repaired official archive.

## Session debrief

The public API returned the incompatible entry successfully. Chrome reached the fresh-home setup
screen before Marketplace. Opening its model selector was rejected by automatic approval review
because model selection could change configuration or start authentication. No alternate route was
used to bypass that rejection. Rendered Marketplace behavior remains unverified in this run.

## Automated evidence

- Catalog publisher and marketplace suites passed with `-race`.
- Shared Marketplace API suite passed with `-race`, including legacy incompatible detail reads.
- Three isolated daemon distribution scenarios passed: stamped-version compatibility and repaired
  Linear installation, v3 publication, and curated updates/digest rejection.
- Focused Go lint (repository-pinned v2.13.1) reported zero issues.

## Limitation

No OAuth authorization was attempted; the issue occurs before authorization. Completing the browser
leg requires approval to select a model in the disposable lab's initial setup. The user's daemon and
credentials were not changed. PR readiness remains subject to current-head CI and external reviews.

## Authorized browser re-walk

The orchestrator clarified that model selection and configuration in this disposable lab were
authorized QA. The second walk completed setup with the lab's own project directory. No provider
login, credential copying, or agent session was performed.

The real Marketplace list and Linear detail both render **Blocked**, replacing the Install action.
The detail badge's rendered `title` is:

```text
extension: incompatible manifest: current daemon version "0.4.0" does not satisfy min_compozy_version "0.5.0"
```

The existing UI expresses the blocker as a status and tooltip, not a disabled button. This re-walk
supersedes the earlier visual blocker. Desktop and narrow captures were inspected at their original
sizes:

- `docs/qa/evidence/2026-10-24-issue-917-marketplace/marketplace-blocked.png` — 1440 × 900 list.
- `docs/qa/evidence/2026-10-24-issue-917-marketplace/linear-blocked.png` — 1440 × 900 detail.
- `docs/qa/evidence/2026-10-24-issue-917-marketplace/linear-blocked-narrow.png` — 320 × 800 detail.

Evidence synchronization encountered an external tooling limitation: Skeeper cannot open this
worktree (`core.repositoryformatversion does not support extension: worktreeconfig`). GitHub's
browser upload also requires the browser extension's file-URL permission. The PNG files are retained
locally; publication is being coordinated separately. These are evidence-upload limitations, not a
failed Marketplace render. Current-head CI and review remediation remain separate delivery gates.

The re-walk teardown reported `clean: true`, with no surviving owned processes. The strict targeted
QA evidence audit passed with zero blockers and warnings; its local validation record explicitly
uses the user's focused-check override rather than claiming a full local gate.
