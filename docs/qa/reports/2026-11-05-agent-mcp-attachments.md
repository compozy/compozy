# Agent MCP attachment QA

**PASS — issue #1305, Gap 2.** Dora configured an agent through API, UDS CLI and a headless Web browser in an isolated targeted lab. No live desktop window or production daemon was driven. Provider inference and external MCP endpoint execution are outside this configuration journey.

| Journey | Result | Retained lab evidence |
| --- | --- | --- |
| Pick configured server, Save and reload | Pass; sidecar contains a name reference, allowed tools initially unchanged | `evidence/api-readback.json`, `evidence/cli-readback.json`, `evidence/web-attached.png` |
| New HTTP connection with Vault header | Pass; reload retains connection, reads expose key presence only, agent files contain no plaintext | `evidence/web-new-server.png`, `evidence/final-readback.json` |
| Update/remove configured source | Pass; sidecar keeps no endpoint copy, removed source displays Broken reference without disabling the agent | `evidence/legacy-and-live-reference.json`, `evidence/web-broken.png` |
| stdio binding and two unrelated Saves | Pass; both Saves return 200 and preserve all three then-current attachments and their digest | `evidence/stdio-readback.json`, `evidence/web-final.png` |
| Existing frontmatter and sidecar | Pass; additive upsert preserves canonical AGENT.md bytes and complete existing sidecar data | `evidence/legacy-and-live-reference.json` |
| Package ownership and stale digest | Pass; Add is absent, crafted package write is 403, stale write is 409 without Vault mutation | `evidence/package-readonly.json`, `evidence/web-readonly.png`, `journey-log.jsonl` |
| Discard and remove one pending addition | Pass; discard writes no credential; removing one staged server retains another draft edit | `evidence/web-final.png`, `journey-log.jsonl` |
| Allowed-tool canary and final CLI/API agreement | Pass; allowed tools save independently; both final reads include four entries after adding the legacy frontmatter server | `evidence/final-readback.json` |

The retained bootstrap manifest, journey log, screenshots, redacted reads, verification report, audit and teardown receipts live under `qa-artifacts/qa/` in lab `agent-mcp-attachments-20261105-024755-263560`. The strict evidence auditor reports **PASS, 0 blockers, 0 warnings**. Teardown reports **clean: true** after stopping the exact registered lab processes and verifying their identities. The provider home was empty and provider usage collection disabled.

## Focused verification

- `CGO_ENABLED=1 go test -race -tags=integration ./internal/config ./internal/api/core ./internal/api/spec ./internal/tools/builtin ./internal/daemon ./internal/cli -run 'TestAgentMCPSidecarAdditions|TestBaseHandlersCreateAgentEndpoint|TestBaseHandlersAgentDefinitionMutations|TestDocumentTracksRequiredFieldsAndEnums|TestAgentDefinitionMutationLifecycleIntegration|TestNativeAgentCreate|TestAgentUpdateCommand|TestAgentCreateCommand|TestAgentKindToolText' -count=1` — passed. The real daemon resource synchronization journey covers name references without publishing them as standalone MCP definitions.
- After the response/digest repair: `CGO_ENABLED=1 go test -race -tags=integration ./internal/api/core ./internal/daemon -run 'TestBaseHandlersCreateAgentEndpoint|TestBaseHandlersAgentDefinitionMutations|TestAgentDefinitionMutationLifecycleIntegration|TestNativeAgentCreate' -count=1` — passed.
- `COMPOZY_GO_LINT_SCOPES='./internal/config ./internal/api/core ./internal/api/contract ./internal/api/spec ./internal/daemon ./internal/tools/builtin ./internal/cli' make go-lint` — passed, zero issues with normal and integration build tags. The final core response repair also passed the scoped core lint.
- `make codegen` and `make codegen-check` — passed; OpenAPI, TypeScript/Go clients, native catalog and CLI reference regenerated through their owners.
- Root Turborepo Web tests for `agent-mcp-servers-panel`, `agent-editor-stores` and shared `mcp-server-editor` — 23 passed. The broader agent editor canary also passed (83 tests across four files).
- `bunx turbo run typecheck --filter=compozy-web --only` and `bunx turbo run build --filter=compozy-web --only` — passed. Scoped `oxlint --deny-warnings` and formatting checks passed. The build retains existing large-chunk notices.
- `npx react-doctor@latest --verbose --scope changed --base origin/main` — no issues, score 93. `bun run proto check --json` — zero problems; pre-existing coverage/spec-catalog notices remain.

## Review and investigation

Claude Opus 5.5/high reviewed the full change and two corrective rounds. Repairs cover resource publication, package ownership, broken references, config-only reference rejection, committed credential lifetime, and full effective readback/digests on consecutive Saves. Final review approved with no blockers. Its documentation wording suggestion was applied; its optional pending-removal coverage suggestion was verified through the real Web journey.

An extra ad hoc exact-byte legacy probe initially failed because the existing AGENT.md renderer canonicalizes handwritten YAML. The renderer is unchanged from the baseline. The original assertion was retained and passed on the normalized definition; no repository test was weakened. Initial browser-driver failures came from cold loading, onboarding transitions, label selection and a response URL query; corrected drivers re-walked the affected steps successfully. The SDK generator's initial missing-template failure was repaired by restoring pinned patched dependencies with `bun install --frozen-lockfile`.

Screenshot hosting was not performed: the managed `wrangler` skill could not be loaded (`skill_catalog_resolve` backend failure). Local screenshots were inspected and retained in the isolated lab evidence. CI and hosted reviewers remain separate delivery gates.

## Cross-surface impact

The PR owns the impact audit defined by `docs/_memory/change-impact.md`: additive HTTP/UDS agent payloads and generated clients; `compozy__agent_create` input and agent read projections; CLI create/update `--mcp-server`; agent-local `mcp.json` persistence and runtime name resolution; Web Tools sheet, site documentation and the official `skills/compozy/` reference. Existing frontmatter keeps loading per SD-013. No new native tool IDs, hooks, extension SDK protocol, SQLite migration or `config.toml` key is introduced. Scoped server lookup preserves the selected workspace/profile. Successfully committed Vault credentials remain explicitly managed because duplicated agents can share them; failed creates clean up newly minted credentials.

The final native credential-boundary addition passed `CGO_ENABLED=1 go test -race ./internal/daemon -run TestNativeAgentCreate -count=1` and daemon-scoped Go lint under both tag sets. The skill's test-shape heuristic still flags the unchanged `TestNativeAgentCreateKindAlias`; it reports no finding in the added subtest.
