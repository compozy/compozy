# QA Run Report — 2026-09-13 — marketplace-catalog

- **Scope:** Marketplace v3 hard cut, source acquisition, extension inputs/owned MCP state, preserved installed extensions/manual MCP/local skills.
- **Verdict:** PASS for selected QA; final delivery gates/review/CI remain pending.
- **Build:** e60d78ea11366526d0d156bd5d8f96addc136017 plus the owned task10 repairs in the working tree.
- **Environment:** isolated production Web/daemon and production site; local package and OAuth protocol fixtures. No external provider account login claimed.
- **Started:** 2026-09-13T18:56:19Z.
- **Plan:** `.compozy/tasks/marketplace-catalog/memory/qa.md`.
- **Lab:** `/Users/pedronauck/dev/qa-labs/compozy-marketplace-catalog-final-20260913-185619-918531-lab/qa-artifacts/qa`.

## Session Results

| Charter | Persona | Result | Observed outcome |
|---|---|---|---|
| CH-marketplace-under-a-minute | Bruno | PASS | Browse/search, optional and required input acquisition, installed contents, update/removal, source registration and recovery |
| CH-marketplace-scope-isolation | Bruno | PASS | Workspace/profile requests, owner-qualified OAuth, manual same-name MCP isolation, retained sources/installed origins and exclusive-secret cleanup |
| CH-agent-marketplace-parity | Ada | PASS | CLI/HTTP/UDS/native parity, retired contracts refused, published v3/site, real SQLite upgrade and reopen |
| Adjacent preservation canary | Bruno | PASS | Local skills/source exposure, disabled embedded Marketplace MCPs with preserved content/provenance, manual MCP retention |

## Behavioral Evidence

All paths below are relative to `docs/qa/evidence/2026-09-13-marketplace-catalog/` unless stated otherwise.

- **Acquisition:** Context7 installed with its optional key blank and reports Running (`context7-installed.json`). Brave required a workspace/key, retained its input across restart, updated2.1.0to2.1.1 and remained Running (`brave-installed.json`, `workspace-update-after.json`). The final input summary displays the captured workspace/profile and Vault guidance; cancellation installed nothing (`install-summary-destination.png`).
- **Owned state:** local OAuth completed for extension-owned sentry.sentry while manual sentry remained separately ready. Removing Brave deleted only its scoped installation and exclusive secret; seven other extensions, Sentry tokens and both Sentry runtimes remained (`vault-before-removal.json`, `vault-after-removal.json`, `inventory-after-workspace-removal.json`, `sentry-extension-sentry-after-brave-removal.json`, `sentry-manual-after-brave-removal.json`). This is local protocol proof, not external Sentry account access.
- **Sources:** off/on changed Browse; an unreachable folder retained cached4plugins; removal retained installed review-tools and other extensions; a foreign ref under the retained name was refused; the original ref re-added as restored-team rejoined by ref (`source-degraded.json`, `source-removed-inventory.json`, `source-readded.json`). The competing origin reports Name in use.
- **Digest and config races:** E2E-004 rejected an old approval with409 and no install, then required fresh confirmation. E2E-005 preserved registered sources across installation/config application. Corrected tests retain the original refusal, no-mutation and preservation assertions. Logs `.cache/marketplace-task10-digest-rewalk2.log` and `.cache/marketplace-task10-browser-rewalk.log`.
- **Catalog freshness:** expired healthy snapshots are distinct from failed sources. The live browser performed two successful reads and showed14entries without a stale notice or manual Refresh (`background-refresh-after.json/png`). The lab TTL was restored to1h.
- **Retired navigation:** `/marketplace/mcps?q=saved` stayed at the refused address on reload; both window identities/geometries remained intact. Back deliberately returned to `/marketplace?q=saved` (`retired-web-{before,reopened,recovered}.json`). Canonical component tests also preserve desktop, placement, unrelated fields and saved two-segment locations.
- **Structured surfaces:** scoped CLI/HTTP/UDS/native catalog results match revision, origin and conflicts (`parity-*.json`). Retired routes reject24HTTP/UDS cases; old CLI/native kind inputs are refused (`retired-routes.json`). Actual CLI detail, JSONL and TOON retain current identities/page data (`cli-info-context7.json`, `cli-search-{jsonl,toon}.txt`).
- **Site:** production search narrowed20extensions toContext7; detail shows Inputs/Provenance. Bundled/bridge routes return200; retired MCP/skill routes return404 (`site-{search,context7}.png`, `site-readback.json`, `site-adjacent-routes.json`). Root Turbo production build passed49.881s.
- **Upgrade/publication:** unchanged canonical v109to117 migration/repeated reopen passed0.323s. IT017 runs the actual publisher, serves its untouched output, reads all20entries through HTTP/UDS after reopening the daemon home and refuses root-only feeds (`.cache/marketplace-task02-published-daemon-final.log`,20.741s).
- **Adjacent skills:** five current browser skill-source cases passed. Unchanged real-file registry integration preserves SKILL.md, enablement and ClawHub provenance while excluding its embedded MCP (`.cache/marketplace-task05-mcp-retirement-integration.log`,1.053s). The exact invariant was re-inspected in TestRegistryIntegrationRefreshPromotesSidecarBackedSkillToMarketplace.

