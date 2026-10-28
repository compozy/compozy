# Update holder identity and recovery outcome

## Scope

Issue #1000 reports WSL2 updates fencing a live CLI during replacement daemon startup.
The code path is confirmed: holder liveness used a wall-clock process start timestamp.
The existing mutation lock is released before restart, so it cannot prove holder identity
through this window. The executable-path fix from #991 is unchanged; the audited update
restart/coordinator paths use the captured executable dependency.

Linux holders now record the boot ID plus `/proc/<pid>/stat` start ticks. Identity checks
compare that token without wall-clock conversion; a changed boot, reused PID, unreadable
process, or exited process is rejected. The operation holder, applying-installer deadline
check, mutation-lock prior owner, and desktop running-state check use this identity.
macOS and Windows retain their existing native timestamp checks.

The new persisted fields are optional. Existing journals, history, lock owner records and
app snapshots remain readable without rewriting or deleting user state. Records created by
older binaries retain timestamp verification: a stable identity cannot safely be inferred
retroactively from a potentially reused PID. Newly created Linux holders carry the token.
An older running daemon's strict operation decoder may reject a new holder field until
that daemon restarts; the new daemon reads both forms. No database/config migration or
public HTTP schema change is introduced.

A fenced CLI observes the same operation's journal/history and prints the recovered result.
Completed recovery reports whether a manual restart is needed. A pending recovery is bounded
by cancellation and the operation deadline and remains explicitly unresolved on timeout.

## Verification

- Focused macOS procutil/update/daemon/CLI tests passed, including the CLI command over a real
  isolated journal and the existing update mutation/recovery suites.
- Linux ARM64 test binaries ran in isolated, network-disabled Docker containers. The procutil
  reader/matcher cases passed; a real current-process token with a deliberately drifted
  timestamp prevented operation recovery takeover, and a changed token was rejected.
- The daemon boot-recovery test passed in `golang:1.26-bookworm`, preserving revision and
  executor generation for the live holder. The smaller Debian image lacked `ps`, required by
  the existing fixture; that environment failure was not treated as a product regression.
- Linux AMD64 and Windows AMD64 builds of procutil/update/daemon/CLI passed.
- Targeted desktop app-state suite: 19 tests passed through root Turborepo `--only`.
  Desktop lint and typecheck passed through the same affected-package path.
- A production desktop publisher run in an isolated Linux Bun container wrote a token matching
  the current process kernel identity despite a deliberately old wall-clock start timestamp.
- `go vet` passed for the four touched Go packages.
- Test-shape checker passed for new/extended update, daemon-settings and CLI suites; the
  procutil suite reports pre-existing naming/direct-assertion findings outside the new cases.

No WSL2 host sleep/resume reproduction or packaged end-to-end update was run. Full CI and
reviewer evidence belong to the PR's current head. No user's daemon was restarted or stopped.
The complete existing update-recovery scenario retains its prior untested platform status;
this report establishes the scoped identity and CLI behavior only.
