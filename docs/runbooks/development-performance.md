# Development and test performance

## Measured changes — 2026-09-05

Baseline: commit `610abf293`, `tests-cleanup` worktree. Machine: macOS arm64,
16 logical CPUs, 128 GiB RAM, Go 1.26.6, Bun 1.4.0. These are local measurements;
they are not measured reductions in GitHub Actions duration.

| Operation | Before | After | Reduction | Evidence |
| --- | ---: | ---: | ---: | --- |
| Repeat Web typecheck with a cached downstream task | 29.653 s | 0.527 s | 98.2% | 10 runs after 3 warmups |
| Repeat artifact preparation for development | 17.460 s | 0.517 s | 97.0% | 10 runs after 3 warmups |
| Air build with unchanged inputs | 11.731 s | 0.925 s | 92.1% | 10 runs after 3 warmups; identical executable SHA-256 |
| Resolve the ready daemon's configuration | 648.7 ms | 21.5 ms | 96.7% | 10 runs after 3 warmups; identical decoded JSON |
| Full Web unit suite | 147.32 s | 116.96 s | 20.6% | One complete run per pool; all 6,807 cases passed |
| Codegen race suite after removing two duplicates | 50.944 s | 49.058 s | 3.7% | One profiled race run per version |
| Five selected GlobalDB migration suites | 170.720 s | 155.982 s | 8.6% | One race-enabled run per version; all assertions preserved |

The first four rows measure repeated work with populated compiler/dependency caches.
Source or generated-artifact changes still trigger real work. First installs and
changed-source compilation remain separate costs. The Web and migration figures
are individual observations, not stable throughput estimates. Other worktree
activity affected some exploratory samples; part of the generation baseline also
ran with the development supervisor alive. The final first-two-row measurements
include the Go/Bun/Mage context key and overlapped other validation; their standard
deviations were 56 ms and 64 ms respectively. Raw timings include that variation.
Do not add these percentages together or extrapolate them to total CI time.

## What changed

### Reuse successful checks and development generation

The Turbo codegen prerequisite uses the existing gate evidence under `.cache/gate/`.
The fingerprint covers repository content and now includes Go toolchain/build
settings, the Bun executable/version, and Mage selection. A successful retained log
is required. Input edits, checked-artifact edits
or removal, and failures invalidate reuse. Turbo still runs the prerequisite so
stale generated artifacts cannot hide behind a cached typecheck or test result.

`make dev`, `make web-dev`, `bun run dev`, and `make desktop-dev` reuse artifact
generation through `make codegen-dev`. This is a distinct record from codegen
verification. A generation that changes the tree can require another run before
evidence becomes reusable because the gate fingerprints inputs before execution.
This conservative behavior avoids certifying source edits made during generation.

`make codegen` always generates, and `make codegen-check` always performs the full
check. These commands remain the explicit paths for refreshing artifacts and
checking drift. E2E asset preparation now publishes the same process-local success
marker already used by Verify, after the last codegen check succeeds. Descendant
Turbo/script tasks reuse that successful check within the pipeline.

### Preserve the executable cache across atomic Air publication

