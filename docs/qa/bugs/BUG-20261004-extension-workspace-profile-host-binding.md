# BUG-20261004-extension-workspace-profile-host-binding: Profile extensions lose their Host API workspace binding

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-operate-profiles, approve a command that creates an editorial task
- **Scenarios:** ET-profile-approval-owner-resume
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction and evidence

Install the local Studio Capture SDK extension into the Studio Operations workspace under
resume-editorial, with declared tasks/create permission. Invoke Capture editorial task and approve
the pending command. The decision succeeds, but execution fails and the task catalog stays empty.
The real Web replay is recorded in palette-decision-replay2-ended-summary.json. A subsequent
engineering invocation, apr_06499c75-66e6-4d26-b75f-f50aea346bb3, logs the Host API RPC data:
task: permission denied. Evidence: palette-approval-studio-reason-{invoke,approve,logs}.json under
docs/qa/evidence/2026-10-02-untested/.

## Root cause and repair

The extension manager correctly launches a process with a workspace_profile resource scope. The
shared Host API workspace resolver recognizes only workspace, so it treats the process as unbound.
Task actor derivation loses its workspace and the task service correctly refuses the write. The
repair recognizes the validated compound scope at that shared boundary. Domain calls inherit its
workspace, and explicit global or foreign workspace requests remain refused. Resource methods keep
their complete workspace_profile scope for authorization by the resource kernel.

The existing Host API handler suite owns this invariant and uses real SQLite for the created task
and resources. The new task case fails before repair; after repair, owned task creation, foreign and
global refusals, profile resource snapshots, and foreign profile refusal pass with the race detector.
The first resource test used an invalid tool identifier; its corrected fixture is retained separately
from the production regression. Receipts: extension-workspace-profile-binding-red.json,
extension-workspace-profile-binding-green.json, and extension-workspace-profile-binding-valid-resource.json.
The third fresh Dora replay verifies the repair through the real SDK subprocess and daemon:
Web approval creates task-4d8ec399d22f06ec, and a post-switch CLI approval creates
task-db6cd3ede33d0beb. Both retain workspace ws_7af64cef6bc02b2b and resume-editorial. Independent
draft-inclusive UDS reads, HTTP detail and a refreshed Web deep link confirm their identity.
Evidence: palette-decision-replay3-ended-summary.json. The affected gate passes Go lint, race tests
and 6,927 Web tests (palette-host-binding-delivery-gate.json). The convention checker reports no new
findings against HEAD; existing unrelated findings remain in extension-host-binding-test-conventions-comparison.json.

Fix commit: `073b1705b`.
