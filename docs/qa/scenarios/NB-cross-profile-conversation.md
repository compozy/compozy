---
id: NB-cross-profile-conversation
area: NB
title: Let agents in different profiles keep talking to each other
persona: Ada
journey: J-run-bounded-live-collaboration
expected: Network delivery is never predicated on profile — agents owned by two different profiles converse on the shared network with nothing dropped, delayed, or filtered by the owner tag; peers and infrastructure stay machine-level with no per-profile duplication; conversation and work rows are owned by the profile of the side that created them and read back scoped or aggregate accordingly; unattended work a bridge delivers is owned by the bridge instance's profile.
entry_points: two sessions started in different profiles on the shared network; compozy network channel|thread|work list with --profile and --all-profiles; network detail routes over HTTP and UDS; bridge instance delivery; compozy bridge list
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: NB-execution-participation-defaults; ET-profile-scoped-work-reads; ET-profile-deep-link-owner
---

Minted by Profiles task 12 (planning) as the cycle's adjacent canary. The Profiles migration stamped
the four network work roots and the bridge instances, but Safety Invariant 12 states delivery is
never predicated on profile — the tag must not have become a block. ADR-010 keeps peers and
infrastructure machine-level and ADR-011 makes the entry point, not a routing table, decide the
owner of unattended inbound work. Task 13 owns the walk, the evidence, and the verdict.

Walk:

1. Start one agent session in each of two profiles and have them hold a real conversation on the
   shared network; confirm every message arrives, in order, with no drop, delay, or filter
   attributable to the differing owners.
2. Confirm peers and infrastructure are listed once and identically from both profiles — no
   per-profile duplication and no profile-scoped peer catalog.
3. List channels, threads, direct rooms, and network work scoped to each profile and confirm each
   row belongs to the side that created it; list with the explicit aggregate and confirm every row
   carries its owner.
4. Open one conversation's detail from the profile that did not create it and confirm the scoped
   read returns not found while the aggregate-by-id read returns it owner-labeled.
5. Deliver unattended work through a bridge instance owned by one profile and confirm the produced
   work carries that instance's owner regardless of which profile is currently active.
6. Confirm the network audit and permission logs remain readable and are not erased by the owner
   dimension.
7. Archive one of the two profiles mid-conversation and confirm delivery behavior matches the
   documented contract rather than silently dropping messages.

Expected evidence: paired session transcripts with message timestamps from both sides; the peer and
infrastructure listing from each profile; scoped and aggregate listings for all four network work
families; the scoped-versus-aggregate detail pair; the bridge-delivered work with its owner; the
audit and permission log reads; and the archive-time delivery outcome.
