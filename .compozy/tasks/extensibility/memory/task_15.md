# Task Memory: task_15.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot

- Deliver the TypeScript authoring surface under workspace packages, plus the starter templates, author docs, explicit TS validation, and the real lifecycle-observer smoke test against the Go extension manager.

## Important Decisions

- Use two `sdk/*` workspace packages so the repo layout matches the existing JS monorepo conventions:
  - `sdk/extension-sdk-ts` publishes `@compozy/extension-sdk`
  - `sdk/create-extension` publishes `@compozy/create-extension`
- Keep the starter template source of truth inside the SDK package so the scaffolder, docs, and repo tests all consume the same files.
- Add a public TS testing surface (`@compozy/extension-sdk/testing`) with an in-memory mock transport and host harness, mirroring the ergonomics of the Go SDK enough for template and SDK tests.
- Add repo-level `Makefile` targets for the npm packages because the TS SDK and scaffolder ship from inside the monorepo.

## Learnings

- The public Go SDK from task 14 can be mirrored closely in TypeScript without inventing a second protocol layer: the initialize direction, hook capability mapping, and Host API method inventory all translate directly.
- The repo already has root TypeScript/Vitest tooling, so package-local builds can use `tsc` and the repo-level tests can stay on the existing toolchain.
- The real lifecycle-observer subprocess smoke test is easiest to validate through `internal/core/extension.Manager` with a temporary wrapper script that executes the built Node entrypoint.
- The runtime and protocol use `review.post_fetch.patch.issues`, so the public SDK surface must expose `IssuesPatch` rather than reusing `EntriesPatch`.

## Files / Surfaces

- `sdk/extension-sdk-ts/{package.json,tsconfig.json}`
- `sdk/extension-sdk-ts/src/{index,types,transport,host_api,handlers,extension}.ts`
- `sdk/extension-sdk-ts/src/testing/{index,mock_transport,test_harness}.ts`
- `sdk/extension-sdk-ts/templates/{lifecycle-observer,prompt-decorator,review-provider,skill-pack}/**`
- `sdk/extension-sdk-ts/test/{transport,handlers,fluent_hooks,extension,host_api,templates}.test.ts`
- `sdk/create-extension/{package.json,tsconfig.json,README.md,src/index.ts,bin/create-extension.ts,test/create-extension.test.ts}`
- `.compozy/docs/extensibility/{index,getting-started,architecture,hook-reference,host-api-reference,capability-reference,trust-and-enablement,testing,hello-world-go,hello-world-ts,migration-guide}.md`
- `internal/core/extension/ts_template_manager_integration_test.go`
- `sdk/extension/{hooks.go,handlers.go,extension_test.go,smoke_test.go}`
- `Makefile`

## Errors / Corrections

- The first SDK typecheck failed because empty hook patch objects were narrower than the initial `JsonValue` definition; the JSON object type was widened and the hook patch write path now casts through the public JSON value type.
- The first Go smoke-test version assumed the package test ran from the repo root; the helper now resolves the repo root from `internal/core/extension` before copying the TS template and SDK package.
- The TypeScript lifecycle template records `kind` rather than the Go mock fixture's `type`; the smoke test now reads the correct record shape instead of mutating template behavior just for tests.
- `make verify` exposed a stale test assumption about observer-hook completion order. The integration assertion now checks presence and partial order instead of assuming `run.pre_shutdown` always records before `run.post_shutdown`.
- `make verify` also exposed a `goconst` lint hit for repeated `"start"` literals; the extension package now uses named command constants.

## Ready for Next Run

- Task 15 implementation is complete.
- Verification evidence captured:
  - `npm run build --workspace @compozy/extension-sdk --workspace @compozy/create-extension`
  - `npx vitest run sdk/extension-sdk-ts/test/*.ts sdk/create-extension/test/create-extension.test.ts`
  - `npx vitest run sdk/extension-sdk-ts/test/transport.test.ts sdk/extension-sdk-ts/test/handlers.test.ts sdk/extension-sdk-ts/test/fluent_hooks.test.ts sdk/extension-sdk-ts/test/extension.test.ts sdk/extension-sdk-ts/test/host_api.test.ts --coverage` -> 83.3% statements / 83.23% lines for the TS SDK package
  - `go test ./sdk/extension -count=1`
  - `go test ./internal/core/extension -count=1`
  - `make verify`
