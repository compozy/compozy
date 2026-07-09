# Implementation Readiness Markers

When the user opts into a code peer review, the change is "ready for external review" only
when all markers below pass. These correlate with high-signal output (tight blockers, real
risks) versus shallow noise (the reviewer rediscovering broken builds, half-finished WIP, or
stray artifacts).

These markers are language- and stack-agnostic. If any marker fails, report the failed
markers and abort the round — external review on a broken or incomplete change wastes credit
and produces noise.

## Marker 1: Build & Test Gate

The project's verification command succeeded against the current worktree. Resolve the
command in this order:
- the user-provided `--verify <cmd>`, if given;
- otherwise auto-detect: a Makefile `verify` target, else a Makefile `test`/`check` target,
  else a `package.json` script (`verify` > `test` > `lint`), else a recognizable test runner
  for the stack (e.g. `cargo test`, `go test ./...`, `pytest`);
- if none can be resolved, ask the user which command proves the build/tests are green.

Capture the timestamp or rerun before the review. A red gate means the reviewer spends
reasoning on noise the linters/tests would have caught.

## Marker 2: Non-Empty Diff

The diff (`git diff <base>...HEAD --stat`, or `git diff --staged --stat` when reviewing
staged-only state) is non-empty and lists real source/test/doc/config changes — not only
scratch notes, generated junk, or unrelated whitespace.

## Marker 3: No Stray Local Tracking Artifacts

`git status` shows no accidentally committed build outputs, scratch/`.tmp` dirs, local
notes, or unintended binaries (compiled outputs, screenshots). These belong outside the
review scope and pollute the reviewer's context.

## Marker 4: No Obvious WIP Markers

A grep over the changed files for WIP/debug/conflict markers returns empty (or only matches
the author has explicitly justified inline):

```
git grep -nE 'TODO\(WIP\)|FIXME\(WIP\)|XXX\(WIP\)|<<<<<<<|=======|>>>>>>>|console\.log\(|fmt\.Println\(|[^.]\bprint\(|debugger;?' -- <changed-files>
```

Active conflict markers, leftover scaffolding, or debug prints mean the author is mid-edit.

## Marker 5: Generated Artifacts Co-Ship (soft check)

If the diff touches sources that drive code generation (API/IDL/schema definitions, OpenAPI
or protobuf specs, ORM schema files, codegen inputs), confirm the regenerated outputs
co-ship in the same diff. Drift here is a blocker the reviewer would flag correctly — surface
it before spending review credit. If the project exposes a codegen-check command, run it.

## Marker 6: Scope is Reviewable

`git diff <base>...HEAD --stat` reports ≤ 5000 changed lines and ≤ 80 files. Larger diffs
require explicit user confirmation and a `--files` scoping pass — external review produces
shallow findings on sprawling diffs.
