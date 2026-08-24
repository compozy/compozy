---
id: ET-profile-approval-owner-resume
area: ET
title: Resume a pending approval under the profile that created it
persona: Ada
journey: J-operate-profiles
expected: A pending tool or palette approval records the profile that created it and resumes under that owner even after the operator switches, re-running the session-immutability, local-management, availability, and policy checks on resume; archive and delete refuse with profile_approvals_pending while an executable pending approval belongs to the profile, naming the approval ids to resolve or cancel; pending approvals are never counted as work items.
entry_points: destructive palette or tool invocation awaiting approval; compozy approvals show|resolve; compozy profile archive|delete; GET /api/profiles/{name}/archive-plan|delete-plan; POST /api/profiles/{name}/archive; DELETE /api/profiles/{name}; compozy__cmd_palette_invoke
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-profile-lifecycle-race-guards; ET-agent-command-invoke; ET-profile-cli-lifecycle
---

Minted by Profiles task 12 (planning): ADR-016 makes `tool_approval_pending` the eighteenth durable
owner root — excluded from the work-items count but blocking archive and delete — and requires
owner-preserving resume. `ET-agent-command-invoke` owns the approval lifecycle itself; this row owns
its profile ownership. Task 13 owns the walk, the evidence, and the verdict.

Walk:

1. Start a destructive invocation in a non-default profile so an approval is left pending, and
   confirm the pending record names that profile as its owner.
2. Switch the operator to another profile and resolve the approval; confirm the effect executes
   under the original owner — the resulting work is stamped with it, not with the current profile.
3. Repeat with a resume that should fail: make the owning profile unavailable through a pending
   lifecycle operation and confirm the resume is refused with the availability reason rather than
   silently running.
4. Repeat from a remote surface and confirm the local-management refusal still applies on resume.
5. Attempt the resume with an acting profile that differs from the originating session's binding and
   confirm the typed session-conflict refusal.
6. With an executable approval still pending, read the archive plan and the delete plan and confirm
   both list it as a blocker; attempt both mutations and confirm each refuses with
   `profile_approvals_pending` naming the approval ids.
7. Resolve or cancel the approval, re-read both plans, and confirm the blocker is gone and the
   mutation now proceeds.
8. Confirm the profile's work-items count never included the pending approval, on `profile list`,
   the plan payloads, and the aggregate labels.

Expected evidence: the pending record showing its owner; the post-switch resume result with the
stamped owner of the produced work; the unavailable, remote, and session-conflict refusals; both
plan payloads before and after the approval is cleared; the refused mutation bodies naming the
approval ids; and the work-items count on either side.
