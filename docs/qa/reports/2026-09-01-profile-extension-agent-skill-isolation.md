# QA Run Report — 2026-09-01 — Profile extension Agent and Skill isolation

- **Scope:** Profile-owned extension Agent publication and same-name Agent-local Skill precedence.
- **Cadence tier:** targeted
- **Build:** `7c53637c` · **Binary SHA-256:** `ff797bac4f40dc372667af01cddf5059747c3ff48259d6c7b16668afe35577bb`
- **Environment:** isolated home, workspace, HTTP port, UDS socket, and immutable built binary
- **Status:** complete; delivery CI follows the push

## Session

| Charter | Scenario | Persona | Tour | Status |
|---|---|---|---|---|
| CH-profile-extension-agent-skill-isolation | ET-profile-extension-agent-skill-isolation | Ada | Feature Tour | Pass |

## Results

### Profile-only extension Agent

Ada installed a local extension whose manifest placed `finance-extension-only` only in the
`finance` Profile. Structured CLI and HTTP Agent lists both returned zero matches in `default` and
exactly one match in `finance`. The same four reads were repeated after the precedence walk with the
same result.

### Agent-local Skill precedence

The walk introduced the same `layer-agent` and `scope-sentinel` Skill one layer at a time. Every
layer used a distinct description and body. CLI `skill list`, `skill where`, and `skill view`, plus
HTTP skill list and detail, agreed on each winner.

| Context | Winning description | Winning body |
|---|---|---|
| Global only | `global winner` | `global body` |
| Default Profile | `default Profile winner` | `default Profile body` |
| Non-default `finance` Profile | `finance Profile winner` | `finance Profile body` |
| Workspace + `finance` Profile | `Workspace and Profile winner` | `Workspace and Profile body` |

The first walk exposed a real stale-catalog defect: the Agent list selected the Profile Agent while
the Agent-scoped Skill read still loaded the global Agent source. Commit `7c53637c` makes scoped
Skill reads use the already-resolved workspace Agent before the resource-catalog fallback. The
complete four-layer walk passed on the rebuilt binary.

### Negative and recovery probes

- HTTP rejected an empty `for_agent` with status `400` and a specific validation error.
- CLI rejected an unknown Agent with a typed not-found error and non-zero exit.
- The finance extension isolation and Workspace+Profile body were read again after both errors and
  remained correct.

## Verification

- Focused race tests passed for the core resolver matrix and the daemon Profile-only extension
  publisher regression.
- Both owning regressions were mutation-checked: restoring the publisher leak failed the daemon
  isolation test, and bypassing scoped Agent selection failed the distinct-winner matrix.
- `make gate` passed before the implementation commits. The final pre-push gate is recorded in the
  delivery evidence.
- The strict real-scenario evidence audit passed.
- The bootstrap-provided teardown command completed with `qa/teardown.json` reporting
  `clean: true`, an empty survivor list, no listener on the isolated port, and no UDS socket.

Evidence is rooted at `qa-artifacts/qa/`; the key receipts are under `public-cli-api/`,
`qa-audit-report.json`, and `teardown.json`.

## Runtime observations

The lab intentionally had no provider credentials, so provider health probes were degraded. No
provider session was required or started for this CLI/API read-only charter, and the targeted
surfaces returned no unexpected runtime error.

## Compozy Impact Audit

- **Native tools and generated contracts:** no tool ID, descriptor, input schema, registry digest,
  or generated SDK surface changed.
- **Extensibility and hooks:** the existing extension manifest and lifecycle contracts are
  unchanged. The daemon regression now proves Profile ownership at the publisher boundary; no hook,
  extension schema, or SDK contract changed.
- **Workspace isolation and persistence:** the walk proves global, Profile, and Workspace+Profile
  Agent-local sources do not leak across the tested read scopes. No store schema, migration, or
  persistence contract changed.
- **Official bundled Skill:** no bundled `compozy` Skill content or activation contract changed.

## Final status

- **Scenario verdict:** Pass
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Human verification needed:** None
- **Cleanup:** Complete and clean