The Air build script copies the current binary to its next-output path before
calling `go build`. Go can then reuse the executable when its build ID matches the
current inputs. Source, embedded data, and build flags remain inputs to Go's own
invalidation. The next output is published only after a successful build; errors
leave the previous runnable binary intact. This uses Go's existing
[build-ID cache behavior](https://go.dev/src/cmd/go/internal/work/buildid.go).

The supervisor queries configuration with the just-built executable after receiving
the current run's readiness event. `COMPOZY_AIR_BUILD_DIR` is now honored consistently
by build, launch, configuration lookup, and cleanup.

### Reduce test preparation and process overhead

Web Vitest uses the threads pool with the same worker limit and test isolation.
A normalized list of every test name and result has the same SHA-256 before and
after. Per-process memory statistics for forks and threads are not comparable as
aggregate memory, so no memory reduction is claimed.

Two legacy extension-environment migration fixtures clone the repository's existing
empty migration-prefix template. They still seed their own historical rows and
execute the upgrade, reopen, constraint, and ownership assertions. No migration SQL
or persisted-user-data contract changed.

Two success-only codegen helper cases duplicated output/header assertions already
owned by `TestRunWithPaths`, which additionally checks the generated contracts.
Those duplicate cases were removed. Write-failure tests and global artifact-drift
checks remain. The rest of the handoff's proposed deletions were not applied in bulk:
its own audit found only 23% precision in the mechanical flags, and several sampled
E2E candidates protect distinct browser or failure behavior.

### Restore tooling coverage

Root Vitest collection no longer references the removed scaffold workspace and now
includes Desktop and the lint-plugin project. The 47 lint-plugin tests run as a
Turbo workspace and participate in CI selection. They exposed four failures in the
UI reuse rule: it read only the top-level barrel after the UI exports had moved to
separate inventories. The loader now follows those inventories; all existing tests
pass. Three affected local UI identifiers were renamed without changing behavior.

The local gate selects readiness tests for Air/supervisor changes and lint-plugin
tests for UI inventory changes. The tracked 15.2 MB Go test executable was removed,
and root test executables are ignored. Removing a generated executable does not
remove source coverage.

## Validation

`make gate` passed with all affected lanes: script integration, Mage tests, Go
lint/race tests, and root Turbo lint/typecheck/test. The JavaScript projects passed
8,158 tests, including all 6,807 Web cases and the restored 47 lint-plugin cases.
The final gate reused valid Go/Turbo results from the preceding complete runs.
Evidence: `final-gate-3.log`, with uncached storage/Web results in `final-gate-2.log`.

`make test-e2e-runtime` passed 262 tests across daemon, HTTP, UDS, runtime harness,
and remote-gateway CLI suites. Its asset builds reused the successful parent
codegen check. Both the runtime envelope and development lab recorded
`clean: true` teardown. Evidence: `final-e2e-runtime.log`, `e2e-teardown.log`, and
`dev-teardown.log`.

Completed focused evidence includes the full Web suite, selected real SQLite
migration suites with the race detector, lint-plugin tests using actual Oxlint
processes, and development process tests using real Bash/Go builds. The build
publication suite checks unchanged bytes, changed source, changed embedded data,
and retention of the runnable executable after a failed build.

The isolated development walk reached onboarding through Vite, survived a browser
refresh, and returned successful status/workspace/task API responses. CLI status,
direct HTTP, and the Vite proxy all reached the same isolated daemon. A deliberate
invalid Go edit left the API serving HTTP 200; removing it restored a successful
build without replacing unchanged daemon bytes. See the
[development QA report](../qa/reports/2026-09-05-dev-performance.md).

Lint reported zero issues. Existing test-environment diagnostics (jsdom APIs,
reduced-motion and failure-path logging), macOS linker deprecation messages, and
Web bundle-size/build-plugin notices remain visible; none were suppressed. No
remote CI run or full Playwright suite was performed for this local worktree.

## Change impact

- Native tools and extensibility: no `compozy__*` IDs, CLI/HTTP/UDS contracts,
  extension/hook interfaces, or persisted config keys changed. The development
  supervisor invokes the existing config CLI through its ready executable.
- Workspace data: migration SQL and schemas are unchanged. Tests clone only empty
  historical prefixes and still exercise seeded upgrades and isolation assertions.
- Official skill: `skills/compozy/` needs no update because public behavior is
  unchanged. Developer build-directory selection is documented above.
- Web/Docs: test pooling and development orchestration changed; three local
  component identifiers were renamed to satisfy the restored lint rule. Runtime
  rendering is unchanged. The owning readiness scenario and QA report record the
  actual development walk; public site documentation is unaffected.

## Reproduce and inspect

Run frontend validation from the repository root:

```sh
bunx turbo run typecheck --filter=compozy-web
bunx turbo run test --filter=compozy-web --force
bunx turbo run test --filter=@compozy/lint-plugins
make gate
make gate-status
```

For repeated-work measurements:

```sh
hyperfine --warmup 3 --runs 10 'make codegen-dev'
hyperfine --warmup 3 --runs 10 'bash scripts/build-air.sh'
```

Use an owned `COMPOZY_AIR_BUILD_DIR` for build benchmarks and the isolated runtime
manifest for daemon queries. Do not run benchmark suites alongside other CPU-heavy
verification if precise comparisons are needed.

Raw local evidence is retained under `.compozy/tasks/tests-cleanup/performance/`:
`turbo-typecheck-before/final.json`, `codegen-generation-before/final.json`,
`air-build-before/after.json`, `air-build-checksums.json`,
`dev-config-benchmark.json`, `web-before.json`, `web-threads.json`,
`web-behavior-checksums.json`, and the migration JSONL logs. CPU profiles, the
opportunity matrix, rejected experiments, and command logs are retained there too.

## Remaining costs and rejected changes

- Changing Go's race-test processor cap from 16 to 4 improved the sampled migration
  by only 1.3% over three runs. The global processor policy was kept.
- Migration-directory loading represented less than 0.1% of sampled CPU time.
  A production cache would add invalidation complexity without a demonstrated win.
- Mage dispatch and gate fingerprinting were already subsecond. A rewrite was not
  justified by their measured cost.
- The codegen race profile still spends substantial time rendering and comparing a
  14.5 MB OpenAPI document. No global render cache or public schema restructuring
  was introduced without evidence that mutable contract checks remain correct.
- E2E runtime/data isolation, failure recovery, browser-only behavior, race checks,
  and the existing CI shard layout remain part of the quality contract. The
  handoff's historical 86 E2E job-minutes is not a current before/after CI result.
