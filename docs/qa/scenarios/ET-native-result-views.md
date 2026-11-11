---
id: ET-native-result-views
area: ET
title: Select compact and full native tool results
persona: Ada
journey: J-agent-marketplace-parity
expected: Native tools default to compact, full preserves the complete record, selectors fail before execution when invalid, and Loop references select the correct contract.
entry_points: compozy tool info; compozy tool invoke; compozy loop
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/reports/2026-11-10-native-result-views.md
last_report: docs/qa/reports/2026-11-10-native-result-views.md
---

Walk E2E-001–003 in an isolated provider-free ACP lab. Measure status, create,
archive and spawn in both views. Status/archive/spawn must save at least 80%; create
is recorded without a threshold. Reuse compact IDs through the session API. Inspect
output_views via tool info, reject summary/non-string and a selector on skill_view.
Validate a reference to session.lineage.permission_policy.tools: compact must fail
with result_view_field_requires_full naming inspect, full must validate and complete.
Verify hook pin refusals through the owning focused suites. Hosted MCP tools/list must omit
outputSchema for every native tool while retaining result_view inputs. Through real MCP and dispatch,
verify compact/full structuredContent and JSON text, truncated continuations and typed errors.
Read both contracts from tool info; compare catalog bytes against the pre-feature baseline.

Full-record parity checks must explicitly select `result_view: "full"`: extension skill list/search
provenance, provider model catalogs compared with HTTP/CLI, and approval grant set/list records
compared across transports and daemon restart. Keep the complete-record assertions in the existing
daemon integration suites; the default compact projection remains covered by the result-view suites.

Regression checks: shorthand Loop schemas such as `produces: {type: string, id: string}`,
nested `type` field maps, and transform maps resolve their declared fields. Real JSON Schema
keywords remain non-addressable. Run `TestNamespaceSchemaNamedFields` in the refs suite.
`TestRuntimeRegistryDispatchResultViews` verifies selector-bearing audit previews and event
input digests; `TestToolHandlersExposeOperatorSessionInvokeAndToolsets` verifies that the shared
HTTP/UDS invoke handler ignores wire `View` and `PinnedView` before dispatch.

R1b regression checks: run `TestLinterOutputShapeRuntimeAgreement` and
`TestLinterResultViewReferences`. Keyword-only transform maps retain their fields; authored
`produces`, `expect`, and `output_schema` shorthand with `properties` or `type: array` plus
`items` follows runtime normalization. Explicit open objects permit unknown fields with a warning;
historically accepted authored scalar/metadata references validate with cannot-be-proven warnings.
Annotated tool schemas keep strict declared-property
resolution, including `$comment`, `$ref`, `patternProperties`, `else`, and extension annotations.

R2/R3 regression checks: tools with result views and any `produces` declaration remain bounded by their
pinned descriptor view for current, previous and best references; declarations can only narrow
the contract. Non-tool inferred input/source/sub-loop references retain baseline acceptance,
using cannot-be-proven warnings when only the legacy field-map lookup accepts a path.
`TestLinterOutputShapeRuntimeAgreement` also compares 93 generated authored definitions with
the baseline field-map acceptance rule. List/range warnings name the complete reference.
`TestGenerationHistoryProjection` pins exact first-generation previous/best defaults across
node sources, including legacy shorthand quirks, null transform fields, template rendering,
and CEL null guards. Schema normalization for lint must never change these runtime defaults.

R3 declaration checks: tool `produces` retains baseline raw field-map/list acceptance and its
own scope. Tools without result views (extensions, MCP, and native single-result tools) do not
add a descriptor bound. Tools with views intersect that declaration with the selected descriptor;
full-open acceptance must warn even when the declaration proves the field. The result-view suite
covers templates, transform `from`, ranges, and current/previous/best paths; the generated
compatibility comparison includes single-result, compact and full tool producers.

M1 integration with bounded native reads: tool search retains `offset`, `count`, `total`,
`has_more` and `next_offset` in both views, omitting optional zero fields identically.
`TestActionRegistryResultViewPins` verifies translation of current-main pre-view pins for
`tool_search` and `worktree_list`. Pins predating the bounded-read input changes remain stale,
as documented in `docs/release/native-read-defaults.md`; result-view translation does not
waive an unrelated input-contract change. Automation histories remain single-result pages.

P1 regression checks: external MCP and extension tools retain their advertised output schemas,
including continuation and error envelopes. Schema updates must re-register the tool through
`TestHostedExternalOutputContracts` over real MCP transport. Compact and full Loop references
follow declared union, nullable-array, and local-reference paths for current, previous, and best
with or without `produces` (`TestLinterBuiltinComposedResultViews`).
`TestBuiltinResultViewSchemaPaths` proves every declared catalog path to bounded depth;
`TestNamespaceComposedSchemaProof` checks branch proof and closed-object rejection.
