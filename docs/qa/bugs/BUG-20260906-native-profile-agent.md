# BUG-20260906-native-profile-agent: Native tool policy cannot find a profile-only session agent

- **Status:** fixed — native clarify / Web decision re-walk passed
- **Impact:** Task-Blocked
- **Severity:** Major · **Priority:** P1
- **Persona:** Théo · **Scenarios:** RT-019, RT-session-native-stop
- **Found:** 2026-09-06 · **Report:** docs/qa/reports/2026-09-06-sessions-stability.md

A native clarify call from the integrated lab's history-operator session returned tool_backend_failed before invoking the broker. Temporary build overlays isolated the underlying error to the registry policy resolver: agent not available in workspace. The agent exists in the session's default project profile; policy resolution loaded the unprofiled workspace. The production fix resolves that workspace with the caller's resolved profile and injects the existing profile-name owner at boot. It preserves agent policy narrowing and refuses an agent absent from that profile.

Invariant: native tools apply the agent and configuration from the session profile, without fallback to a different profile. Owner/canonical suite: daemon TestDaemonNativeRuntimePolicyResolver in native_tools_test.go. Default-profile reproduction failed before repair; the complete suite with default, named and unavailable-profile cases passed with race (5.604s). Evidence: .cache/sessions-qa-native-profile-red.log, -final.log, -shape.log; .cache/sessions-qa-final-repairs-lint-final.log. No test was weakened.

Real re-walk used the normal binary, with all diagnostic overlays removed, on daemon PID78720. While Watch held the active turn, native compozy__clarify opened the Web Question dock and waiting-for-input badge. A coordinate click on Staging returned status completed / choice 0 / fallback false, replaced the dock with the durable answered marker, and restored running state without browser errors. Integrated lab evidence: clarification-profile-answer-result.json; clarification-dock-pending.json/png; clarification-dock-answered.json/png. Earlier attempts on an idle/stopped view or left unanswered were canceled; those do not count as successful answers.
