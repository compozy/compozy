---
id: ET-profile-palette-view
area: ET
title: Switch and start profile actions from the palette Profiles view
persona: Bruno
journey: J-command-profiles-from-palette
expected: The Profiles view opens from the Views group and from root search, lists every profile with glyph, name, and state — current, archived, needs-setup — disables an unavailable row with the runtime's own reason instead of hiding it, switches through the canonical selection route, and hands every lifecycle action to the existing Profiles dialog carrying its plan revision rather than mutating through a palette-only path.
entry_points: Command-K root search; Views group; palette.view.profiles; profile.use|create|update|rename|archive|unarchive|delete actions; compozy cmd-palette list|inspect; GET /api/cmd-palette/catalog and views routes over HTTP and UDS
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-profile-palette-lens-isolation; ET-profile-web-settings-lifecycle-dialogs; ET-palette-registry-driven-root; ET-palette-domain-views
---

Minted by Profiles task 12 (planning): tasks 04–06 registered `palette.view.profiles` and the stable
`profile.*` descriptors, but the S14 view's own states had no scenario owner —
`ET-profile-switcher-restore` owns the menubar switcher and
`ET-profile-remote-write-boundary` owns only the remote refusal. ADR-016 forbids a second lifecycle
path, so the handoff is the contract under test, not an implementation detail. Task 13 owns the
walk, the evidence, and the verdict.

Walk:

1. Open the palette at rest and confirm `palette.view.profiles` appears in the Views group with the
   same id, label, and effective chord that root search returns for it.
2. Push the view and confirm each row shows the glyph, the name, and its state — the current profile
   marked, an archived one readable as archived rather than only dimmer, and one carrying credential
   asks flagged needs-setup.
3. Put a profile into a pending lifecycle operation and confirm its row stays visible, disabled, and
   carries the runtime's own unavailability reason verbatim; confirm `compozy cmd-palette list`
   reports the same reason.
4. Run `profile.use` on a real profile and confirm the switch goes through the canonical selection
   route and that an attached shell performs it — not a direct palette mutation.
5. Start each lifecycle action from the view and confirm it opens the canonical Profiles dialog
   pre-filled for that profile, and that the mutation carries the plan revision the dialog read.
6. Make one plan go stale from a terminal while its dialog is open, then confirm the mutation is
   refused and the dialog re-reads rather than executing the old plan.
7. Attempt `profile.delete` and confirm the destructive confirmation still gates it.
8. Repeat the discovery step from a session with `compozy__cmd_palette_list` and confirm the view
   and actions are present with their shipped input shapes.

Expected evidence: screenshots of the Views entry, the populated view with the current, archived,
needs-setup, and disabled rows, and the destructive confirmation; the selection request and response
pair for the switch; the plan read beside the mutation quoting its revision; the stale-plan refusal;
and matching `cmd-palette list` and native-tool output for the reason and descriptor parity.
