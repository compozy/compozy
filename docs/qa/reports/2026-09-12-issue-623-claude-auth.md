# Issue 623: Claude structured authentication probe

Scope: Dora administers provider authentication through the existing
CH-provider-auth-surfaces-dora charter, restricted to RT-026 and its local CLI
equivalent. Existing operator credentials remain unchanged. No Compozy sessions,
worktrees, or Goals are created by this execution assignment.

| Journey | Status | Evidence |
| --- | --- | --- |
| Native Claude JSON status through local CLI | Pass | `local-claude.json`: authenticated, exit 0, verdict only |
| Native Claude JSON status through daemon HTTP/UDS | Pass | `http-claude.json`, `remote-claude.json`: authenticated, exit 0, verdict only |
| Unknown provider adjacent canary | Pass | Unconfigured acpmock returned provider_not_installed without probing |

The canonical classifier suite reproduces the reported unknown state and an
identity-bearing diagnostic before the fix. It also exposes false authentication
from incidental JSON text and negative text on exit zero. The provider suite now
passes with the race detector. Classification belongs to `classify_test.go`;
subprocess output minimization belongs to `runner_test.go`; the existing API
failure case owns privacy at the injected-runner response boundary.

Platform boundary: the available native Claude CLI is 2.1.266 on macOS; the
reporter's exact Linux/2.1.269 environment is unavailable. Malformed and logged-out
cases are deterministic canonical tests, without logging out the operator.

Dora's local and daemon reads agree without a translation shim. A fresh native
status read confirmed the same true boolean. The generated support bundle included
11 artifacts, including providers and diagnostics; scanning each artifact for the
current native email, organization ID, and organization name found zero matches.
Only the field names and zero-match summary were retained as evidence; no raw
native identity payload was captured. Lab teardown finished with `clean: true` and
no survivors.

The isolated lab's scratch evidence includes `provider-attempt.json`,
`journey-log.jsonl`, `support-privacy-summary.json`, and `teardown.json`. The
executor's private report retains their exact paths. The structured-error rejection
added after this walk affects an unobserved error case, covered by the canonical
classifier suite in CI; it does not invalidate the unchanged successful native
response path. The operator's login state was never modified.

Validation boundary: initial provider race tests, focused API tests, and the real
app build passed before the user changed the execution policy. Earlier local gate
attempts exposed formatting and constant-lint findings; those were repaired. The
user then explicitly required GitHub CI-only delivery gates. The executor cancelled
its own remaining local gate and used per-command `HUSKY=0` for commit hooks. No
local gate completion is claimed. The QA metadata auditor's local-gate requirement
(C14) is superseded by that explicit user instruction; its missing report finding
(C12) is resolved by this report and the lab-side copy.

Final delivery is pending the current-head GitHub CI and both external reviewers.
The historical Daytona leg of RT-026 remains outside this local-provider slice.
No first-prompt replay was performed because the assignment prohibits creating
Compozy sessions; existing integration/E2E workflows own broader session evidence.

PR #632 review remediation: Greptile finding 3995000662 exposed two untested
prefix cases. The shared projection now preserves bracket-labeled text probes
and locates a JSON object after a warning. It suppresses the surrounding identity
payload, preserves classified prefix errors, and keeps malformed mixed output
unknown. The existing classifier table covers both reported forms and adjacent
true/malformed/error-prefixed variants. These cases run in GitHub CI under the
execution override; the unchanged native true response retains the earlier live
probe evidence. CodeRabbit completed the initial source review without actionable
comments; its generic docstring-coverage warning conflicts with the repository's
explicit rule against restating obvious private helper behavior in comments.
