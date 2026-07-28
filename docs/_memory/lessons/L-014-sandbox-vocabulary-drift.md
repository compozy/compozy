# L-014: Runtime Vocabulary Must Match Public Contracts

**Class:** Architecture / Product vocabulary

## Incident

The execution isolation feature was implemented and exposed as "environments" even though the product concept was Sandbox. The mismatch appeared across internal packages, config keys, database columns, API fields, CLI flags, hook topics, extension Host API methods, web navigation, settings UI, generated docs, and task artifacts.

The feature was also under-documented: the landing page did not explain Sandbox, runtime docs did not have a dedicated Sandbox section, and the web UI hid the feature under Settings instead of giving it primary navigation.

## Root cause

The first implementation used an overloaded infrastructure term and let it harden into contracts before product vocabulary was settled. Once code, docs, generated references, and QA artifacts diverged, the feature became difficult to discover and easy to misrepresent.

## Fix / Rule

Public runtime concepts need one canonical noun before they reach contracts. If the noun changes during greenfield alpha, perform one hard cut across every public and internal surface in the same change:

- runtime packages, storage columns, config keys, generated contracts, CLI flags, hook names, Host API methods, logs, and tests
- web routes, navigation, page copy, mocks, route tests, and generated client types
- landing pages, docs navigation, runtime guides, generated CLI docs, and examples
- `CLAUDE.md` / `AGENTS.md`, `.compozy/tasks/*` artifacts, glossary, and lessons

Do not keep aliases, redirects, dual fields, or fallback parsing for the obsolete product noun. Generic operating-system terminology remains valid when it describes process environment variables or host context.

## Evidence

- Accepted implementation plan: `.codex/plans/2026-04-28-sandbox-hard-cut.md`
- Runtime package hard cut: `internal/sandbox/`
- Public API/schema source: `internal/api/spec/spec.go`
- Web IA hard cut: `web/src/routes/_app/sandbox.tsx` and `web/src/components/app-sidebar.tsx`
- Dedicated docs: `packages/site/content/runtime/core/sandbox/index.mdx`
- Landing page surface: `packages/site/components/landing/sandbox-section.tsx`
