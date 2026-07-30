# BUG-20260729-explorer-active-home-schema: Explorer bundle was invalid outside the default home

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-31, discover and inspect the authored explorer definition
- **Scenarios:** RT-028
- **Found:** 2026-07-29 · **Report:** docs/qa/reports/2026-07-28-untested-full.md
- **Origin:** Fresh isolated installer, HTTP, UDS, CLI, and browser replay

## Summary

The opt-in `agent-exploration` bootstrap helper wrote the bundled `explorer` definition to literal
`$HOME/.compozy`, even when `COMPOZY_HOME` selected another global registry. The bundled definition
also used unsupported legacy frontmatter keys and omitted required `name`, so installing the same
bytes into the active registry made strict agent discovery reject them.

## Reproduction

1. Set `HOME` and `COMPOZY_HOME` to distinct directories.
2. Run `.agents/skills/agent-exploration/scripts/install-explorer.sh`.
3. Start or refresh Compozy against `COMPOZY_HOME`, then list and inspect `explorer`.

**Expected:** The helper installs one strict global definition under
`$COMPOZY_HOME/agents/explorer/AGENT.md`; HTTP, UDS, CLI, and web discovery expose one valid row and
detail succeeds.
**Actual before the fix:** The helper wrote outside the active registry. The shipped asset declared
`title`, `description`, `ide`, and `access_mode`, omitted `name`, and was rejected by the strict
`AGENT.md` parser.

## Evidence

- Pre-fix diagnosis and rebuilt public-surface replay:
  `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/033-agent-catalog`.

## Fix

- **Root cause:** The skill encoded operator-home and pre-beta agent-schema assumptions instead of
  consuming the runtime's active global-home and strict-definition contracts.
- **Correction:** The asset now uses the strict Compozy agent schema. The helper installs beneath
  `${COMPOZY_HOME:-$HOME/.compozy}`, and the skill instructions plus routed references describe the
  same active-home behavior.
- **Fix commit:** `351f3535`
- **Regression owner:** `docs/qa/scenarios/RT-028.md`; this real public-surface scenario owns the
  development-skill integration. A cross-layer `internal/config` asset test was previously rejected
  because the runtime does not bundle or install this opt-in skill.

## Verification

- `bash -n` passes for the corrected helper.
- A distinct-`HOME` isolated run wrote only beneath `COMPOZY_HOME` and preserved byte identity with
  the bundled asset.
- The active daemon discovered the definition without restart. HTTP, UDS, CLI, and the web Agents
  list/detail exposed exactly one valid explorer with no parser or duplicate diagnostic.
- Missing agent reads still returned matching HTTP/UDS 404 responses. The isolated browser session
  closed cleanly and reported no product console error.

## Compozy Impact Audit

- **Native tools:** no impact; checked agent native create/tool catalogs and capability gates. Agent
  reads remain CLI/HTTP/UDS-owned and no native tool ID, descriptor, schema, or fallback changed.
- **Extensibility and hooks:** the `agent-exploration` skill asset and bootstrap lifecycle now honor
  the active global registry. Workspace overrides, extensions, hooks, capabilities, tool/resource
  registries, bundles, bridge SDKs, MCP sidecars, and `config.toml` keys are unchanged.
- **Workspace data isolation:** `explorer` is global-scoped under `COMPOZY_HOME`. Workspace discovery
  still resolves workspace/additional roots before global and returns the same global definition to
  each authorized workspace without duplicating or reclassifying it.
- **Official Compozy skill:** no update required; checked
  `skills/compozy/references/agent-definitions.md`, which already names `$COMPOZY_HOME` as the global
  registry and documents the strict fields used by the corrected asset.
