# QA Run Report — PR 551 profile tool isolation

- Scope: selected-profile operator tools and toolsets rebased onto main `eacd0cb53b9cf4b72fb61fb5db6a75868a3a48ce`.
- Tier: targeted; public CLI, HTTP API, and extension runtime.
- Persona: Bruno, desktop, en-US, local connection.
- Status: public walk complete; PR CI pending.

| Charter | Scenario | Tour | Verdict |
| --- | --- | --- | --- |
| CH-extension-command-authority | ET-profile-workspace-tool-isolation | Feature Tour | Pass |

## Results

| Action | Observed result |
| --- | --- |
| Scaffold, build, and link the Go greeting extension | Public authoring commands activated a real extension subprocess in the owner workspace. |
| List through CLI and HTTP | One greeting tool in work/owner; zero in work/peer and default/owner. |
| Search, inspect, and invoke in owning scope | Search found the tool; inspection returned its schema; invocation completed with `No results for quarterly plans`. |
| Inspect and invoke from peer/default | Each command returned a nonzero structured denial. |
| Refresh owning catalog | The same tool remained visible in a new CLI request. |
| Toolset list and detail | Both CLI list and HTTP list/detail succeeded with the selected profile. |
| Mint approval | Public CLI returned a grant for the selected tool/profile/workspace/session/input. |
| Select archived profile | All seven operator routes returned HTTP 409 with `profile_archived`. |
| Select all profiles or an unknown profile | HTTP 400 and 404 respectively. |

## Session debrief

Bruno found, inspected, and used the workspace extension through the public CLI, then checked the
same catalog through HTTP. Peer and default selections excluded its profile-bound resource.
No product defect or sharp paper cut was found. Initial setup used the stock template, whose tool
has no profile placement and intentionally appears in every profile. Declaring `Profile: "work"`
and reloading through the public extension command established the intended fixture; every final
isolation claim uses that generation. A capture script initially looked for invocation status inside
`result`; the public payload places it at the top level. The corrected capture did not change the product.

## Validation and limits

Race-enabled core, specification, and approval-store suites passed. They separately prove active-token
cross-profile/workspace rejection, owning-scope single use, and replay denial; the public walk used a
read-only extension, so it does not claim a live approval-required action. Test-shape checks passed for
core and approval tests; the spec-file heuristic flagged an unchanged pre-existing test outside this diff.
No provider or browser session was part of this bounded operator journey.

## Evidence receipts

Exact command captures, inputs, outputs, and timestamps are retained in the isolated lab and indexed
by the external PR delivery report. No credential or raw approval token is published here.

- `walk-summary.json` SHA-256 `23fd89def1d08357b9e0e80f16d756710c056a76c2a482a949fd00b20108e4c2`.
- `owner-invoke.json` SHA-256 `8d57e2ee747945a1d2a9b406e0ffc34a97cbf5a327b21f82c5043b0e52088c07`.
- `archived-invoke.json` SHA-256 `e862347c71e9b67510a088066ff787a411bd387c07917a398064a0e234ea959e`.

## Final status

Public walk PASS. Local `make gate` passed all affected lanes, including 7105 Web tests. Site typecheck, 329 tests, and build passed through root Turborepo. Teardown reports `clean: true`; exact-head PR CI remains the delivery gate.

- Local gate evidence: `/tmp/compozy-pr-551-560-herdr-20260909/pr-551-gate.log`.
- Teardown evidence: isolated lab `qa/teardown.json`, `clean: true`, no survivors.
