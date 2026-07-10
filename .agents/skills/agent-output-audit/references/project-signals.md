# Project Signal Guide

Use this guide when repository instructions do not already define the canonical QA contract.

## Priority Order

1. Root instructions such as `AGENTS.md`, `CLAUDE.md`, or repository-specific agent docs
2. Dedicated umbrella commands in `Makefile`, `Justfile`, task runners, or CI wrapper scripts
3. CI workflows under `.github/workflows/`
4. Ecosystem-native manifests such as `package.json`, `go.mod`, `pyproject.toml`, or `Cargo.toml`
5. Language-default commands as a last resort

## Common Signals

### Makefile or Justfile

Treat `verify`, `check`, `ci`, `test`, `lint`, `build`, `start`, `run`, and `dev` as high-confidence targets.

### package.json

Prefer explicit scripts in this order:

1. `verify`, `check`, `ci`
2. `test`, `test:ci`, `test:e2e`, `test:integration`
3. `lint`, `typecheck`
4. `build`
5. `start`, `dev`, `serve`, `preview`

### E2E support

High-confidence E2E signals include:

1. Runnable commands such as `e2e`, `test:e2e`, `playwright`, `cypress`, or `acceptance`
2. Framework configs such as `playwright.config.*`, `cypress.config.*`, or `wdio.conf.*`
3. Existing spec locations such as `e2e/`, `tests/e2e/`, `test/e2e/`, or `cypress/e2e/`
4. CI workflows that clearly run the same E2E command

Treat the repository as E2E-capable only when a runnable command exists and at least one other signal confirms the harness.

### Go modules

If no umbrella command exists, treat `go test ./...`, `go build ./...`, and repository formatting/lint commands as the minimum baseline. Prefer repository wrappers over direct Go commands when both exist.

### Python projects

Look for `pytest`, `tox`, `nox`, `ruff`, `mypy`, `python -m build`, and any scripts declared in `pyproject.toml`.

### Rust projects

Treat `cargo test`, `cargo build`, `cargo fmt --check`, and `cargo clippy --all-targets --all-features -- -D warnings` as strong defaults when the repository does not define wrappers.

### Mixed Repositories

When multiple ecosystems exist, identify the product entrypoint first. Do not assume every manifest is part of the same runtime surface.

## Scenario Selection Rules

Always cover:

1. A baseline verification gate
2. The workflows directly touched by the change
3. At least one adjacent regression-critical workflow
4. Startup or readiness if the change can affect bootstrapping
5. A realistic fixture path if the feature consumes external projects, repos, files, or APIs
6. An automation classification for each changed or regression-critical public flow when E2E support exists

## E2E policy

When the repository already supports E2E, require new or updated automated coverage for:

1. Changed P0 or P1 flows
2. Release-critical smoke paths
3. Bug fixes that restore public regressions

Keep flows manual-only only when automation is the wrong tool, and mark flows blocked when the harness exists but credentials, data, or runtime prerequisites are missing.

## Evidence Rules

Capture exact commands, inputs, outputs, and artifact paths. Prefer observable outcomes over interpretation.
