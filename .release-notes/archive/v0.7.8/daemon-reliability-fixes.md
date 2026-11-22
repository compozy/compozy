---
title: The daemon boots with a missing project, stops silent turns on time, and installs updates right after a release
type: fix
---

- **Boot with a renamed or removed project folder.** If a registered project's folder was renamed,
  removed or became unreadable, the daemon could fail to start after a restart or an update, which
  locked you out. It now starts, logs a warning naming the affected project and profile, and leaves
  that project's saved layout as it was. Other projects load normally.
- **Silent agent turns are stopped on schedule.** An ACP agent can report a failed tool call and then
  never answer the prompt. The quiet watchdog now starts measuring silence right away, so it warns at
  `session.supervision.quiet_after` (30 minutes by default) and stops the turn after
  `session.supervision.stop_grace` (10 minutes by default). Before, it waited extra watchdog intervals while the turn still
  looked active. Any new update from the agent clears the warning, and after the stop you can use
  **Resume** to retry.
- **Hosted MCP calls no longer hang forever.** A tool call made without its own deadline could wait
  indefinitely if the daemon stopped answering. Such a call now has a time budget built from the
  longest tool wait (24 hours), the longest approval wait (10 minutes) and `client.request_timeout`
  (30 seconds by default). When it runs out, the call ends with a tool error. Clarifications without
  a timeout and long extension calls keep running as long as the daemon reports progress. Calls with
  their own deadline behave as before.
- **App updates right after a release.** For a few minutes after a release was published, every
  **Update** or **Retry update** click could fail with "The app update could not be installed."
  because the update feed still named the previous version. The desktop app now installs the
  version the daemon planned, straight from its GitHub release, and still checks its digest against
  the release checksums.
