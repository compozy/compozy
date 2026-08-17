# Resource-Only Extension Development Loop Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let native resource-only extensions use `extension build`, `dev`, `reload`, and `dev --watch` without running build or describe subprocesses.

**Architecture:** Classify a source with no Go or TypeScript toolchain by loading its native manifest. A valid manifest with at least one static resource tree and no executable extension contract follows a dedicated path that validates and publishes the manifest plus declared resources into the existing immutable-generation pipeline. Code-backed sources keep the current build and describe path unchanged.

**Tech Stack:** Go 1.26, Cobra CLI, TOML extension manifests, existing extension generation and runtime integration tests, Fumadocs MDX.

## Global Constraints

- Confirmed reproduction on 2026-08-16: `go run . extension validate /home/franciscpd/Projects/batuta-compozy -o json` returns `status: valid`; `go run . extension build /home/franciscpd/Projects/batuta-compozy -o json` fails with `extension: unsupported source; expected package.json or go.mod`.
- A resource-only build must execute neither the build command nor `__describe`.
- A source with executable extension behavior but no supported toolchain must fail closed.
- Go and TypeScript authoring behavior must not change.
- Artifacts and documentation are written in English; conversation remains Brazilian Portuguese.
- The final gate is one fresh `make gate-full` after source freeze.

## Compozy Impact Audit

- Native tools: `compozy__extensions_build`, `_dev`, and `_reload` gain resource-only source support through their existing `BuildBundle` dependency; tool IDs, descriptors, schemas, risk flags, and capability gates do not change.
- Extensibility and hooks: native resource-only manifests can enter the existing generation/dev-overlay lifecycle; code-backed extension, hook, MCP sidecar, and dynamic `resources.publish` contracts remain unchanged and executable extension contracts still require a toolchain.
- Workspace data isolation: no stored shape or scope changes; existing dev links remain keyed by daemon-resolved `workspace_id`, and the QA scenario must prove the overlay is visible only in its workspace.
- Official Compozy skill: update `skills/compozy/references/extensions.md` and `skills/compozy/references/extension-authoring.md` so agents select the resource-only authoring path correctly.
- Web/Docs Impact: no `web/` code impact because the change is source packaging before the existing daemon API; update the extension development and manifest documentation under `packages/site`.
- Config lifecycle: no config key, default, validation, or settings surface changes; `extensions.dev.watch_interval` retains its current meaning.

---

### Task 1: Resource-only build classification and publication

**Files:**
- Create: `internal/extension/build_resource_only.go`
- Modify: `internal/extension/build.go`
- Modify: `internal/extension/build_toolchain.go`
- Test: `internal/extension/build_test.go`

**Interfaces:**
- Consumes: `LoadManifest`, `requiresSubprocess`, `staticResourcePathGroups`, `prepareBuildOutput`, and `publishBuildGeneration`.
- Produces: `buildResourceOnlyBundle(ctx context.Context, req BuildRequest) (*BuildResult, error)` and a matchable internal no-toolchain condition from `detectBuildToolchain`.

- [ ] **Step 1: Add the failing build regression cases**

Add cases to `TestBuildBundle` that create a native `extension.toml` with `skills = ["skills"]`, call `buildBundle` with `newBuildTestRunner`, and assert a generation contains the manifest and skill while `runner.Commands()` is empty. Rebuild after changing `SKILL.md` and assert the generation hash changes. Add negative companions for an empty resource manifest and a manifest with `[subprocess] command`, asserting no generation is published.

- [ ] **Step 2: Run the focused test and verify RED**

Run:

```bash
CGO_ENABLED=1 go test -race ./internal/extension -run 'TestBuildBundle/Should (publish a resource-only generation without commands|reject resource-only sources that require executable behavior)' -count=1
```

Expected: the positive case fails with `unsupported source; expected package.json or go.mod`.

- [ ] **Step 3: Add a typed no-toolchain branch**

Change the no-`package.json`/no-`go.mod` result in `detectBuildToolchain` to a package-private sentinel and branch on it with `errors.Is` in `buildBundle`. All other toolchain errors continue to return unchanged.

- [ ] **Step 4: Implement the dedicated resource-only build**

Implement the focused helper with this order:

