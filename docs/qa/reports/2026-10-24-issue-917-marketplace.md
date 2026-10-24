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
