---
id: ET-extension-code-first-authoring
area: ET
title: Build a code-first extension from the embedded CLI templates
persona: Ada
journey: J-extension-kit-lifecycle
expected: Running `compozy extension init hello -t tool-provider-go`, then build and validate with declared agents, automation, and layouts, produces one immutable generation whose copied resources and generated manifest match the SDK definition and validate with no issues.
entry_points: `compozy extension init`; `compozy extension build`; `compozy extension validate`
qa_status: blocked-verify
bug_ids: BUG-20260802-scaffold-sdk-version;BUG-20260802-manifest-mcp-tool-handler;BUG-20260729-public-extension-sdks-unpublished;BUG-20260807-extension-template-help
fix_status: partial
retest_status: pass
fix_commits: 7866661;881a254
evidence: /Users/pedronauck/dev/qa-labs/compozy-ext-improvs-final-20260729-230047-267985-lab/qa-artifacts/qa/extension-charters.json;/Users/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260802-195112-911343-lab/qa-artifacts/qa;/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/test-cases/41-extension-template-discovery.json
last_report: docs/qa/reports/2026-08-07-remote-gateway.md
overlaps: ET-compozy-extension-contract-identity
---

Added by ext-improvs Task 03. Repeat the first-success path for all seven embedded templates, confirm `build` never mutates an existing generation, and inspect structured output for the stamped SDK minimum version, positioned issues, and derived consent areas.

QA impact 2026-08-02: code-first `resources` now declares and copies agents, automation, and layouts
into the generated manifest. Reset to verify the complete kit rather than the earlier tool-only build.

QA impact 2026-08-06: Go and TypeScript connectivity-provider templates joined the embedded
scaffold catalog. Flag only; Tasks 08–09 own the re-walk.

QA walk 2026-08-07: both connectivity templates scaffolded and became discoverable in CLI help.
The clean Go build then proved the published SDK lacks their declared API; TypeScript dependency
resolution was blocked by the machine's minimum-release-age policy. External build remains blocked.

Release CI repair: `TC-EXT-005` in `web/e2e/__tests__/extensions.spec.ts` exercises both
tool-provider templates against the checked-out SDKs and a real daemon. Build immutable
generations, install and invoke them, update to a generation with changed tool output, then
disable and remove each extension. Check the installed cards at every standard viewport and
confirm disabled tools and removed extensions disappear from their registries. TypeScript
packages must start without access to the source directory's dependencies. The fixture registry
serves the actual generated archives; it does not replace the extension subprocess or SDK.

Focused re-walk 2026-09-30: `TC-EXT-005` passed against a freshly built daemon and both current
SDKs (one Playwright journey, 17.8 seconds). Each language returned changed tool output after
updating to 0.2.0, disappeared from the callable tool registry on disable, and disappeared from
the installed catalog on removal. This verifies the two tool-provider templates; the older
external-provider blockers above are not reclassified by this local SDK check.
