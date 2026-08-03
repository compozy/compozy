# QA Run Report — 2026-08-03 — bundles removal review

- **Scope:** Review-remediation changes to extension preview deltas, Network confirmation, the Web kit inventory, and manifest v2 duplicate-layout diagnostics.
- **Cadence tier:** targeted
- **Build:** `c009e6b1` plus the uncommitted review-remediation worktree · **Environment:** fresh isolated lab `bundles-removal-review-20260803-040035-513450`, daemon `http://127.0.0.1:55158`, manifest-owned runtime/provider homes, UDS, tmux socket, and Web proxy.
- **Started:** 2026-08-03T04:00:35Z · **Completed:** 2026-08-03T05:18:57Z · **Status:** PASS

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-extension-kit-lifecycle; CH-extension-network-consent |
| Vera | Power User | desktop / wifi-fast / en-US | CH-extension-contract-policy |

## Flows in Scope

- `J-extension-kit-lifecycle` — preview, consent to, and inspect one extension kit without hidden mutations (`../journeys/J-extension-kit-lifecycle.md`).
- `J-extension-policy-admin` — validate manifest v2 declarations without permissive fallback (`../journeys/J-extension-policy-admin.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---:|---|---|---|---|---|---|---|
| 1 | CH-extension-kit-lifecycle | J-extension-kit-lifecycle / ET-ext-preview | Bruno | Feature Tour | Fixed | BUG-20260803-extension-preview-layout-identity | Phase D remediation checkpoint |
| 2 | CH-extension-kit-lifecycle | J-extension-kit-lifecycle / ET-web-extension-kit-inventory | Bruno | Feature Tour | Pass | | |
| 3 | CH-extension-network-consent | J-extension-kit-lifecycle / ET-ext-network-confirm | Bruno | Multi-Tab Tour | Pass | | |
| 4 | CH-extension-contract-policy | J-extension-policy-admin / ET-extension-manifest-v2-surfaces | Vera | Garbage Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### Bruno — Preview and inventory

Bruno installed the local `review-kit` through the public CLI. While disabled, CLI/UDS and HTTP
agreed that the skill and window layout were shipped but not live. Preview returned only the two
canonical `added` rows, the current Network digest, and no hidden mutations: the complete status
document was byte-identical before and after preview.

The first enabled-state preview exposed BUG-20260803-extension-preview-layout-identity: one
unchanged layout appeared as both removed and added. After the production fix and daemon restart,
CLI/UDS and HTTP both returned an empty change set for the unchanged enabled kit.

### Bruno — Network consent and Web detail

Enable without consent and with a stale digest both failed with the exact current digest and left
the disabled status byte-identical. The HTTP lane returned 409 with the same structured
`extension_network_confirmation_required` payload. In the real Marketplace UI, the dialog showed
that digest and required an explicit “Confirm and continue” action. After consent, the detail view
showed both resources as `live`, and structured inventory agreed.

### Vera — Duplicate layout diagnostics

The valid resource-only fixture returned zero validation issues. The deliberately invalid fixture
failed before mutation and named `layouts/alpha.json`, `layouts/beta.json`, and the duplicate
`two-up` ID in one diagnostic.

## What Was Fixed

- BUG-20260803-extension-preview-layout-identity — preview now derives the authored window-layout
  name from the canonical resource ID instead of comparing the materialized full ID with the short
  authored name. The owning daemon suite now covers an unchanged enabled layout.

## Paper Cuts

None recorded.

## Runtime Errors Observed

Only the two intended Network-consent refusals and the intended duplicate-layout validation failure
were observed. Browser console and page-error inspection were clean.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- Resource identity must be checked after the same materialization step used by publication;
  canonical JSON equality alone cannot join desired and live records when their names diverge.
- The structured preview, confirmation dialog, and live inventory now form one auditable chain from
  inert install to informed consent.

## Experiential Lens Pass

- **Visual hierarchy:** The kit inventory groups skill and layout counts clearly and distinguishes
  `shipped` from `live` without using color alone.
- **Interaction clarity:** The Network dialog names the extension, explains the side effect, exposes
  the exact digest, and offers distinct cancel/confirm actions.
- **Accessibility:** The inventory, toggle, and dialog expose semantic headings, switch state, and
  labelled buttons in the accessibility tree.
- **Responsiveness:** The 1440×900 deterministic capture preserved complete manage and inventory
  context without overlap or clipped controls.

## Evidence

- Bootstrap: `/Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/bootstrap-manifest.json`
- Structured lifecycle: `/Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/install-review-kit.json`,
  `/Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/preview-cli.json`, `/Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/preview-enabled-retest.json`,
  `/Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/inventory-enabled.json`
- Network refusal and consent: `/Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/enable-missing-consent.txt`,
  `/Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/enable-stale-consent.txt`,
  `/Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/review-kit-network-confirmation.png`
- Web: `/Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/review-kit-inventory-disabled.png`,
  `/Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/review-kit-inventory-enabled.png`,
  `/Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/ui-capture/review-kit-detail.png`
- Manifest validation: `/Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/validate-review-kit.json`,
  `/Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/validate-duplicate-layouts.json`
- Teardown: `/Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/teardown.json` (`"clean": true`)
- Automated gate: `make gate` escalated to full verification and passed with fingerprint
  `be2355a72dca53855f96de493554dde19590226f`; log `.cache/gate/logs/full-1785733633.log`.

## Final Status

- **Exit gate (affected automated lanes):** PASS — `make gate` (full escalation), fingerprint
  `be2355a72dca53855f96de493554dde19590226f`
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 1 fixed · Friction 0 · Cosmetic 0
- **Coverage:** 4 / 4 scenarios settled
- **Verdict:** PASS; all in-scope behavior and affected automated lanes are verified.
