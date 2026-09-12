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
| [CodeRabbit docstring check](https://github.com/compozy/compozy/pull/624#issuecomment-5640821264) | Insufficient documentation of touched functions | Added concise intent/contract comments to 46 previously undocumented touched production functions. The follow-up documents 25 touched Go test functions/helpers. The recursive TypeScript audit then adds contracts to 14 touched nested callbacks, helpers and constructors that the initial top-level audit missed. Emitted JavaScript is identical before and after those comments. The remote check remains authoritative for its own coverage calculation. |
| [React Doctor](https://github.com/compozy/compozy/pull/624#issuecomment-5640817028) | LoopRunRegisters complexity | Separated actionable footer construction, selected-node presentation and the distinction between full-event reads and the notable-event fallback. Lane/round/selection ownership remains in the original component. |
| [React Doctor](https://github.com/compozy/compozy/pull/624#issuecomment-5640817028) | LoopRunNeedsYouCard complexity | Extracted the approval decision region from the shared requests/quarantine shell, retaining gate selection, decision callbacks and conditional separators. |
| [React Doctor](https://github.com/compozy/compozy/pull/624#issuecomment-5640817028) | LoopRunDetail complexity | Separated window topbar publication and coordinated control overlays from the data/page orchestration. Hook order, run reset, historical controls and quarantine dialog nesting are preserved. |
| [CodeRabbit 3994021248](https://github.com/compozy/compozy/pull/624#discussion_r3994021248) | Extracted native wrappers omit intrinsic props | The selected-node panel and approval decision region extend native div props, merge className and forward the remaining React 19 props, including ref. |
| [CodeRabbit 3994021251](https://github.com/compozy/compozy/pull/624#discussion_r3994021251) | Disabled Goal history looks busy indefinitely | The Goal history hook projects isLoading instead of isPending. The existing Loop read-hook suite reproduces the disabled-query failure and verifies disabled, actively fetching and settled states using the real Query client. |
| [CodeRabbit 3994021261](https://github.com/compozy/compozy/pull/624#discussion_r3994021261) | Ledger presentation inspects adapter errors | The session ledger hook classifies unavailable reasons; the window controller projects that reason and unexpected errors into InspectorMemoryState. Presentation consumes that view model. Existing session read and inspector suites cover unavailable reasons and genuine read failures. |

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

The follow-up Go test-shape heuristic produces exactly the same findings as the published baseline for every documentation-only test edit. No heuristic rule or existing test assertion was relaxed.

## CI Marketplace acquisition race

CI run 34655209602 passed all lanes except Web shard 2, where acquisition returned HTTP 200 but the filtered card retained installed=false. The retained browser trace shows the initial search overlapping installation, with no subsequent Marketplace reread; the separate skills list already contains the installed artifact. An existing action-hook test reproduces a pending initial search resolving with pre-install data after mutation settlement. Marketplace invalidation now cancels older reads first, then rereads the canonical query family, retaining server scope and page envelopes. No timeout, retry, expected installed state or E2E assertion was relaxed.

Validation: all eight Marketplace action-hook cases pass after the repair. The unchanged `operator acquires marketplace capabilities against one real daemon` E2E passes with a freshly built daemon and Web bundle in 24.0 seconds (28.4 seconds total), under the existing machine-wide verification lock. Original QA inventory counts remain unchanged.
