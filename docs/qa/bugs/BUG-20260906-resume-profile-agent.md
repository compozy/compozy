# BUG-20260906-resume-profile-agent: A stopped session cannot resume its project-profile agent

- **Status:** fixed — public same-session resume passed
- **Impact:** Task-Blocked
- **Severity:** Major · **Priority:** P1
- **Persona:** Théo · **Journey:** J-14 read and continue a transcript
- **Scenarios:** RT-session-context-rebuild, RT-session-prompt-cancel
- **Found:** 2026-09-06 · **Report:** docs/qa/reports/2026-09-06-sessions-stability.md

After restarting the isolated daemon, the 3,023-entry Incident archive remains readable but a new public prompt fails with `agent not available in workspace: history-operator`. The agent exists in the workspace's default profile and public agent info resolves it. Initial resume validation used the unprofiled workspace resolver, while creation and actual resume preparation used the persisted profile.

The repair reuses resolveStoredSessionWorkspace at the validation boundary. The existing TestValidateInfrastructure suite verifies agents available only in the persisted default and named profiles. Both cases fail before the production repair and pass afterward with race; no fallback to another profile is added.

Evidence: integrated lab logs/navigation-live-prompt.log and navigation-live-retry.log; .cache/sessions-qa-resume-profile-red.log and .cache/sessions-qa-resume-profile-green.log (3.867s). Public same-session resume passed after daemon PID23345 restarted: session sess-5f467905b4a62303 reports lifecycle_state=active, state=prompting, active_prompt=true and agent_name=history-operator; its new assistant text appears in the real browser. Evidence: integrated lab evidence/navigation-status-resumed.json and logs/navigation-live-verified.log. An earlier attempted fixture prompt omitted the exact fixture wording and was rejected by the controlled driver after successful resume; the corrected public prompt was accepted.
