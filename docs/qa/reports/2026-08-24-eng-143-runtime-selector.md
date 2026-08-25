# QA Run Report — 2026-08-24 — eng-143-runtime-selector

- **Scope:** ENG-143 runtime-selector provider tooltips
- **Cadence tier:** targeted
- **Build:** working tree for `linear-eng-143` · **Environment:** isolated Web lab at `http://[::1]:3000/`, with `COMPOZY_WEB_API_PROXY_TARGET=http://127.0.0.1:60608`
- **Started:** 2026-08-25T00:51:20Z · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Sol | default onboarding | desktop / local isolated lab / en-US | CH-untested-018-17-sol |

## Flows in Scope

- `J-17` — discover provider identity in the runtime selector (`../journeys/J-17.md`)
- `ET-web-runtime-selector-minimal-slider` — runtime selector entry points and provider/model surfaces

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-untested-018-17-sol | J-17 / ET-web-runtime-selector-minimal-slider | Sol | Feature Tour | Pass | | |

## Session Debriefs

### CH-untested-018-17-sol — Sol

- **Ran:** 2026-08-25T00:51:20Z → 2026-08-25T00:53:38Z (box respected: yes)
- **Findings:**
  - Codex displayed the exact `Codex` tooltip on hover and the shared tooltip appeared above the popup.
  - Keyboard focus on the Codex provider chip displayed the same tooltip while preserving the radio trigger.
  - Groq displayed the truthful `Groq · needs sign in` label.
  - The pinned cross-provider GPT-5.6 Sol row displayed the exact `Codex` tooltip when its provider glyph was hovered.
  - Searching for `gpt` disabled the provider radios and did not show a tooltip when a disabled chip was hovered.
- **Bugs filed/updated:** none
- **Scenarios settled:** provider hover, keyboard focus, needs-sign-in hover, pinned model-glyph hover, and disabled-search behavior all passed in the live walk
- **Paper cuts:** none
- **Surprises:** none
- **Suggested next charter:** none for ENG-143; CI should complete the repository-wide verification gate

## What Was Fixed

No QA bugs were found. ENG-143 implementation details and regression coverage are recorded in the PR.

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|
| Sol | J-17 provider rail | none | dull | no action |

## Runtime Errors Observed

- None. `agent-browser errors` returned no page errors during the walk.

## Decisions for a Human

None. The full repository gate is an automated CI release-readiness prerequisite, not a human-only verification step.

## Learnings

- The shared tooltip primitive provides the required portal and stacking behavior without adding a local provider.

## Final Status

- **Exit gate (full automated suite):** not run locally by instruction; CI must run `make verify` as the automated release-readiness prerequisite.
- **Task-scoped gate:** `make gate` ran the Web lint/typecheck/test lane; lint and typecheck passed, while the pre-existing `web-storybook-visual-contract.test.ts` timeout and `use-window-manager-stream.test.tsx` failure occurred independently of the ENG-143 files. Both failures reproduced or were isolated separately.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1/1 targeted journey walked; pointer, keyboard-focus, needs-sign-in, pinned-glyph, and disabled-search states all passed.
- **Verdict:** ready — ENG-143 behavior passed automated and live checks; CI still owns the full gate and the unrelated baseline test failure.

Evidence: `/Users/pedronauck/dev/qa-labs/compozy-eng-143-runtime-selector-20260825-004835-671661-lab/qa-artifacts/qa/`; teardown evidence: `teardown.json` (`clean: true`, no survivors).
