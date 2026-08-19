# QA Run Report — 2026-08-18 — typed-loop-inputs

- **Scope:** Typed Loop entity/runtime inputs, cross-surface validation, config defaults, and entity-annotated human responses.
- **Cadence tier:** targeted
- **Build:** `3e3415b3` + working tree · **Environment:** isolated lab, CLI/API/Web/runtime; no live provider required
- **Started:** 2026-08-18T22:55:37-03:00 · **Status:** pass

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Lea | New User | laptop / wifi-fast / en-US | CH-typed-loop-entity-inputs |
| Ada | Power User | desktop / wifi-fast / en-US | CH-compozy-runtime-input-preflight |
| Bruno | Power User | desktop / flaky / en-US | CH-typed-request-entity-answer |

## Flows in Scope

- `J-01` — Start a default Loop with exact typed inputs (`../journeys/J-01-arrive-and-use-run.md`)
- `J-02` — Preview effective defaults and runtime validation without side effects (`../journeys/J-02-dry-run-preview.md`)
- `J-supervise-loop-request` — Resolve one request and resume trusted work (`../journeys/J-supervise-loop-request.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-typed-loop-entity-inputs | J-01 / LP-select-typed-loop-entities | Lea | Garbage Tour | Fixed | BUG-20260729-tool-invoke-structural-redaction; BUG-20260818-runtime-input-split-controls | pending implementation commit |
| 2 | CH-compozy-runtime-input-preflight | J-02 / LP-loop-input-defaults | Ada | Garbage Tour | Fixed | BUG-20260818-loop-input-object-get | pending implementation commit |
| 3 | CH-compozy-runtime-input-preflight | J-02 / LP-runtime-validation-preflight | Ada | Garbage Tour | Pass | | |
| 4 | CH-typed-request-entity-answer | J-supervise-loop-request / LP-answer-typed-request-entities | Bruno | Network Tour | Fixed | BUG-20260818-nested-entity-picker-missing | pending implementation commit |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

- **Lea:** CLI prompts and `--no-prompt` preserved exact enum, agent, skill, secret, file, and
  partial-runtime values. HTTP/UDS, native dry-run, and Web agreed. A rebuilt Storybook story showed
  the canonical runtime selector in both healthy and catalog-error states.
- **Ada:** Workspace runtime defaults round-tripped as an object, appeared with `origin: workspace`,
  and accepted an exact custom model. Invalid references returned `input_validation` before any run
  was created.
- **Bruno:** A missing nested reviewer returned `origin: response` at
  `assignment.reviewer` and kept the request pending. A fresh run/browser then rendered the agent
  picker, answered once, reached `done`, and drained the independent CLI live-request list.

## What Was Fixed

- Public Vault refs and Loop input declarations no longer disappear under structural redaction;
  secret values remain redacted.
- `config get` can read an exact saved runtime object, not only its flattened leaves.
- Nested entity annotations project into the shared picker and assemble the nested response object.
- Runtime inputs always use `RuntimeSelector`; Storybook no longer invents a missing `ws_default`
  workspace or replaces the selector with three raw fields.

## Paper Cuts

- The Runtime selector accessible name includes a punctuation-heavy summary. It remained clear and
  keyboard reachable, so this run records it as polish rather than a blocker.

## Runtime Errors Observed

- The first native dry-run was correctly rejected because the QA fixture had not declared a
  `native_tool` start. The fixture was republished with that public start contract; no production
  defect was filed.
- Missing entities and invalid nested response shapes produced their intended public errors without
  closing the request or creating a run.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- Taxonomy plan: J-01 owns journey/functional coverage, exact-value and stale-reference paths own errors, Garbage Tour owns input friction, and CLI/API/Web/native comparison owns cross-cutting consistency.
- Taxonomy plan: J-supervise-loop-request owns journey/functional coverage, invalid/duplicate/network-drop paths own errors, Network Tour owns recovery, and independent fresh reads own cross-cutting consistency.
- Responsive/mobile coverage is deliberately skipped: this change touches desktop authoring controls, while the supported mobile Loop surface is read/approve rather than run authoring.
- Runtime catalog failure must degrade inside the canonical selector. Replacing the component at an
  error boundary silently forks interaction, accessibility, and exact-ID behavior.

## Final Status

**PASS** for targeted behavioral QA. All four scenarios passed after fixes on a rebuilt candidate.
The strict evidence audit, clean lab teardown, one requested deep-review round, and workstream gate
remain delivery gates outside this behavioral verdict.
