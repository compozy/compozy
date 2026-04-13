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

## Evidence Rules

Capture exact commands, inputs, outputs, and artifact paths. Prefer observable outcomes over interpretation.