Nine selected browser cases passed across the owning re-walks: five skill-source cases, dev overlay/local source-union, current Browse, source retention, and digest approval. Existing integration receipts are reused for unchanged code; they are not represented as newly executed browser journeys.

## Visual Contracts

PASS:41accepted rendered reference/implementation pairs cover all23task/VCrows. Fable inspected all four images per pair; the owning bundle validator passed every accepted pair. `visual-contract/accepted-matrix.json` records exact targets/source identities and `visual-contract/review.md` maps all rows. Historical failures remain in their original directories and are superseded by the repair matrices. Canonical boards were not changed.

The approved states include all logo rungs, shelf9/+3 and both densities, source search, Installed one-column/count/updates-in-flight, required/optional/invalid inputs, owned Server detail, source failure folds, all Add dialog stages and Name in use. Runtime facts, shared component anatomy and OS chrome retain their recorded owners.

## Fixed Defects

Ten verified bugs are recorded under `docs/qa/bugs/BUG-20260913-marketplace-*.md`: install destination, instance mutation scope, workspace Dev state, plugin inventory panic, name-conflict trail, collision recovery copy, search/loading visuals, config replay, stale version/digest conflict, and background refresh. Each has owning checks and affected re-walk evidence. Additional visual corrections restore Installed count/layout and neutral empty actions through existing primitives.

Fixture/capture corrections are separate: contained plugin executable, current retired-test URLs, declared listing description, installed inventory handler, unoccupied Found source, and CDP focus emulation for blur-driven stories. No production safeguards were relaxed.

## Verification Boundaries and Teardown

Focused owning tests, Web/UI typechecks, site/Web production builds and scoped formatter/linter checks passed. Final `make gate`, branch review and current-head PR CI remain the enclosing delivery stage's obligations. No PR-delivery claim is made here.

The strict QA auditor initially reported only the missing final verification report/local gate receipt (C12/C14); C99 notes that its automatic API equality check is not implemented, so the captured cross-plane evidence owns that assertion. Final gate evidence and the audit refresh will be recorded during delivery.

Lab teardown completed at2026-09-13T21:27:43Z. `teardown.json` reports clean:true and no survivors; all owned daemon, fixture, site, Storybook and reference-server processes stopped. Both Fable tabs were retired. No human decision is outstanding.


## Post-QA Review Repairs

The enclosing review repaired mixed-source background refresh, omitted-source lookup order, retained development logs, secret-input env-binding changes, native Marketplace acquisition/workspace selectors, authorization feedback, partial batch progress, and Installed descriptions/profile ownership. The real daemon integration now exercises native pinned Marketplace installation (25.469s); owning SQLite/race suites verify input persistence, attachment identity and rollback. Receipts and review findings are recorded in `.compozy/tasks/marketplace-catalog/memory/peer-review.md`.

Targeted Storybook browser validation covers the changed authorization toast, local descriptions/profile qualifiers, 0-of-2 and 1-of-2 progress, and completed-group controls. Two inspected reference/implementation comparison bundles and the toast image are under `.compozy/tasks/marketplace-catalog/evidence/visual/review-01/`. The partial-progress repair supersedes the earlier waiver of k-of-n progress; the reference boards are unchanged. These presentation probes use controlled HTTP fixtures and are distinct from the real daemon integration. Prior evidence is reused only for unaffected journeys and visual states.

The final gate exposed an optional metadata lookup that invalidated otherwise valid runtime snapshots after their attachment was absent. The projection now leaves that optional field unset for the typed missing-attachment result and continues returning genuine database errors; read authorization remains in the owning resolver. The unchanged inventory/readiness suites own the regression check. Final delivery gates, review verdict and current-head PR CI remain pending.

The navigation follow-up also preserves a row's profile/workspace in its detail link. Canonical component/palette coverage passed66 tests; a real browser click confirmed engineering/global and default/workspace request targeting (detail-scope.json). The controlled story lacks sideload detail content, so this records navigation identity only; the earlier daemon integration owns actual scoped detail/mutation behavior.
