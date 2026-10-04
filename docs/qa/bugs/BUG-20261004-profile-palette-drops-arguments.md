# BUG-20261004-profile-palette-drops-arguments: Profile commands lose their requested lifecycle action or name

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Ada; Bruno
- **Journey Step:** J-operate-profiles, delegated lifecycle invocation; J-command-profiles-from-palette, dialog handoff
- **Scenarios:** ET-profile-remote-write-boundary; ET-profile-palette-view
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

An attached CLI profile.create command reports success and opens Settings without its creation
dialog. Direct Web invocation opens that dialog but discards the name already entered in the
palette. Delegated profile.use also reports success without switching. The operator must discover
and repeat the missing action or input.

## Reproduction

- **Charter:** CH-profile-lifecycle-plan-recovery · **Tour:** Interrupt Tour
- **Environment:** isolated production daemon/Web on port 50727, desktop 1512×862, en-US,
  browser-use with a real attached Chrome client; 0b9c79779 plus the owner/admission repair
  subsequently committed as 7d30fa3e2.

1. Discover profile.create with cmd-palette list and the attached browser client.
2. Invoke it with --arg name=dispatch-palette. CLI returns status ok; Settings opens without a dialog.
3. After a fresh reload and confirmed descriptor availability, repeat once: the same result.
4. In a fresh browser tab, choose Create profile in the palette and type dispatch-palette.
5. Submit. The canonical creation dialog opens with an empty Name field. Cancel without mutation.
6. Invoke profile.use with profile=research against the attached client. CLI returns status ok,
   but the visible profile and independent workspace selection remain resume-editorial after reload.

Expected: delegated invocation opens the canonical dialog and preserves the supplied name.
Actual: daemon navigation loses the flow; the direct dialog handoff loses the supplied name;
delegated selection loses its target.

## Evidence

Under docs/qa/evidence/2026-10-02-untested/: profile-local-palette-create-invoke.json,
profile-local-palette-create-ready-retry.json, profile-local-palette-create-no-dialog.png,
profile-local-palette-create-absent.json, profile-local-palette-argument-clean-entry.json,
profile-local-palette-argument-confirmed.json, and profile-local-palette-empty-name.png.
Independent UDS returns profile_not_found. Both screenshots were inspected.
The selection probe is retained in profile-local-palette-use-invoke.json,
profile-local-palette-use-observed.json, profile-local-palette-use-after.json and
profile-local-palette-use-ended.json. The two browser receipts carry the harness default actor Lea;
the actual adopted persona is Ada, as their recording title/session end text and entry receipt state.

The first direct-Web attempt stalled before its dialog could be observed. Its premature empty-name
conclusion is explicitly superseded by profile-local-palette-argument-ended-correction.json.
The fresh-tab replay supplies the actual empty-name evidence. The stall remains separately tracked.

## Investigation

The daemon sends merged action/invocation arguments, but CMD_PALETTE_DAEMON_OPS forwards only
pathname to the navigation port. Direct Web dispatch already forwards scalar route arguments.
The profile route parser then recognizes only flow/profile, although the published create and
rename descriptors declare name and new_name. The dialog bridge also leaves those arguments in
the persisted route. The selection handler reads only flat arguments although daemon commands
wrap them in args, so it silently receives an empty target. No mutation, policy, plan revision,
or approval contract needs replacement.

The navigation invariant belongs to the existing cmd-palette-client-ops.test.ts suite. Route
normalization belongs to -settings-split.test.tsx. Actual dialog prefill, cancellation, and reload
belong to the existing E2E-027 in profiles.spec.ts. Repair remains bounded to those handoffs.

## Fix

The daemon navigation handler shares the local scalar route builder. The profile flow parser
and dialog bridge carry create name and rename new_name, then consume all intent fields. The
profile.use handler reads the daemon args envelope while retaining local flat arguments. Rename
draft fallback preserves a supplied suggestion without preventing the operator from clearing it.
Commit pending delivery gate.

## Verification

The original two unit failures and old-bundle E2E failure are retained in
profile-palette-arguments-red.json and profile-palette-arguments-e2e-red.json. The separate
profile.use red case receives an empty target before repair. Root Turbo focused checks and
typecheck pass; final E2E-027 and adjacent E2E-017 pass against the production build.

Fresh Ada structured invocation and Bruno operator replay on Web index SHA-256
ace0ae12b295e4ed9f52bf92539194ea097a7507e61c6eb26665b3e16cd9f5e0 complete all seven descriptors.
Create dispatch-delegated is prefilled, identity saves, rename preserves dispatch-edition, archive
and unarchive persist, and delete requires command approval plus the canonical confirmation.
Cancel preserves the profile; the final confirmed delete returns an independent 404. Rename,
archive and delete requests quote the independently reviewed plan revision. The original
workspace selection is restored and reload opens no dialog.

Receipts: profile-palette-lifecycle-*.json, profile-palette-fixed-use-invoke.json and the independent
selection/reload receipts. The inspected delegated-create and delegated-delete screenshots and
the stopped 12-frame profile-palette-lifecycle-fixed-bruno recording retain visible evidence.
The separate navigation-stall finding remains open; successful replay is not causal proof of its fix.
