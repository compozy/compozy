---
id: MS-profile-memory-tier-scope
area: MS
title: Keep the profile memory tier inside its owner
persona: Ada
journey: J-layer-profile-resources
expected: The home-level memory tier is named profile and is owned per profile — catalog, search, recall, and full-text results never return another profile's entries and an aggregate memory read is refused; repository workspace memory stays shared across profiles and agent-tier memory follows its owning agent's layer; profile-tier reads fail closed while the directory move is still pending; pre-profile entries read back under default from the new location and the old path is never used as a fallback.
entry_points: compozy memory list|show|write|search; GET /api/memory routes over HTTP and UDS; native memory projections inside a session; $COMPOZY_HOME/profiles/<name>/memory/; Web Settings → Memory
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: MS-repo-profile-layer-adoption; RT-migrate-memory-stream-when-disabled; ET-profile-scoped-work-reads
---

Minted by Profiles task 12 (planning): ADR-013 renames the shipped `global` memory tier to `profile`
and makes profile ownership durable identity, and couples the directory move to the memory stream's
own migration with a fail-closed guard while its maintenance row is pending. The Read-Scope
Enforcement Matrix marks memory as the one family where the aggregate is refused rather than
labeled. Task 13 owns the walk, the evidence, and the verdict.

Walk:

1. Write memory entries in the profile tier under two different profiles, with overlapping slugs and
   overlapping search terms.
2. Read catalog, search, recall, and full-text results in each profile through CLI, HTTP, UDS, and a
   native projection inside a session; confirm no entry from the other profile appears in any of
   them, including as a match highlight or a count.
3. Request an aggregate memory read and confirm it is refused rather than returned labeled — memory
   is the documented exception to the two read modes.
4. Write a workspace-tier entry and confirm both profiles see it; write an agent-tier entry and
   confirm it follows its agent's layer rather than the acting profile.
5. Confirm the shipped scope vocabulary is `profile | workspace | agent` on every surface — CLI help
   and output, API payloads, and the Settings page — with no `global` value accepted or emitted for
   memory.
6. Seed an install whose memory maintenance move is still pending, attempt a profile-tier read, and
   confirm it refuses fail-closed instead of reading the old path; complete the move and confirm the
   read succeeds and the old path is gone.
7. Confirm pre-profile entries read back under `default` from the new location with their content
   and metadata intact.
8. Delete an empty profile that owns memory entries and confirm the delete preview counts them and
   the applied delete removes exactly those, leaving the other profile's entries intact.

Expected evidence: paired per-profile catalog, search, and recall output on every surface; the
refused aggregate response; workspace-tier and agent-tier reads from both profiles; the scope
vocabulary as printed by CLI and API; the pending-move refusal and the post-move success; the
migrated entry read under default; and the delete preview beside the delete result with entry counts.
