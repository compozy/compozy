# QA Run Report — 2026-08-05 — Issue 312 Cursor models

- **Scope:** Cursor account model discovery before sessions, exact runtime-ID acceptance, and the Web exact-ID interaction.
- **Cadence tier:** targeted
- **Build:** `codex/issue-312-cursor-model-catalog` working tree · **Environment:** isolated lab `issue-312-cursor-models-20260805-200518-943803`, daemon `http://127.0.0.1:61075`
- **Started:** 2026-08-05T20:06:12Z · **Completed:** 2026-08-05T20:42:24Z · **Status:** pass

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Autonomous Agent | desktop / wifi-fast / en-US | CH-cursor-account-models |
| Bruno | Delivery Builder | desktop / wifi-fast / en-US | CH-web-exact-model-id |
| Dora | Detail Debugger | desktop / wifi-fast / en-US | CH-compozy-runtime-input-preflight |

## Flows in Scope

- `J-20` — discover and inspect the truthful provider catalog from structured surfaces (`../journeys/J-20-catalog-curation-agent-surfaces.md`).
- `J-17` — choose and persist the next-prompt runtime from the composer (`../journeys/J-17-session-create-unified-selector.md`).
- `J-02` — validate provider identity while preserving exact model IDs during dry-run (`../journeys/J-02-dry-run-preview.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-cursor-account-models | J-20 / MS-cursor-account-model-discovery, MS-042, MS-055 | Ada | Feature Tour | PASS | — | — |
| 2 | CH-web-exact-model-id | J-17 / RT-web-exact-model-id-entry, RT-session-runtime-selection-continuity | Bruno | Feature Tour | PASS | — | — |
| 3 | CH-compozy-runtime-input-preflight | J-02 / LP-runtime-validation-preflight | Dora | Garbage Tour | PASS | — | — |

## Session Debriefs

- **Ada:** `cursor-agent models` exposed 193 real account rows. The first Compozy read discovered them
  before a session; the second CLI/UDS and HTTP reads kept the first timestamp; explicit refresh
  advanced it. Normalized CLI/UDS and HTTP payloads shared SHA-256
  `b43dc2e8c7dc0022ad2e004d6f97320f8baac51eb8797315ca1ecde286209049`.
- **Bruno:** selecting Cursor exposed the exact-ID action. It opened a focused `Exact model ID` field,
  disabled empty confirmation, enabled `Use "composer-2.5"`, accepted Enter, returned cleanly to
  catalog search, and left the trigger at `Cursor Agent / Composer 2.5`.
- **Dora:** session runtime selection and Loop dry-run both preserved `cursor/composer-2.5` across
  CLI/UDS and HTTP. `flarp/anything` still returned `unknown_provider`; dry-run created zero runs and
  started no ACP process.

## What Was Fixed

- Live-source config snapshots now reconcile atomically and refresh only when discovery-relevant
  fields change. Disabling discovery clears old rows and records `disabled`; metadata-only edits do
  not invoke the provider.
- A models-only override for a built-in provider is now classified `live` even when it creates or
  removes the first raw `[providers.cursor]` table. Real `config set ...enabled false` and `config unset`
  both applied without restart; unset restored all 193 Cursor rows.

## Paper Cuts

None.

## Runtime Errors Observed

The first lifecycle probe exposed the restart classification defect above. It was fixed in production,
covered in the canonical settings/runtime suites, and the identical public probe passed on the rebuilt daemon.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- Cursor's real command output is stable machine-readable data in the form `id - display name`; parsing
  only that grammar avoids inventing models from tips or headings.
- Curated rows are metadata, not runtime membership. Provider identity is validated in Compozy; exact
  model acceptance remains at the ACP/provider boundary.
- Discovery configuration has two owners that must agree: lifecycle classification decides whether the
  runtime applier runs, and the catalog runtime atomically swaps and refreshes the live source.

## Final Status

**Verdict: PASS.** All flagged scenarios have terminal pass verdicts and evidence in the isolated
lab's `qa/issue-312-evidence.md`. The machine-owned `qa/teardown.json` records `"clean": true` with
no survivors. The strict evidence audit is repeated after the final gate; its report and the current
`make gate-status` record are the authoritative repository-verification evidence.
