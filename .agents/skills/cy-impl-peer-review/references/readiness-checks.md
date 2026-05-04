# Implementation Readiness Markers

When the user opts into a code peer review, the change is "ready for Opus" only when all markers below pass. These correlate with high-signal Opus output (tight blockers, real risks) versus shallow noise (Opus rediscovering broken builds, half-finished WIP, or stray artifacts).

If any marker fails, abort the round and report the failed markers. Opus review on a broken or incomplete change wastes credit and produces noise.

## Marker 1: Build & Test Gate

The most recent `make verify` (or, when scope is narrowly bounded, the relevant per-surface gate — `make lint test build`, `make bun-lint bun-typecheck bun-test`, etc.) succeeded against the current worktree. Capture the timestamp or rerun before the review.

A red gate means Opus will spend reasoning on noise the linters would have caught.

## Marker 2: Non-Empty Diff

`git diff main...HEAD --stat` (or `git diff --staged --stat` when explicitly reviewing staged-only state) is non-empty and lists real source/test/doc/config changes — not only `.tmp/`, `ai-docs/`, `.compozy/tasks/*` notes, or unrelated whitespace.

## Marker 3: No Stray Local Tracking Artifacts

`git status` shows no committed `.tmp/` or `ai-docs/` paths, and no unintended binaries (compiled outputs, screenshots accidentally added). These belong outside the review scope and pollute Opus's context.

## Marker 4: No Obvious WIP Markers

`git grep -nE 'TODO\(WIP\)|FIXME\(WIP\)|XXX\(WIP\)|<<<<<<<|console\.log\(|fmt\.Println\(' -- <changed-files>` returns empty (or only matches the agent has explicitly justified inline). Active conflict markers, leftover scaffolding, or debug prints mean the author is mid-edit.

## Marker 5: Codegen Co-Ship (when contracts touched)

If any of the following changed:
- `internal/api/contract/**`
- `internal/api/spec/**`
- `openapi/agh.json`
- `web/src/generated/**`

…then `make codegen-check` passes. A drift here is a hard blocker the skill must surface before Opus sees the diff. (Source: `agh-contract-codegen-coship` skill.)

## Marker 6: Migration Co-Ship (when schema touched)

If any `*.sql` schema file changed (column add/drop, index change, constraint change), the diff also contains a numbered migration entry in the registry — not an `EnsureSchema`-style boot reconciliation. (Source: `agh-schema-migration` skill, lessons L-001..L-013.)

## Marker 7: Surface Co-Ship Statement

For backend changes, the user (or the matching task file) has named the web/docs impact: either the diff includes the web/docs changes, or there is an explicit "no impact" rationale that the reviewer can validate. A backend-only diff with no impact analysis is a `cy-web-docs-impact` violation and Opus will flag it correctly — but the skill should call it out before spending Opus credit.

## Marker 8: Scope is Reviewable

`git diff main...HEAD --stat` reports ≤ 5000 changed lines and ≤ 80 files. Larger diffs require explicit user confirmation and a `--files` scoping pass — Opus produces shallow findings on sprawling diffs (cy-review-round prioritization rule applies here too).
