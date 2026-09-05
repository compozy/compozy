# Compozy Change Impact

Use for changed runtime behavior, public contracts, config, or feature documentation. Record this analysis once in the owning spec/task/PR and link it from implementation slices; update only the affected entries. A small change needs only concise findings. Editorial changes with no runtime contract may state `not applicable — editorial only`.

- **Native tools:** changed `compozy__*` IDs, toolsets, descriptors, schemas/digests, risk flags, capability gates, diagnostics, and CLI/API fallbacks.
- **Extensibility and hooks:** extensions, hooks, skills/capabilities, resources, registries, bridge SDKs, MCP sidecars, and config lifecycle. Config changes co-ship defaults, loader/overlay behavior, validation, docs, and compatibility under SD-013.
- **Workspace data isolation:** classify changed data as global/workspace/session/agent-scoped. Follow `workspace_id` through affected CLI/HTTP/UDS/core/store/web/cache/SSE/event paths and verify the owning boundary prevents cross-workspace leakage.
- **Official Compozy skill:** update `skills/compozy/` when public behavior, tools, CLI paths, hooks, capabilities, resources, or memory/network/task semantics change.
- **Web/Docs impact:** name affected `web/` routes/components/hooks and `packages/site` docs, plus their verification owner. Backend changes carry this analysis with the feature.

For an unaffected entry, name the checked surfaces and why the change cannot affect them. Do not create separate artifacts or re-audit unchanged surfaces for every checkpoint. Breaking changes also name delete targets and the user-state/public/internal compatibility regime; apply SD-013 before deletion.
