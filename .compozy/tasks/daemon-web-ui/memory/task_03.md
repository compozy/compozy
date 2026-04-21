# Task Memory: task_03.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Replace the placeholder `@compozy/ui` package with a reusable daemon UI foundation derived from `docs/design/daemon-mockup/colors_and_type.css`.
- Keep scope to shared tokens, font loading, initial primitives/shell helpers, `web` consumption wiring, and executable tests proving that surface.

## Important Decisions
- Kept the shared UI surface intentionally small and route-agnostic: `cn`, `UIProvider`, `Button`, `StatusBadge`, `SurfaceCard*`, `SectionHeading`, and `AppShell*`.
- Added Tailwind v4 only where it is needed for the SPA consumer (`web`) and used shadcn-style class conventions in `packages/ui` with `class-variance-authority`, `clsx`, and `tailwind-merge`.
- Self-hosted the mockup’s `Nippo` and `Disket Mono` assets inside `packages/ui/src/assets/fonts/` so the package owns its font-loading path instead of relying on the docs directory at runtime.
- Added a package `./utils` export and a `web/components.json` alias contract so later shadcn-style work can target the shared package boundary directly.

## Learnings
- Vite correctly bundled font assets referenced from the workspace CSS export path when `web/src/styles.css` imports `@compozy/ui/tokens.css`.
- The pre-existing root `test/frontend-workspace-config.test.ts` needed an update because `task_03` intentionally expands the shared package exports beyond the original placeholder contract.
- Root `bun run test` still includes unrelated SDK/template failures outside this task’s scope, so task evidence should rely on targeted frontend tests plus the repo completion gate (`make verify`) until later workflow work addresses the broader JS test contract.

## Files / Surfaces
- `packages/ui/package.json`
- `packages/ui/tsconfig.json`
- `packages/ui/vitest.config.ts`
- `packages/ui/src/index.ts`
- `packages/ui/src/tokens.css`
- `packages/ui/src/assets/fonts/*`
- `packages/ui/src/lib/utils.ts`
- `packages/ui/src/components/{ui-provider,button,status-badge,surface-card,section-heading,app-shell}.tsx`
- `packages/ui/tests/{tokens,package-exports,primitives}.test.{ts,tsx}`
- `web/package.json`
- `web/vite.config.ts`
- `web/components.json`
- `web/src/{main,styles,app}.tsx`
- `web/src/{app,styles}.test.ts`
- `test/frontend-workspace-config.test.ts`

## Errors / Corrections
- Fixed React prop type collisions by omitting the native HTML `title` attribute in component prop interfaces that use `title` as a React node prop.
- Fixed the initial package coverage failure by adding tests for optional shell/button branches instead of relaxing the threshold.
- Fixed the lint warning in the new `cn` test by removing the constant boolean expression instead of suppressing oxlint.

## Ready for Next Run
- Shared package surface is usable from `web/` and ready for later route/domain tasks.
- Fresh validation evidence for this task:
  - `bun run typecheck`
  - `bun run --cwd packages/ui test:coverage`
  - `bun run --cwd web test`
  - `bunx vitest run test/frontend-workspace-config.test.ts packages/ui/tests/*.test.ts web/src/*.test.ts`
  - `bun run build`
  - `bun run lint`
  - `make verify`
