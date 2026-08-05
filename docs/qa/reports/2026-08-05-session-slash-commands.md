# QA Run Report — 2026-08-05 — session slash commands

- **Scope:** Session-effective slash command catalog, inline assistant-ui insertion, public catalog parity, and the exact-text composer canary
- **Cadence tier:** targeted
- **Build:** `f54e62b`, `0044df85`, `a1d6877d`, `acbbb25d`, `cb389514` · **Environment:** two fresh isolated daemon and Web labs; latest manifest `/Users/pedronauck/dev/qa-labs/compozy-session-slash-review-20260805-071845-995212-lab/qa-artifacts/qa/bootstrap-manifest.json`
- **Started:** 2026-08-05T00:51:44-03:00 · **Status:** complete

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Théo | Power User | desktop / wifi-fast / en-US | CH-session-inline-slash-commands |
| Bruno | Power User | desktop / wifi-fast / en-US | CH-session-command-catalog-parity, CH-session-composer-text-entry |

## Flows in Scope

- `J-use-session-slash-commands` — discover and insert only session-effective commands without losing authored text (`../journeys/J-use-session-slash-commands.md`)
- `J-17` — exact session composer text entry as the adjacent canary (`../journeys/J-17-session-create-unified-selector.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-session-inline-slash-commands | J-use-session-slash-commands / ET-session-slash-commands-inline | Théo | Feature Tour | Fixed | BUG-20260805-command-menu-hook-order | f54e62b, acbbb25d |
| 2 | CH-session-command-catalog-parity | J-use-session-slash-commands / ET-session-command-catalog-parity | Bruno | Feature Tour | Pass | | |
| 3 | CH-session-composer-text-entry | J-17 / ET-web-session-composer-text-entry | Bruno | Feature Tour | Pass | | f54e62b, acbbb25d |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-session-command-catalog-parity — Bruno

- **Ran:** 2026-08-05T00:57:51-03:00 → 2026-08-05T01:03:00-03:00 (box respected: yes)
- **Findings:** none. CLI and HTTP were semantically identical and the workspace fence returned 404 without catalog data.
- **Bugs filed/updated:** none
- **Scenarios settled:** ET-session-command-catalog-parity → pass
- **Paper cuts:** none
- **Surprises:** the isolated session also exposed the bundled `/compozy` skill, proving the catalog combined global and workspace sources while applying the agent tombstone.
- **Suggested next charter:** exercise extension enable/disable revision changes in a dedicated extension-lifecycle lab.

### CH-session-inline-slash-commands — Théo (initial walk)

- **Ran:** 2026-08-05T01:03:00-03:00 → 2026-08-05T01:08:00-03:00 (box respected: yes)
- **Findings:** inline insertion preserved the Unicode prefix and existing suffix exactly, and the submitted prompt activated the workspace skill; however, React reported changed Hook order when the native popover mounted and unmounted.
- **Bugs filed/updated:** BUG-20260805-command-menu-hook-order
- **Scenarios settled:** ET-session-slash-commands-inline → fail pending governed fix and fresh re-walk
- **Paper cuts:** none beyond the blocking runtime finding
- **Surprises:** moving the caret left through an existing suffix reopened the assistant-ui query at the correct middle-of-prompt range.
- **Suggested next charter:** re-run this charter from a fresh browser session after the Hook ownership fix.

### CH-session-inline-slash-commands — Théo (fresh re-walk)

- **Ran:** 2026-08-05T01:24:00-03:00 → 2026-08-05T01:29:00-03:00 (box respected: yes)
- **Findings:** none after the fix. Opening, closing, and reopening the native popover preserved `Revisão 😊 /browser-qa before launch` exactly and left the browser console clean.
- **Bugs filed/updated:** BUG-20260805-command-menu-hook-order → verified
- **Scenarios settled:** ET-session-slash-commands-inline → pass
- **Paper cuts:** none
- **Surprises:** axe reported zero violations at 320 px; its only incomplete check was contrast calculation for elements overlapped by the popover itself.
- **Suggested next charter:** exercise live extension enable/disable revision invalidation in a dedicated extension-lifecycle lab.

### CH-session-composer-text-entry — Bruno

- **Ran:** 2026-08-05T01:32:00-03:00 → 2026-08-05T01:36:00-03:00 (box respected: yes)
- **Findings:** none. A newly created session preserved leading, repeated, and trailing spaces plus accented text under sequential keyboard entry, before and after the runtime selector, after refresh, and from a fresh-browser deep link.
- **Bugs filed/updated:** none
- **Scenarios settled:** ET-web-session-composer-text-entry → pass
- **Paper cuts:** none
- **Surprises:** none
- **Suggested next charter:** none; the existing text-entry scenario remains the canonical adjacent canary.

### CH-session-inline-slash-commands — Théo and Bruno (reviewed-head re-walk)

- **Ran:** 2026-08-05T04:24:00-03:00 → 2026-08-05T04:31:18-03:00 (box respected: yes)
- **Findings:** none. The native menu inserted adjacent repeated `/browser-qa` markers after Unicode text; the runtime preserved repeated spacing, deduplicated the invocation by source identity, and the live provider acknowledged the injected skill. Reload restored the exact submitted prompt.
- **Bugs filed/updated:** none
- **Scenarios settled:** ET-session-slash-commands-inline → pass; ET-web-session-composer-text-entry → pass; ET-session-command-catalog-parity → pass
- **Paper cuts:** none
- **Surprises:** the Web, CLI, and HTTP catalog independently omitted the agent-disabled `/hidden-skill`, while the 320 px menu remained readable with no page or console errors.
- **Suggested next charter:** exercise extension enable/disable revision invalidation in a dedicated extension-lifecycle lab.

## What Was Fixed

- `f54e62b` aliases assistant-ui's unstable scope export to a local `use…` Hook name so React Compiler preserves the call order across popover renders.
- The same commit adds the canonical open → close → reopen regression and stabilizes the slash Storybook story with a stopped-session mock.
- `acbbb25d` keeps directive whitespace and draft synchronization under the assistant-ui composer owner, makes catalog-stream failures nonfatal, and tightens disabled-skill and native-tool error coverage.
- `cb389514` covers the unchanged nonempty SSE catalog revision as a silent no-op.

## Paper Cuts

None.

## Runtime Errors Observed

- Initial walk: React Hook-order errors for `CommandCatalogOpenReporter` and `CommandCatalogTriggerRangeBridge` after opening and closing the command popover. Evidence: `/Users/pedronauck/dev/qa-labs/compozy-session-slash-commands-20260805-035316-316748-lab/qa-artifacts/qa/console-before-fix.txt`.
- Fresh re-walk: no page errors, console errors, or warnings. Evidence: `/Users/pedronauck/dev/qa-labs/compozy-session-slash-commands-20260805-035316-316748-lab/qa-artifacts/qa/console-after-fix.txt`.
- Reviewed-head re-walk: no page or console errors. Evidence: `/Users/pedronauck/dev/qa-labs/compozy-session-slash-review-20260805-071845-995212-lab/qa-artifacts/qa/console-reviewed-head.txt`.

## Human Verifications Needed

None. The reviewed-head lab completed a live native-CLI provider turn and retained the source-qualified invocation in session history.

## Decisions for a Human

None.

## Learnings

- Unstable library exports should be imported under a local `use…` name when React Compiler must recognize them as Hooks; namespace property calls are not sufficient.
- Focus-driven popovers need a non-live Storybook session fixture so SSE reconnection cannot remount the composer during visual capture.

## Final Status

- **Exit gate (full automated suite):** pass — `make gate-full` on the final tree
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 1 fixed · Friction 0 · Cosmetic 0
- **Coverage:** 2/2 journeys walked
- **Verdict:** pass — all planned sessions are settled, the reviewed head passed live QA, and the final automated gate passed.
