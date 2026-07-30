# QA Run Report — 2026-07-29 — site-improvs deep review

- **Scope:** Full `site-improvs` branch review remediation, including content-changing MDX, generated CLI reference, public command hard cuts, docs/Marketplace UI, and changed workspace-policy paths.
- **Cadence tier:** targeted
- **Build:** `ed2edf2a` + working tree · **Environment:** isolated lab `site-improvs-deep-review-20260730-024918-833208`, local site, CLI/HTTP/UDS; external beta channels read-only/install-to-disposable-prefix only
- **Started:** 2026-07-29T23:50:06-03:00 · **Status:** complete — conditional pass

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Dora | Power User | desktop / wifi-fast / en-US | CH-site-docs-marketplace-truth, CH-memory-extractor-hard-cut, CH-beta-install-channels, CH-compozy-beta-candidate |
| Cora | Casual User | laptop / wifi-fast / en-US | CH-site-protocol-plain-language |
| Ada | Power User | structured surfaces / wifi-fast / en-US | CH-model-catalog-guidance-parity, CH-agent-marketplace-parity, CH-cross-workspace-mode-seams |
| Iris | Power User | laptop / wifi-slow / en-US | CH-remote-operator-manual-auth |
| Bruno | Power User | desktop / wifi-fast / en-US | CH-cross-workspace-consent-audit |
| Vera | Power User | desktop / wifi-fast / en-US | CH-extension-policy-admin-gates |
| Théo | Power User | desktop / wifi-fast / en-US | CH-network-work-lookup-hard-cut |

## Flows in Scope

