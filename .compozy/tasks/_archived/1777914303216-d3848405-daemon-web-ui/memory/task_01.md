# Task Memory: task_01.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Expand the root Bun/Turbo workspace for `web/` and `packages/ui/`, add the minimal React/Vite/UI bootstrap, and prove install/build/type resolution without implementing product routes or daemon HTTP serving.

## Important Decisions
- Kept the initial web app intentionally minimal: React + Vite + `@compozy/ui` provider/tokens seam, no TanStack route tree or API client yet.
- Used a tracked `web/dist/.keep` placeholder plus `web/scripts/restore-dist-placeholder.mjs` so `vite build` can run while the repository still checks in an embed-safe `web/dist` directory.
- Gave `packages/ui` a real emitted `dist/` build config (`tsconfig.build.json`) to avoid Turbo build warnings while keeping package exports pointed at source files for workspace consumption.

## Learnings
- Root-level Vitest execution does not automatically resolve the new web runtime dependencies from the repo root, so the runtime bootstrap test belongs in `web/` while root config checks stay file-based.
- The repo's existing root `bun run test` currently fails on unrelated SDK/template assumptions (`@compozy/extension-sdk` resolution and missing `sdk/create-extension/dist/bin/create-extension.js`), so task verification should rely on the explicit task checks plus `make verify`.

## Files / Surfaces
- `package.json`
- `turbo.json`
- `.gitignore`
- `bun.lock`
- `packages/ui/`
- `web/`
- `test/frontend-workspace-config.test.ts`

## Errors / Corrections
- `web/tsconfig.json` initially used `baseUrl`, which TypeScript 6 rejected; removed it and kept only the path alias mapping.
- `packages/ui` initially typechecked-only for `build`; switched to an emitted `dist/` build after Turbo warned that the package had no build outputs.
- `packages/ui` build initially inherited `allowImportingTsExtensions`; overrode it to `false` in `tsconfig.build.json` because emitted builds cannot keep that flag.

## Ready for Next Run
- Fresh evidence captured:
  - `bun install --frozen-lockfile`
  - `bunx vitest run test/frontend-workspace-config.test.ts`
  - `bun run test` in `web/`
  - `bun run typecheck`
  - `bun run build`
  - `make verify`
