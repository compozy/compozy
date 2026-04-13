# QA Review 001 — Extensões

Date: 2026-04-11
Commit under test: `a1aa7813f4ab`
Tester: Codex

## Findings

### 1. Critical — `@compozy/create-extension` published artifact is not executable as a CLI

- Impact:
  The documented user flow `npx @compozy/create-extension my-ext` is broken. The package declares a `bin` entry at `dist/bin/create-extension.js`, but the build only emits `dist/src/index.js`, and that file does not contain any CLI bootstrap.
- Reproduction:
  1. Build and pack the local npm packages:
     `npm run build --workspace @compozy/extension-sdk --workspace @compozy/create-extension`
     `npm pack --workspace @compozy/extension-sdk --pack-destination /tmp/...`
     `npm pack --workspace @compozy/create-extension --pack-destination /tmp/...`
  2. Inspect the tarball contents for `@compozy/create-extension`.
  3. Observe that the tarball contains `dist/src/index.js` but not `dist/bin/create-extension.js`.
  4. Running `node sdk/create-extension/dist/src/index.js qa-prompt --template prompt-decorator --skip-install` produces no scaffolded project because the emitted file never calls `parseArgs()` / `createExtension()`.
- Expected:
  The packed npm artifact should contain a working executable entrypoint, and invoking the CLI should scaffold a project.
- Actual:
  The packed artifact has no emitted bin target and the emitted JS performs no CLI action.

### 2. High — `exec --extensions` starts extension subprocesses but never dispatches prompt/run hooks

- Impact:
  Opt-in exec extensibility is functionally incomplete. Extensions initialize and shut down, but `prompt.post_build` and `run.post_shutdown` hooks never fire, so prompt decorators and lifecycle observers do nothing in `exec`.
- Reproduction:
  1. Use the `createExtension()` API to scaffold and install a TypeScript `prompt-decorator` extension (`qa-prompt`), then run:
     `compozy exec --extensions --dry-run --persist "Decorate me"`
  2. Compare with `compozy exec --dry-run "Decorate me"`.
  3. Inspect `.compozy/runs/<run-id>/extensions.jsonl`.
  4. Repeat with a `lifecycle-observer` extension (`qa-life`) and `COMPOZY_TS_RECORD_PATH=/tmp/records.jsonl`.
- Expected:
  `prompt.post_build` should mutate the dry-run prompt/output, and `run.post_shutdown` should emit the lifecycle record.
- Actual:
  Output remains unchanged, no lifecycle record is written, and the audit log only contains `initialize` and `shutdown`.

### 3. Critical — installed extensions resolve relative subprocess paths against the workspace, not the extension directory

- Impact:
  An installed extension can execute the wrong code or fail initialization when the current workspace also contains a matching relative path such as `dist/src/index.js`. This breaks isolation and makes user-scoped extensions non-portable across workspaces.
- Reproduction:
  1. Install and enable only the `qa-prompt` user extension from workspace `qa-prompt`.
  2. Change directory into a different workspace `qa-life` that also contains `dist/src/index.js`.
  3. Run:
     `compozy exec --extensions --dry-run "Cross workspace"`
- Expected:
  `qa-prompt` should execute its own installed artifact from `~/.compozy/extensions/qa-prompt/...`.
- Actual:
  The subprocess resolves `dist/src/index.js` from the current workspace and fails during initialize with:
  `missing:[run.mutate] target:initialize`
  which matches the `qa-life` extension code, not `qa-prompt`.

### 4. High — Go scaffolding default install path produces a project that cannot resolve `sdk/extension`

- Impact:
  The Go authoring flow advertised by the scaffolder is broken in practice. `go mod tidy` fetches `github.com/compozy/compozy@v0.1.10`, but that published module does not contain `sdk/extension`, so scaffolded Go projects fail before first build.
- Reproduction:
  1. Call the `createExtension()` API with:
     `runtime: "go", template: "prompt-decorator", moduleName: "example.com/qa-go", skipInstall: false`
  2. Observe the `go mod tidy` failure.
- Expected:
  The generated Go project should resolve `github.com/compozy/compozy/sdk/extension` and build successfully.
- Actual:
  `go mod tidy` fails with:
  `module github.com/compozy/compozy@latest found (v0.1.10), but does not contain package github.com/compozy/compozy/sdk/extension`

## Scope covered

- CLI management: `ext install`, `ext enable`, `ext list`, `ext inspect`
- TypeScript authoring flow: scaffolding API, `npm install`, `npm run build`, `npm test`
- Exec runtime flow: `exec --dry-run`, `exec --extensions --dry-run --persist`
- Cross-workspace user extension execution
- Go authoring flow: scaffolding API + `go mod tidy`

## Resolution status

- Finding 1: Fixed. The packed `@compozy/create-extension` artifact now ships a real CLI entrypoint at `dist/bin/create-extension.js`, includes templates in `dist/templates`, and the packaged CLI scaffolds projects successfully.
- Finding 2: Fixed. `exec --extensions` now dispatches `run.pre_start`, `prompt.post_build`, `run.post_start`, `run.pre_shutdown`, and `run.post_shutdown`, with manual QA confirming prompt mutation and lifecycle record delivery.
- Finding 3: Fixed. Installed extension subprocesses now launch with a working directory anchored to the extension installation directory or resolved binary directory, eliminating cross-workspace path bleed.
- Finding 4: Fixed. The Go scaffolder now installs `github.com/compozy/compozy/sdk/extension` through an explicit ref fallback strategy, and the generated project resolves/builds successfully.

## Verification after fixes

- Manual QA reruns passed for packed CLI usage, TypeScript prompt decorator, lifecycle observer, cross-workspace installed extension execution, and Go scaffolding.
- Automated verification passed:
  - `npx vitest run sdk/create-extension/test/create-extension.test.ts`
  - `go test ./internal/core/extension ./internal/core/run/exec ./internal/core/subprocess -count=1`
  - `make verify`
