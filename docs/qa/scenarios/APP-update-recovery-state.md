---
id: APP-update-recovery-state
area: APP
title: A failed update never strands me silently
persona: Dora
journey: J-desktop-update-moment
expected: A download or native-staging failure leaves the app usable, archives the failure, and offers Retry update without canceling a staged operation first. A post-swap runtime failure retains the replacement and backup, reports recovery guidance, and preserves migrated user state. A live slow boot remains in progress until ready or actual exit. Recovery clears historical rollback from the live projection without editing history.
entry_points: update surface after a forced app apply failure; compozy update -o json; update-history.jsonl after a runtime failure
qa_status: untested
bug_ids: BUG-20260810-healthy-retry-corrupts-state
fix_status: fixed
retest_status: pass
fix_commits: f081a1e
evidence: docs/qa/reports/2026-08-17-electron-shell.md
last_report: docs/qa/reports/2026-09-09-issue-559-safe-update-recovery.md
overlaps: APP-agent-cli-app-verbs
---

PRD stories: US-015 (AC-1 failed apply → report + manual path; AC-2 malformed/unreachable channel
visible with last-checked; EC-1 crash-on-new-version detectable, fallback reachable; EC-2 no
downgrade), US-016.EC-1 (runtime apply fails → previous usable + diagnostics). Test IDs: E2E-013,
E2E-025; IT-015, IT-017, IT-029; UT-057, UT-059–UT-063, UT-101, UT-102, UT-114, UT-115.

Forced-failure posture (release rehearsal requirement): after the locked-install-dir failure the
app must remain OS-launchable AND the install path's permissions must be unchanged — never left
clamped to `0700` (or the platform ACL equivalent) by the failed apply.

Per-OS evidence: E2E-017 runs on macOS and Linux with the locked-dir fixture — capture the failed-
update report, the opened release page, a fresh OS-level app launch, and the install-path
permission listing before/after. E2E-018 forces a runtime post-swap health failure on both release
OSes and records replacement retention plus the archived `failed` outcome. Overlap:
APP-agent-cli-app-verbs owns the structured CLI readout of the same durable update result.

Issue 559 re-walk: `docs/qa/reports/2026-09-09-issue-559-safe-update-recovery.md`.

Staged recovery verifies the packaged runtime digest before any transition-client execution. The issue 559 packaged re-walk covers both a valid bundle with failing daemon boot and a tampered bundle that must never execute.


### Bootstrap shutdown coordination

A staged app installer can run before runtime readiness, after bundle verification. Before installer handoff or shell shutdown, the desktop cancels and awaits its bootstrap observer, preserves the detached daemon, and prevents late product-window publication. A later successful daemon startup reconciles superseded `starting` restart observations without changing the current restart operation or restoring an older binary. Verified by the issue #559 shell and daemon boot integration evidence in `../reports/2026-09-09-issue-559-safe-update-recovery.md`.

Clean shell shutdown drains already queued app-state publications before removing the startup marker. Later runtime/update callbacks cannot rewrite the state record or recreate the marker. The existing desktop app-state publisher suite owns the concurrent-publication regression; the packaged bootstrap-quit and relaunch scenarios verify the shutdown integration.

Shutdown follow-up (2026-10-19): after draining shell services, close the product window through
its existing state-preserving close path before marking shutdown clean. The final app quit must
not depend on a renderer answering its unload handshake. In the existing packaged E2E-007 suite,
an intentionally blocked `beforeunload` reproduced completed cleanup followed by an unresponsive
renderer and a live shell PID under the unchanged five-second shutdown deadline. The production
fix passed that regression without a termination signal, preserving the same daemon PID. The
focused macOS packaged run also passed bootstrap quit, cache-relaunch #742, and renderer
crash recovery (four scenarios total). This proves the renderer-close failure mode; it does not
identify why the renderer originally stalled in Linux CI. Full cross-platform CI remains the
delivery gate, and prior update-recovery evidence above is retained.

If product-window disposal fails, keep the startup marker while logging the failure and
continuing the remaining cleanup. The packaged E2E-007 failure-path case rejects the real atomic
window-state write and verifies that the next launch's public `app diagnose` reports the prior
boot identity. It reproduced a missing `previous_crash` before the guard and passed after it.

2026-10-21 targeted implementation verification: [update-flow report](../reports/2026-10-21-desktop-update-flow.md). The current full scenario is flagged
`untested`; native fixture and owning-suite results do not replace published-release or
cross-platform acceptance. Prior dated reports retain their historical evidence.

### Linux holder identity and fenced CLI recovery

On Linux, keep an update holder alive while its recorded wall-clock start timestamp differs
from the process table by more than two seconds. Its persisted boot ID and kernel start ticks
must keep the lease live, block daemon recovery takeover, and preserve the executor generation.
A changed boot ID, changed start ticks, or exited process must still reject that identity.
Repeat the applying-installer deadline check and the update-lock prior-owner check.

When another executor has legitimately taken over, the original CLI must observe the same
operation ID in the journal/history and print that executor's terminal result. A successful
recovery reports no manual restart needed; a failed recovery without a completed daemon restart
prints `compozy daemon stop && compozy daemon start` with the persisted failure reason. A recovery
still pending at its deadline must remain explicitly unresolved rather than claiming success.

The owning procutil/update/daemon/CLI suites exercise injected identity readers and isolated
journals. Linux container runs exercise the actual `/proc` reader; they do not reproduce WSL2
host sleep or replace a WSL2 end-to-end update rehearsal.

During detached coordinator handoff, replace the parent identity with the child process token.
If identity becomes unreadable after acquisition, report the lookup error without changing
revision/generation or allowing takeover. With procfs restricted at initial creation, daemon
and desktop startup retain timestamp-based identity. Close the app while a fenced CLI waits;
the next local presence read must return the staged handoff with version and attempt details.