- `J-evaluate-compozy-beta` — public docs, Marketplace, protocol truth, and beta installation.
- `J-20` — model-catalog guidance through structured surfaces.
- `J-agent-marketplace-parity` — daemon-owned discovery and MCP authorization.
- `J-mcp-authorize-repair` — automatic and manual OAuth completion without false success.
- `J-cross-workspace-access` — mode decisions, consent lifetime, Network seams, and foreign spawn caps.
- `J-digest-sessions-into-memory` — extractor operator surfaces and failure replay.
- `J-extension-policy-admin` — live curated-feed configuration.
- `J-23` — canonical Network work-item lookup.
- `J-approve-compozy-beta-candidate` — migration-guide parity and legacy disposition.

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-remote-operator-manual-auth | J-mcp-authorize-repair / ET-047 | Iris | Paste Tour | Blocked (needs human verify) | OAuth-capable MCP server unavailable | |
| 2 | CH-model-catalog-guidance-parity | J-20 / ET-053 | Ada | Feature Tour | Blocked (needs human verify) | Missing-active-price-bucket usage case needs seeded priced usage | |
| 3 | CH-agent-marketplace-parity | J-agent-marketplace-parity / ET-cli-marketplace-search | Ada | Feature Tour | Pass | | |
| 4 | CH-remote-operator-manual-auth | J-mcp-authorize-repair / ET-cli-mcp-auth-manual-exchange | Iris | Paste Tour | Blocked (needs human verify) | OAuth-capable MCP server unavailable | |
| 5 | CH-agent-marketplace-parity | J-agent-marketplace-parity / ET-cli-mcp-authorize | Ada | Feature Tour | Blocked (needs human verify) | OAuth credential transition unavailable | |
| 6 | CH-site-docs-marketplace-truth | J-evaluate-compozy-beta / ET-site-docs-api-reference-ui | Dora | Feature Tour | Pass | | |
| 7 | CH-site-docs-marketplace-truth | J-evaluate-compozy-beta / ET-site-docs-examples-wave-one | Dora | Feature Tour | Pass | | |
| 8 | CH-site-docs-marketplace-truth | J-evaluate-compozy-beta / ET-site-docs-first-session | Dora | Feature Tour | Pass | | |
| 9 | CH-site-docs-marketplace-truth | J-evaluate-compozy-beta / ET-site-docs-masthead-opendesign | Dora | Feature Tour | Fixed | BUG-20260730-docs-index-invalid-hydration | working-tree |
| 10 | CH-site-docs-marketplace-truth | J-evaluate-compozy-beta / ET-site-docs-sidebar-opendesign | Dora | Feature Tour | Fixed | BUG-20260730-docs-mobile-sidebar-offset; BUG-20260730-sidebar-close-lost-reload | working-tree |
| 11 | CH-site-docs-marketplace-truth | J-evaluate-compozy-beta / ET-site-docs-single-tree-ia | Dora | Feature Tour | Fixed | BUG-20260730-docs-index-invalid-hydration | working-tree |
| 12 | CH-site-docs-marketplace-truth | J-evaluate-compozy-beta / ET-site-docs-typography-opendesign | Dora | Feature Tour | Pass | | |
| 13 | CH-site-docs-marketplace-truth | J-evaluate-compozy-beta / ET-site-marketplace-bridges-bundled | Dora | Feature Tour | Pass | | |
| 14 | CH-site-docs-marketplace-truth | J-evaluate-compozy-beta / ET-site-marketplace-catalog | Dora | Feature Tour | Pass | | |
| 15 | CH-site-protocol-plain-language | J-evaluate-compozy-beta / ET-site-protocol-trust-status | Cora | Feature Tour | Pass | | |
| 16 | CH-cross-workspace-mode-seams | J-cross-workspace-access / ET-workspace-access-mode-matrix | Ada | Feature Tour | Blocked (needs human verify) | BUG-20260730-tool-invoke-202-empty-success fixed; expanded mode matrix needs seeded harness | working-tree |
| 17 | CH-cross-workspace-consent-audit | J-cross-workspace-access / ET-workspace-access-prompt-outcomes | Bruno | Interrupt Tour | Blocked (needs human verify) | BUG-20260730-tool-invoke-202-empty-success fixed; full approval-lifetime matrix needs seeded harness | working-tree |
| 18 | CH-memory-extractor-hard-cut | J-digest-sessions-into-memory / MS-019 | Dora | Feature Tour | Pass | | |
| 19 | CH-extension-policy-admin-gates | J-extension-policy-admin / MS-marketplace-catalog-live-config | Vera | Garbage Tour | Blocked (needs human verify) | Two isolated feed servers not launched | |
| 20 | CH-network-work-lookup-hard-cut | J-23 / NB-022 | Théo | Feature Tour | Pass | | |
| 21 | CH-beta-install-channels | J-evaluate-compozy-beta / REL-beta-install-paths | Dora | Feature Tour | Blocked (needs human verify) | Post-publication installer/npm/Go acceptance intentionally deferred | |
| 22 | CH-compozy-beta-candidate | J-approve-compozy-beta-candidate / REL-migration-guide-parity | Dora | Garbage Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

