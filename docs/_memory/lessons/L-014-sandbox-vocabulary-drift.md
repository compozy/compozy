# L-014: Runtime Vocabulary Must Match Public Contracts

**Class:** Architecture / Product vocabulary

## Incident

The execution isolation feature was implemented and exposed as "environments" even though the product concept was Sandbox. The mismatch appeared across internal packages, config keys, database columns, API fields, CLI flags, hook topics, extension Host API methods, web navigation, settings UI, generated docs, and task artifacts.

The feature was also under-documented: the landing page did not explain Sandbox, runtime docs did not have a dedicated Sandbox section, and the web UI hid the feature under Settings instead of giving it primary navigation.

## Root cause

The first implementation used an overloaded infrastructure term and let it harden into contracts before product vocabulary was settled. Once code, docs, generated references, and QA artifacts diverged, the feature became difficult to discover and easy to misrepresent.

## Fix / Rule

Public runtime concepts need one canonical noun. Classify affected surfaces under SD-013: migrate persisted data losslessly, preserve published names through the deprecation window, and hard-cut internal consumers together. Update affected code, generated contracts, CLI/hooks, UI, docs, and glossary in the same feature change. Boundary aliases carry their removal release; internal code keeps one name. Generic operating-system terminology remains valid where it describes actual host/process context.

## Evidence

- Accepted implementation plan: `.codex/plans/2026-04-28-sandbox-hard-cut.md`
- Runtime package hard cut: `internal/sandbox/`
- Public API/schema source: `internal/api/spec/spec.go`
- Web IA hard cut: `web/src/routes/_app/sandbox.tsx` and `web/src/components/app-sidebar.tsx`
- Dedicated docs: `packages/site/content/runtime/core/sandbox/index.mdx`
- Landing page surface: `packages/site/components/landing/sandbox-section.tsx`
