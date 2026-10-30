---
id: RT-session-processes
area: RT
title: Inspect and control session processes across stop and restart
persona: Théo
journey: J-session-processes
expected: Owned background processes and ports are visible; explicit terminate verifies exit; keep preserves detached descendants across restart and permits orphan cleanup
entry_points: Session composer Processes control; command palette Processes left running; compozy session processes; /api/processes
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: reports/2026-10-29-session-processes.md
last_report: reports/2026-10-29-session-processes.md
overlaps:
---

Start an agent that launches `sleep 9999 &` and an HTTP server. Inspect both identities, attribution,
and the listening port through the session list. Stop with terminate; verify both disappear and
cannot serve requests. Start another session, stop with keep, restart only the isolated daemon,
and terminate the retained orphan identities through the operator aggregate. Include a native
terminal as the adjacent lifecycle canary: it closes on daemon shutdown, while detached children
remain. Never signal the daemon or the agent leader.

Exercise stop, archive, and delete policy choices in the Web dialogs. Verify independent API reads
and cross-profile isolation. On macOS, observe the protected sleep while attached before releasing
its parent; the documented pre-observation blind spot is not a passing attribution case.

Automation: `web/e2e/__tests__/session-processes.spec.ts` (`E2E-PROC-001`) owns composer discovery
of an externally parented, marked detached Node process and stop-with-terminate. `E2E-PROC-002`
walks archive and delete from the session list with both keep and terminate, verifies the policy
reaches the daemon, and checks process survival or exit independently. Real OS identity,
lineage/restart persistence, and reused-PID safety remain in `internal/procutil` and
`internal/sessionprocess`; wire policy belongs to the API/client suites.

Switch profile lenses while a process list is cached: never reuse the preceding profile's rows.
A process-change event invalidates every cached profile variant for that session. Submit aggregate
termination for a live owner or a foreign profile: neither may be signaled. Retained owners in
the selected profile remain eligible across workspaces. If a terminal exits during graceful-close
timeout handling, cleanup succeeds without signaling any replacement PID identity.

Profile lifecycle regression: retain a detached process, remove its session, then archive or
remove the otherwise-empty profile. Aggregate operator cleanup must still accept the profile
label (archived) or stable owner ID (deleted) shown by the process list. Live owners and owners
from another selected profile remain refused. The owning HTTP handler race regression covers
both lifecycle selector cases; the prior isolated restart/deleted-session walk remains separate
from this profile-lifecycle coverage.

Catalog freshness regression: open a process-bearing session while its sidebar row still has an
older zero-process summary. Delete and archive must offer the keep/terminate choice using current
profile-scoped detail evidence. A delete confirmation already open must reflect a subsequent detail
update. E2E-PROC-002 preserves the policy-response and independent process-liveness checks.

Refresh the list after closing a session window whose cached detail still reports zero processes.
Archive must fetch the current profile-scoped detail before selecting a policy; a failed read must
not archive. An open delete dialog must preserve the supplied newer row until its active detail
read settles. A process-change event arriving during an older process-list read must trigger a
trailing read, even when no further events arrive.

Linux clock continuity: retain lineage observations with the kernel boot-ID/start-tick token.
After a wall-time correction and daemon restart, the same token preserves ownership while the
current timestamp projection replaces the old one. A different token (PID reuse or another boot)
must never retain ownership. Existing observations without a token keep the exact timestamp fence.

A newly launched agent after a clock correction must remain excluded from cleanup, with its
separate-group descendants still attributed by the captured kernel identity. Uncertain leader
identities protect their group without granting ancestry. A transient Linux clock read failure
must be retried; the first successful projection remains stable. Legacy stop with an explicit
terminate policy must complete cleanup after client disconnect, like waited stop.

For a bulk archive, make one selected session detail unavailable and keep another readable.
Only the unreadable target fails, with its error in the batch report; readable targets still run.
Never mutate an unreadable target using its stale catalog summary. Single-target read failures
continue to block the mutation.

Slow-stop cleanup: when an accepted stop exceeds the caller deadline, HTTP and native waited and
legacy stop preserve the terminate choice through deferred cleanup. Wait using the configured
cooperative/forced/kill budget plus a bounded settlement allowance; termination gets a separate
bounded context. A failed await must never signal descendants. In a mixed bulk archive, the
readable process-bearing session still offers the keep/terminate dialog and an unreadable target
remains a failed result after the answer.

### Cleanup and confirmation races

- A process choice remains bound to its original session while open.
- Delete confirmation fails closed when current details cannot be read.
- A stop's deferred cleanup only targets identities captured before its wait; new processes
  from a resumed session survive that cleanup.
- Archive/delete with terminate retain verified-stop cleanup after caller disconnect.
- Unauthorized process identities return an ownership-neutral result.

- In a partially successful bulk delete, Retry rereads and deletes only failed targets;
  already deleted targets are not fetched again. Readable targets proceed even if another
  selected target cannot be read.
- Dismiss or replace a delete dialog while its confirmation read is pending. Its stale
  callback must not delete anything or report an error into the replacement interaction.
- A canceled delete that never accepted a stop must not schedule descendant cleanup. An
  accepted stop retains the existing subagent cascade and settlement behavior.
- A busy scan must respect the keep/default capture budget. The action may proceed after
  two seconds without a complete capture; explicit terminate retains its error semantics.
- Deferred cleanup retries a timed-out stop wait without expanding its original identity
  set; service shutdown ends the retry. A reused terminal ID with a different PID/start
  identity must remain untouched by both TERM and KILL cleanup.
- When Retry recovers a previously unreadable target with running processes, keep/terminate
  choices become available for that target; successful deletions are not repeated.
- Real PTY and pipe terminals close with their original kernel identity. On Linux a wrong
  token is rejected, while the original token remains valid across timestamp projection changes.