- **Docs, Marketplace, and protocol:** Walked desktop and 320px mobile entry points with a real Next.js server. Canonical routes resolved, legacy `/runtime*` and `/protocol*` routes returned 404, Marketplace snapshot claims matched checked-in data, and protocol pages separated shipped v0 behavior from the unimplemented v1 trust profile. Eight visual-contract bundles passed with zero blocking divergences; API, bridge, Marketplace, and protocol implementation captures were also inspected.
- **First session:** A real provider session returned `QA_READY`, appeared in resumable listings, attached through a real TTY, retained durable history, and became non-attachable only after terminal stop.
- **Marketplace surfaces:** CLI, HTTP, UDS, and `compozy__marketplace_search` returned consistent grouped discovery data. Display-name lookup resolved its canonical entry ID. The checked-in catalog path passed; live feed switching remains blocked on the two-feed harness.
- **MCP authorization:** The removed `mcp authorize` command stayed absent and `mcp auth` exposed only `login`, `logout`, and redacted `status`. Fresh PKCE, manual exchange, and credential-isolation walks remain blocked because this lab did not launch an OAuth-capable MCP server.
- **Model catalog:** Curated, all, status, native-tool, reasoning-capability, sparse five-rate pricing, and deterministic unknown-model paths passed. The missing-active-bucket behavior still needs a seeded priced usage record.
- **Workspace access:** An `approve-all` real session crossed to the secondary workspace. The first approval-required CLI call exposed a transport defect, which was fixed and retested. The expanded deny/read/all Network seams, foreign spawn cap, and complete approval-lifetime matrix require the dedicated seeded harness and remain blocked-verify.
- **Memory extractor:** CLI, HTTP, and UDS agreed for idle status, empty failures, drain, and retry. Removed `list-pending` stayed rejected.
- **Network work:** Two real sessions exchanged one directed work item; CLI, HTTP, UDS, and `compozy__network_work` agreed on state, lineage, identifiers, and timestamps. The removed `work status` command stayed rejected.
- **Release guidance:** `make migration-guide-check` passed all eight normalized sections. Local copy exposes the pinned beta channels and no Homebrew path. Live cross-channel installation is deferred to post-publication acceptance to avoid treating an unpublished candidate as released.

## What Was Fixed

- `BUG-20260730-docs-mobile-sidebar-offset` — mobile sidebar geometry now follows the canonical Fumadocs width token.
- `BUG-20260730-sidebar-close-lost-reload` — an explicit folder close now survives reload without active-route expansion overwriting the stored choice.
- `BUG-20260730-docs-index-invalid-hydration` — docs landing markup no longer nests paragraph elements and hydrates without console errors.
- `BUG-20260730-tool-invoke-202-empty-success` — `compozy tool invoke` preserves HTTP 202 approval-required envelopes as structured errors instead of decoding empty success.

## Paper Cuts

None recorded.

## Runtime Errors Observed

- After the Network lookup invariant had passed, the target test agent independently attempted a reply on a different thread and received `backend_unhealthy`. This did not affect the submitted work item's persisted state or the four-surface lookup result, and it was outside the lookup scenario's contract; it remains an observation rather than a filed defect.

## Human Verifications Needed

- OAuth-backed MCP PKCE, manual exchange, credential isolation, and successful credential transition (`ET-047`, `ET-cli-mcp-auth-manual-exchange`, `ET-cli-mcp-authorize`).
- Priced usage with a missing active rate bucket (`ET-053`).
- Full deny/read/all workspace-mode, foreign-spawn-cap, and approval-lifetime matrices (`ET-workspace-access-mode-matrix`, `ET-workspace-access-prompt-outcomes`).
- Live Marketplace source switching against two isolated feeds (`MS-marketplace-catalog-live-config`).
- Post-publication hosted-installer, npm beta-tag, and Go-module parity (`REL-beta-install-paths`).

## Decisions for a Human

None identified.

## Learnings

- The visual-contract preflight found three stale scenario references to a removed OpenDesign directory; the canonical `docs/design/opendesign/site/` artifacts now own those rows.
- A generic HTTP success range is unsafe for APIs that use 202 as a typed control-flow response; the tool-invoke transport now owns that distinction explicitly.
- Content-changing MDX must stay in branch-scale review scope even when the default review glob ignores documentation files.

## Final Status

- **Exit gate (full automated suite):** `make gate-full` runs after this report is frozen; the content-keyed `make gate-status` record is the authoritative result.
- **Issues by user impact:** Blocks-Completion 1 · Data-Loss 0 · Trust-Damage 2 · Friction 1 · Cosmetic 0 — all four fixed and retested.
- **Coverage:** 22 scenarios triaged; 11 passed, 3 fixed and passed, 8 blocked-verify with explicit harness or publication dependencies.
- **Verdict:** Conditional pass — every locally executable claim passed after remediation; eight external or specialized-harness claims remain explicitly blocked-verify rather than being inferred from static coverage.
