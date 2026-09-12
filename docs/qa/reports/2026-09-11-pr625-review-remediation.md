# PR #625 review remediation

Scope: all CodeRabbit inline and general findings, Greptile inline and outside-diff findings,
React Doctor diagnostics, and rebase onto `origin/main` at `bfd56223b`.

## Review inventory

| Source | Finding | Disposition |
| --- | --- | --- |
| CodeRabbit 3994240648 | Theme selector specificity | Existing fix retained; canonical regression passes. |
| CodeRabbit 3994240654 | All style blocks contribute raw hex counts | Existing fix retained; canonical regression passes. |
| CodeRabbit 3994240658 | Section class attribute order | Existing fix retained; canonical regression passes. |
| CodeRabbit 3994497439 | Body token inheritance | Root and body cascade separately; body overrides inherited root tokens. |
| CodeRabbit 3994497444 | Exact slide theme tokens | Exactly one light/dark class required; hero is independent; rhythm uses parsed tokens. |
| CodeRabbit 3994497450 | Missing wake session | Empty/whitespace session IDs return HTTP 400 before wake service invocation. |
| CodeRabbit 3994497452 | Specific status/wake spy assertions | Each method has explicit expected counters and separate response assertions. |
| CodeRabbit 3994497454 | Soul rollback error code | Sentinel assertion retained and revision_not_found authoring code asserted. |
| Greptile 3994226058 | Unenforced lint exceptions | Existing typed schema and exact per-artifact finding-ID predicate retained and verified. |
| Greptile general 5641865386 | Authored files ignore Profile | Existing end-to-end Profile propagation retained; rebase preserves main session identity handling. |
| React Doctor | Create/settings hook complexity | Provider selection and editor projections extracted; changed-file scan 100/100, zero findings. |
| CodeRabbit general | Docstring coverage threshold | Not applied: generic 80% coverage conflicts with the repository's explicit comment policy (eng-code-guidelines/references/coding-style.md, Comments). Keep short comments for non-obvious invariants; do not add repetitive comments to 170 functions. |
| CodeRabbit embedded OpenGrep | Command injection at styleRe.exec | False positive: this is RegExp.exec on HTML, not child_process execution. No shell command is constructed. |
| Greptile general 5642189064 | 100-file review limit | Service coverage limit, not a code finding. All published findings above are accounted for. |

## Rebase

Backup: `backup-rebase-codex/open-design-20260911_220846`, original HEAD `75158a943`.
All three commits replayed without squash/drop. The change-impact conflict retains both audits.
Profile conflicts preserve request and authorized-session resolution, Profile-scoped wake events,
source-scoped history, and both canonical regression suites. Duplicate fields from automatic
merges were removed. The resulting patch series was inspected with `git range-diff`.

## Verification

- Linter regression: 111 pass / 8 fail before correction; 119 pass / 0 fail afterward against the generated shipped bundle.
- Missing-session regression: omitted and whitespace IDs reached the wake service before correction; both reject with 400 afterward.
- Existing registry-ID fixture now supplies the required owned session while retaining its storage identity assertion and adding status/body checks.
- Root Turborepo focused hooks: 2 files / 16 tests pass.
- Go race checks: authored-context core/transport and resource catalog suites pass; Soul and bundled extension suites pass.
- React Doctor changed-file scan: 93 → 100, two warnings → zero.
- Local gate: `make gate`; current records are available through `make gate-status` and `.cache/gate/`. Current-head CI remains authoritative on PR #625.

## Targeted public walk

Persona: Bruno, an operator calling the documented API to inspect Heartbeat policy and request
an advisory wake, and linting a deck before independent review. Tour: validation boundaries.
The lab uses an isolated home/port/socket; no provider session or prompt is needed for malformed
wake requests. Existing visual and Profile-editor QA remains applicable to unchanged behavior.

| Scenario | Scope | Status |
| --- | --- | --- |
| RT-077 | Omitted/blank wake session rejection, adjacent status read | Pass |
| ET-open-design | Native lint of body tracking and exact slide themes | Pass |

Lab manifest: `/Users/pedronauck/dev/qa-labs/compozy-pr625-review-20260912-011312-313207-lab/qa-artifacts/qa/bootstrap-manifest.json`.


Public walk: both malformed wakes returned 400 with the expected validation message; adjacent
Heartbeat status returned 200. The native linter found the body tracking and compound-theme defects,
then accepted the corrected deck with an independently matching SHA-256. All five actions passed.
No new live-provider or browser claims are made. The exact public results and refinement are in
`qa/public-walk.json` and `qa/refined-lint.json` beside the lab manifest.

Lab teardown completed with `clean: true` and no surviving owned process (`qa/teardown.json`).
The strict audit result is retained beside the manifest in `qa/qa-audit-report.json`; its final
local-gate evidence is populated after the gate finishes.

The first broad Go race pass hit the unchanged five-minute migration-fixture deadline in
`TestGlobalDBToolApprovalGrantMigration/Should_preserve_durable_grants_through_the_head_upgrade`
while other local checks ran. The exact case passed alone in 35.195s without code, fixture, or
timeout changes. The migration and fixture match main. The complete gate is repeated without
competing frontend work; this is a timing-failure diagnosis, not a claimed production fix.
Root Turborepo completed all 24 tasks; Web passed 773 files / 7,298 tests.
