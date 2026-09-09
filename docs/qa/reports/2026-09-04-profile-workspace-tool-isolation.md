# QA Run Report — 2026-09-04 — Profile and Workspace tool isolation

- **Scope:** PR #551 operator-tool projection, approval scope, and Profile-resolution response contracts.
- **Cadence tier:** targeted
- **Code build:** commit `40587df371436a9c239dd9f6d4b76b4419af6690` · tree `09b107bb4fbab563baeaa8862540c0373af62da9` · binary SHA-256 `4c79832b9c3df3ef65f4905b97010efce4cee1488eb93de537f370a7003c6e85`
- **Environment:** isolated local daemon with dedicated home, database, Unix socket, HTTP port, Profile, candidate and peer Workspaces, provider home, and workspace-local QA extension source
- **Started:** 2026-09-04T20:26:36Z · **Status:** closed

## Persona and flow

| Persona | Charter | Journey / Scenario | Tour | Status |
|---|---|---|---|---|
| Bruno | `CH-extension-command-authority` | `J-run-extension-commands` / `ET-profile-workspace-tool-isolation` | Feature Tour | Pass |

Bruno listed, searched, inspected, and invoked the QA Lab action from the owning `work` Profile and
candidate Workspace. He repeated the catalog and denied-call checks from the default Profile and a
peer Workspace, then compared the CLI result with the HTTP `GET /api/tools` projection.

## Results

| Claim | Exercise | Observation | Verdict |
|---|---|---|---|
| Owning scope projects lifecycle tools | CLI list/search/info with `work` plus candidate Workspace | Exactly two QA Lab tools, with the selected Profile and Workspace as the registry scope | PASS |
| Peer and default scopes remain isolated | CLI list/info/invoke against the peer Workspace and default Profile | Zero QA Lab tools; info and invoke returned non-zero structured denials | PASS |
| HTTP uses the same scope | `GET /api/tools` for owner, peer, and default | Counts matched CLI: owner 2, peer 0, default 0 | PASS |
| Scoped runtime action uses the candidate source | Invoke `ext__qa_lab__capture_candidate` in the owning scope | Completed with candidate commit `40587df371436a9c239dd9f6d4b76b4419af6690`, tree `09b107bb4fbab563baeaa8862540c0373af62da9`, and clean state | PASS |
| Approval tokens bind both isolation dimensions | Race-enabled public-handler regression | Workspace mismatch and active-token cross-Profile use were denied; the owning Profile consumed the token once; cross-Profile replay remained denied; one registry execution total | PASS |
| Profile-resolution failures are documented | Spec regression plus deterministic generation | All seven Profile-resolving routes expose 409; invoke preserves the Profile-or-tool union; OpenAPI and TypeScript output match generation | PASS |

## Session debrief

- **Findings:** None after setup correction.
- **Setup correction:** The first probe retained a global QA Lab install and therefore projected tools
  into the peer scope. Removing that user-level install established the intended workspace-only
  source. Every result above comes from the post-removal evidence set; this was fixture contamination,
  not a product defect.
- **Bugs filed:** None.
- **Paper cuts:** None.
- **Surprises:** Workspace-local extension development correctly remained available to the owning
  Workspace after the global install was removed.

## Public-safe receipt

The detailed captures remain in the isolated retained evidence directory. These content hashes bind
the report without publishing machine paths or credentials:

| Receipt | SHA-256 |
|---|---|
| `public-surface-final-summary.json` | `0924df7335f653d350adb22774f843aa6d6aa9b52ecafca3a2a3b6ba186de149` |
| `cli-owner-invoke-final.json` | `638d6080b74a5a18bd423f492674d7ada3501ddf970ae822051d307f0739c0b0` |
| `approval-handler-race.log` | `381570ce5d98f20b14d16a4564cff35f2e1fe5e02166d1e0ef794c1a0f08fd9f` |
| `final-make-verify.log` | `5a253950308f4af9be0142e4e96a7515271f557108f67ca24fa12925a7ccae70` |

## Validation and limits

- Focused approval store, public approval/invoke handler, parallel projection, and specification tests
  passed under the race detector, including repeated parallel runs.
- The full `internal/api/...` and `internal/tools/...` suites passed under the race detector.
- `make codegen-check`, scoped Go lint, and `make source-size` passed.
- The proportional `make gate` completed codegen, lint, and Go race lanes. Its Web lane reported 37
  unrelated existing failures in session-thread, channel-hook, and onboarding tests; no changed file
  intersects those suites, and this report does not claim a full local gate pass.
- No browser surface was required for this CLI/API/runtime-only change.

## Final status

- **QA audit:** strict PASS with zero blockers and zero warnings at
  2026-09-04T20:34:59Z; the machine-readable receipt is `qa-audit-report.json`.
- **Teardown:** clean at 2026-09-04T20:35:14Z with zero survivors; the
  machine-readable receipt is `teardown.json`.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** one targeted scenario across CLI, HTTP API, runtime extension execution, permission
  denial, schema contracts, and both Profile and Workspace isolation.
- **Verdict:** PASS — the code build closes the scoped admission claims with a strict evidence
  audit and clean teardown. Exact-head checks and PR CI remain terminal gates.
