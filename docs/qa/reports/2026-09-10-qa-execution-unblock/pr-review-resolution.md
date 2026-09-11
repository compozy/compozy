# PR #624 review remediation

The user expanded the delivery scope to resolve every CodeRabbit, Greptile and React Doctor finding. Remaining persona QA stays open; this review pass does not change the original 357-case accounting.

## Finding register

| Source | Finding | Resolution and evidence |
| --- | --- | --- |
| [CodeRabbit 3993601029](https://github.com/compozy/compozy/pull/624#discussion_r3993601029) | Missing settled edge after prompt startup failure | Capture the running snapshot when activity starts and publish settlement after cleanup on every failed setup exit. The existing badge-wait suite reproduced one missing edge and now covers both provider startup and delivery-preparation rejection. |
| [Greptile 3993820142](https://github.com/compozy/compozy/pull/624#discussion_r3993820142) | Startup, managed driver attachment and delivery failures leave lifecycle subscribers running | The shared deferred failure publisher covers all three exits before ownership transfers to the persistence pump; normal completion retains its existing publisher. The same canonical wait suite owns this invariant. |
| [CodeRabbit 3993601038](https://github.com/compozy/compozy/pull/624#discussion_r3993601038) | Evidence and Cause micro-labels bypass the design primitive | Both labels now use the existing Eyebrow component. Adjacent values, accessible text and layout remain in the same positions. |
| [CodeRabbit 3993601041](https://github.com/compozy/compozy/pull/624#discussion_r3993601041) | Goal turns disclosure bypasses Eyebrow | The disclosure label uses Eyebrow; count, toggle, retry and pagination behavior are preserved. |
| [CodeRabbit 3993804612](https://github.com/compozy/compozy/pull/624#discussion_r3993804612) | JSON duplicate detection rounds adjacent integers above 2^53 | Both payloads decode with json.Decoder.UseNumber, with trailing data rejected. The existing hosted-proxy suite verifies that 9007199254740992 and 9007199254740993 retain distinct preview and exact structured text; equivalent reordered JSON remains deduplicated. |
| [CodeRabbit 3993804607](https://github.com/compozy/compozy/pull/624#discussion_r3993804607) | Terminal lifecycle helper has no caller | False positive. TestTerminalBehavioralConformance passes assertTerminalLifecycleHandlers directly to t.Run("Should preserve the polled terminal lifecycle [IT-011]", ...). A focused JSON test run confirms that exact subtest executes and passes. No duplicate wrapper or test was added. |
| [CodeRabbit additional security note](https://github.com/compozy/compozy/pull/624#pullrequestreview-5183838389) | Non-default native Profile silently falls back when its reader is missing | Native resource resolution now rejects that dependency mismatch before resolving the workspace. The existing native Heartbeat suite covers the missing-reader path plus default and selected Profile policies. Production boot already injects state.profiles through native_tools_dependencies_builder.go; the guard protects incomplete constructions too. |
| [CodeRabbit additional identity note](https://github.com/compozy/compozy/pull/624#pullrequestreview-5183838389) | Confirm the provenance of DesiredSessionID | Verified without changing authorization. DesiredSessionID is absent from public creation DTOs and native creation input. Its production producer is Goal binding, deriving the ID from run/workspace/generation/node/item/handle/epoch and carrying checkpoint/control fences. Session creation validates the ID and reserves active/pending identity before startup. ACP terminal requests do not choose the Compozy scope ID. |
| [CodeRabbit docstring check](https://github.com/compozy/compozy/pull/624#issuecomment-5640821264) | Insufficient documentation of touched functions | Added concise intent/contract comments to 46 previously undocumented touched production functions. The local Go/TypeScript AST audit finds no remaining undocumented named functions intersecting changed production hunks. The remote check remains authoritative for its own coverage calculation. |
| [React Doctor](https://github.com/compozy/compozy/pull/624#issuecomment-5640817028) | LoopRunRegisters complexity | Separated actionable footer construction, selected-node presentation and the distinction between full-event reads and the notable-event fallback. Lane/round/selection ownership remains in the original component. |
| [React Doctor](https://github.com/compozy/compozy/pull/624#issuecomment-5640817028) | LoopRunNeedsYouCard complexity | Extracted the approval decision region from the shared requests/quarantine shell, retaining gate selection, decision callbacks and conditional separators. |
| [React Doctor](https://github.com/compozy/compozy/pull/624#issuecomment-5640817028) | LoopRunDetail complexity | Separated window topbar publication and coordinated control overlays from the data/page orchestration. Hook order, run reset, historical controls and quarantine dialog nesting are preserved. |

The second CodeRabbit review also included nine LGTM observations and a correct note that Go 1.26 range variables need no capture workaround. All were read; none requires an additional change.

## Verification

- React Doctor 0.9.13, schema 3, Web project, unfiltered changed scope against main at 35cad0cdd: 93/100 with three warnings before remediation; 100/100, zero errors and warnings afterward. Project coverage reports complete. No rule or severity was disabled.
- Root Turborepo typecheck passed. The existing Loop run page and OS run-detail component suites passed all 175 tests.
- Session badge waits cover failed startup and failed delivery after the running edge; the original success/visibility cases remain intact.
- Hosted MCP helpers cover exact large integers and equivalent JSON; native Heartbeat tests cover Profile isolation and missing dependencies.
- The real-daemon portable-extension and complete implement-tasks E2E suites passed in 42.895 seconds after the review fixes. Their provider is the existing ACP fixture, distinct from the earlier real Cursor QA evidence.
- The targeted ACP JSON test run proves that the alleged uncalled lifecycle subtest executes.
- Required make gate and current-head PR CI are recorded in the PR delivery status; a passing local check is not a remote CI claim.

## Scope and impact

Existing native/HTTP/UDS/CLI/extension surfaces retain their schemas and IDs. Prompt subscribers receive the missing settled lifecycle edge, MCP text retains exact numeric values, and native resource lookup fails closed if a non-default Profile cannot be resolved. No persisted data migration or configuration change is needed. Official skill workflows remain valid. UI refactoring preserves existing operations and uses the established micro-label primitive.

Original QA remains 17 passed, 2 unresolved defects, 2 platform blockers and 336 incomplete cases. The closed real-provider lab was not restarted. Raw local evidence publication remains open under the previously documented Skeeper boundary.
