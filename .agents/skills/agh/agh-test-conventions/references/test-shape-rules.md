# AGH Test Shape Rules

Verbatim canonical rules. Reviewers will quote these. Stay aligned.

## Subtest naming

- Every case is `t.Run("Should <verb-phrase>", func(t *testing.T) { ... })`.
- `Should <verb-phrase>` reads as "this case asserts that the system Should ...". E.g., `"Should reject empty workspace path"`, `"Should release lease on heartbeat failure"`.
- Table-driven layouts use a `name` field where `name = "Should ..."`.

## Parallelism

- Independent subtests: `t.Parallel()` is mandatory.
- `t.Setenv` users: `t.Parallel()` is forbidden (Go contract). Reviewers who request `t.Parallel()` on such tests are wrong.
- Tests that mutate process globals must first isolate or inject that state. `t.Setenv` is the only legitimate `t.Parallel()` opt-out; do not preserve avoidable global coupling by serializing the case.

## Error handling

- `_ = err` is forbidden in production code AND in tests.
- Capture both `data, err := json.Marshal(x)`, handle `err`, and use `data`; never discard either result.
- Cleanup with `t.Cleanup(func() { ... })` returns errors via `t.Errorf` (not `t.Fatalf` — cleanup is best-effort).

## Assertions

- HTTP/UDS handler tests assert status code AND at least one of:
  - response body content (parsed shape, specific field values),
  - error message text (when 4xx/5xx),
  - persisted state read-back (the row exists / has the expected shape).
- Idempotency tests re-send the SAME idempotency key on retry. Empty-body retries don't prove anything beyond re-entry.
- Time/ID assertions use injected determinism. `time.Now()` and `rand` belong inside helpers that accept a clock/generator.

## Interface assertions

- Every new exported type implementing an interface has `var _ Interface = (*Type)(nil)` adjacent to the type (in production code, not the test).
- Tests that depend on an interface satisfaction also call the interface explicitly to catch drift.

## Build tags

- Integration tests live in `*_integration_test.go` with `//go:build integration`.
- Co-located with the package; no `test/` subdirectory.
- `make test` = unit only. `make test-integration` = `+integration`. `make test-e2e-runtime` and `make test-e2e-web` are separate lanes.

## Integration / E2E

- `TestMain` for expensive one-time setup/teardown.
- Use real dependencies: real SQLite via `t.TempDir()`, mock ACP server as a subprocess (`acpmock`). Prefer subprocess mocks over in-process fakes.
- Keep package runtime ~30s max in CI.
- Heavy E2E (`make test-e2e-nightly`) lives in the release-PR `dry-run` job — never in a cron/schedule workflow.
- E2E tests are part of the runtime contract: when a runtime contract changes (prompt augmenter, situation context, fixture format), the E2E mock and matchers ship in the same PR.
- Replace fragile string-matching with structured metadata. ACP prompt routing in `acpmock` uses typed prompt metadata, not rendered prompt substrings.

## Mocks

- Mock via interfaces, not test-only methods on production types.
- Mock servers as helper subprocesses where applicable (`acpmock`), not in-process fakes.
- Replace fragile string matching with structured metadata when the mock fixture must canonicalize input (acpmock prompt routing).

## Coverage

- 80% coverage minimum per package. Affected package coverage reported in task memory `Completion Notes`.
- Track absolute coverage trend in `MEMORY.md` per task; a drop is a regression.

## Helpers

- `t.Helper()` on every helper function used inside a test.
- `t.TempDir()` for filesystem isolation. Never `os.TempDir()`.
- `t.Cleanup` for ordered teardown.

## Race / cgo

- `make verify` runs `-race`. Race-enabled tests need `CGO_ENABLED=1`.
- `runRaceEnabledGoCommand` (or equivalent) clones caller env and forces `CGO_ENABLED=1` for race subprocesses. Do not trust ambient env.
- Linux-Race CI parity: before claiming `make verify` complete on race-sensitive packages (`internal/session`, `internal/acp`, `internal/hooks`, `internal/subprocess`, `internal/resources`), reproduce locally with `act workflow_dispatch -W .github/workflows/ci.yml -j verify --container-architecture linux/amd64`.

## Commit gate

- `make verify` is the commit gate. If verification is blocked by an external/branch-side asset issue (missing test fixture, etc.), do NOT commit — report the verified blocker and hold.
- Test failures are production bugs. Fix production code; don't weaken assertions. The only exception is documenting an INVALID review item with concrete evidence.

## E2E follows runtime contract

- When a runtime contract changes (prompt augmenter, situation context, fixture format), the E2E mock and matchers ship in the same PR.
- Use structured metadata (typed prompt metadata) rather than rendered-prompt substring matching in `acpmock`.