```go
manifest, err := LoadManifest(req.SourceDir)
if err != nil {
    return nil, fmt.Errorf("extension: load resource-only manifest: %w", err)
}
if err := validateResourceOnlyBuildRequest(req, manifest); err != nil {
    return nil, err
}
if err := os.MkdirAll(req.OutputDir, 0o755); err != nil {
    return nil, fmt.Errorf("extension: create build output %q: %w", req.OutputDir, err)
}
if err := prepareBuildOutput(req.OutputDir); err != nil {
    return nil, err
}
return publishBuildGeneration(ctx, req.OutputDir, req.SourceDir, manifest)
```

The validator accepts only native Compozy manifests with at least one skill, Loop, agent, automation, or layout path; it rejects `BuildCmd`, `requiresSubprocess(manifest)`, and portable Agent Plugin synthesis with actionable errors.

- [ ] **Step 5: Run focused and package tests GREEN**

Run:

```bash
python3 .agents/skills/eng/eng-test-conventions/scripts/check-test-conventions.py internal/extension/build_test.go
CGO_ENABLED=1 go test -race ./internal/extension -run '^TestBuildBundle$' -count=1
CGO_ENABLED=1 go test -race ./internal/extension -count=1
```

Expected: all new cases pass; record any unrelated pre-existing timeout separately without weakening assertions.

### Task 2: Public documentation and official skill

**Files:**
- Modify: `packages/site/content/docs/extensions/develop.mdx`
- Modify: `packages/site/content/docs/extensions/manifest.mdx`
- Modify: `skills/compozy/references/extensions.md`
- Modify: `skills/compozy/references/extension-authoring.md`

**Interfaces:**
- Consumes: the Task 1 source-classification contract.
- Produces: one public development workflow and matching agent guidance.

- [ ] **Step 1: Document source classification**

State that code-backed sources compile and run describe mode, while native resource-only sources validate and publish the handwritten manifest plus declared resource trees without executing extension code.

- [ ] **Step 2: Document the resource-only loop**

Add a minimal `extension.toml` + `agents/hello/AGENT.md` example and the commands:

```bash
compozy extension build .
compozy extension dev .
compozy extension reload hello .
compozy extension dev . --watch
```

Document that executable contracts (`subprocess`, capabilities, permissions, or dynamic publication) still need `package.json` or `go.mod`.

- [ ] **Step 3: Align the official Compozy skill**

Update both extension references with the same branch selection and failure boundary; keep them concise and avoid duplicating the site guide.

- [ ] **Step 4: Verify docs and skill links**

Run the site/docs lane selected by `make gate` after the Go diff is complete; do not add prose-string tests.

### Task 3: Real authoring lifecycle and QA evidence

**Files:**
- Modify: `internal/daemon/daemon_extension_authoring_e2e_integration_test.go`
- Create or reset: one content-addressed scenario under `docs/qa/scenarios/`

**Interfaces:**
- Consumes: the real CLI, daemon, immutable generation, workspace dev-link, resource registry, reload, and last-good behavior.
- Produces: behavior evidence for build, dev, resource exposure, changed generation, invalid-edit retention, and watch-compatible rebuilds.

- [ ] **Step 1: Extend the canonical authoring E2E suite**

Add a resource-only subtest that writes a native manifest plus an agent/skill in the harness workspace, runs real `extension build` and `extension dev`, verifies the workspace overlay and resources, edits a resource and reloads to a new hash, then makes an invalid edit and verifies the previous generation remains active.

- [ ] **Step 2: Run the focused integration test**

Run:

```bash
CGO_ENABLED=1 go test -race -tags=integration ./internal/daemon -run '^TestDaemonE2EExtensionAuthoringShouldCompleteTheDevelopmentLoopWithoutTrustPrompts$/Should_complete_the_resource-only_development_loop$' -count=1
```

Expected: PASS with no orphan daemon or extension process.

- [ ] **Step 3: Flag and execute the user-visible QA scenario**

Create or reset the content-addressed extension-authoring scenario, bootstrap a fresh isolated QA lab, run the build/dev/reload/watch workflow with a native resource-only fixture, record evidence, and always execute the manifest teardown command. The final evidence must include `teardown.json` with `"clean": true`.

- [ ] **Step 4: Run scoped and final gates**

Run:

```bash
make gate
make gate-full
make gate-status
```

Expected: the scoped gate and exactly one fresh full gate pass with zero warnings and errors.

