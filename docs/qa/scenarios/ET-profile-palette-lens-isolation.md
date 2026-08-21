---
id: ET-profile-palette-lens-isolation
area: ET
title: Keep every command-palette seam inside one profile lens
persona: Ada
journey: J-command-profiles-from-palette
expected: Catalog, search, providers, domain views, view sessions, invoke, events, and web caches all carry one real profile lens or the explicit labeled aggregate; omitting the lens at a public boundary resolves default rather than everything; a session-bound caller cannot select a different profile; ranking, recents, query hits, and pins are partitioned per lens with the aggregate keeping its own history and never reading or mutating a real profile's.
entry_points: Command-K in two profiles; compozy cmd-palette list|inspect|pin|unpin|personalization show|reset; palette catalog, search, views, view-sessions, invoke, and SSE routes over HTTP and UDS; compozy__cmd_palette_list|invoke inside a session
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-profile-palette-view; ET-palette-personalization-lifecycle; ET-agent-palette-config-parity; ET-profile-scoped-work-reads
---

Minted by Profiles task 12 (planning) for the task_06 and task_07 palette rows of the Read-Scope
Enforcement Matrix. ADR-016 makes the lens explicit at every seam and rebuilds the three
personalization tables around `profile_lens_id` with `@all` reserved for the aggregate;
`ET-palette-personalization-lifecycle` owns the pre-profile personalization behavior and this row
owns its partitioning. Task 13 owns the walk, the evidence, and the verdict.

Walk:

1. Build distinct palette histories in two profiles — different commands used, different queries
   typed, different pins — then open the palette in each and confirm ranking, recents, and pins
   reflect only that profile.
2. Turn on the explicit aggregate lens and confirm it carries its own history, shows owner labels on
   entity rows, and neither reads nor writes either real profile's personalization.
3. Call the catalog, search, view, view-session, and invoke routes over HTTP and UDS without a
   profile parameter and confirm each resolves `default` — never an unscoped superset.
4. Supply a profile and the aggregate together and confirm the structured conflict response.
5. From inside a managed session, call `compozy__cmd_palette_list` and confirm the lens comes from
   the session's immutable identity; then pass a different acting profile and confirm it is refused
   rather than ignored.
6. Open a domain view in one profile, switch profiles with the palette open, and confirm the view
   session, its cached rows, and the SSE invalidation all re-fence — reopening shows no row from the
   previous lens.
7. Reload the browser after the switch and confirm the palette query keys and cached view sessions
   still hold no stale entry from the prior profile.
8. Delete a profile that accumulated personalization and confirm only that lens's usage, query hits,
   and pins are removed while the other profile's are intact.

Expected evidence: paired palette screenshots per profile and for the aggregate; structured
`cmd-palette list` and `personalization show` output per lens; the no-parameter and conflict
responses on both transports; the native-tool refusal payload; before-and-after cache captures
across a switch and a reload; and the personalization row counts before and after the delete.
