# File explorer S7 backend and desktop evidence

Scope: S7 of `.compozy/tasks/file-explorer/_slices.md`, branch `feat/file-explorer-s7`, based on integration `818ba3d5d`.
The owning cross-surface audit remains the spec's `_impact-entry.md`; S10 owns its consolidation, official-skill documentation and QA scenarios. No push or PR is part of this slice.

## Implementation

- `compozy session changes <session-id>` reads the existing HTTP/UDS Changes endpoints, supports branch/uncommitted scopes, explicit bases, repeated patch paths, sensitive reveal, human tables and JSON/JSONL. Domain failures retain their code and exit with status 1.
- `compozy__session_changes` is an additive `compozy__sessions` RiskRead descriptor. Its daemon binding reuses `changes.NewEngine`, registered workspace/worktree roots and BaseRef, profile read scope, shared HTTP payload conversion and existing PR projections. It never uses resumed CWD. Requested rename patches carry the old path from the summary.
- The Electron catalog owns fixed application IDs/argv. Detection reuses the login-shell PATH resolver; macOS also detects application bundles. Windows detection resolves native executables instead of batch shims. `configured` delegates to the existing editor flow; `reveal` uses `showItemInFolder`. Main and preload validate requests and only the product main frame can invoke the new bridge methods.
- Targets pass through the existing lexical/realpath/native-confirm policy. A regression exposed missing project credential checks, so the policy now also confirms `.env`, `.env.*`, PEM/key/PKCS files and the other credential file names in S1’s classifier, while excluding `.env.example`, `.env.sample` and `.env.template`. Launches use `shell:false`.
- The viewer header and outside-path state compose the existing UI Button/DropdownMenu primitives. Successful selection remembers the target; Cmd/Ctrl-click opens that target without routing the panel. Browser modifier clicks keep the viewer behavior.
- Catalog and CLI documentation are regenerated through their owners. No schema/config/hook/extensibility shape changes were needed. The official skill is unchanged here, per the S7 slice notes and S10 ownership.

## Canonical test ownership

| IDs | Existing suite and tests |
| --- | --- |
| UT-052 | `web/src/systems/session/components/__tests__/message-markdown.test.tsx`, `Should Cmd-click directly into the last-used target without opening the viewer`; existing browser routing cases |
| UT-060 | `desktop/src/product/__tests__/file-opener.test.ts`, `Should detect login PATH targets and omit absent binaries` |
| UT-061 | Same suite, parameterized `Should launch %s with fixed argv and no shell` (VS Code, Zed, IDEA) |
| UT-062 | Same suite, `Should reveal through the file manager without spawning` |
| UT-063 | Same suite, `Should reject unknown and vanished targets` |
| UT-064 | Same suite, `Should confirm and cancel %s before launching` (outside and project .env) |
| UT-065 | Same suite, `Should delegate configured target to the existing editor command` |
| UT-070 | `internal/daemon/native_tools_test.go`, `TestNativeSessionChanges/Should return real changes and gate sensitive patches` |
| UT-071 | Same suite, `Should classify unknown and user scope sessions`; `Should return root unavailable for user scope sessions` |
| IT-008, IT-009 | `internal/daemon/session_files_integration_test.go`, `TestDaemonSearchSessionFilesWorktreeBoundIT005/Should expose Changes summary patches contents and bases identically over HTTP and UDS`, extended with hosted MCP and CLI parity, human output and exit-1 base error |
| E2E-008 | `desktop/e2e/_electron/__tests__/shell.spec.ts`, `E2E-008: Open in remembers Cursor for transcript modifier clicks` |

Additional behavior coverage stays in `file-viewer-pane.test.tsx` (select app, remember primary target) and `product-bridge-controller.test.ts` (main-frame authorization of both target methods). No new test files were needed.

## Verification

Completed receipts:

```sh
go test -ldflags=-w ./internal/daemon ./internal/cli ./internal/tools/builtin ./internal/api/core -run 'TestNativeSessionChanges|TestSessionChanges|TestBuiltinNativeDescriptors|TestProjectChanges' -count=1
GOFLAGS=-ldflags=-w go test -tags integration ./internal/daemon -run '^TestDaemonSearchSessionFilesWorktreeBoundIT005$' -count=1
# Real daemon journey passed in 17.296 s, including hosted MCP/HTTP/CLI parity and CLI exit code 1.
go run ./cmd/compozy-codegen all
make cli-docs
bunx turbo run typecheck --filter=./web --filter=./desktop
# Five tasks passed.
bunx turbo run test --filter=./web -- src/systems/session/components/__tests__/message-markdown.test.tsx src/systems/files/components/__tests__/file-viewer-pane.test.tsx
# 31 tests passed.
bunx turbo run test --filter=./desktop -- src/product/__tests__/file-opener.test.ts src/product/__tests__/product-contract.test.ts src/product/__tests__/product-bridge-controller.test.ts src/product/__tests__/login-shell-path.test.ts
# 60 tests passed; focused statements 92.42%, branches 89.05%, functions 94.73%, lines 94.73%.
bunx turbo run build --filter=./web
# Passed. Existing CSS ::highlight optimizer and chunk-size/plugin-timing warnings remain in the integrated baseline.
bunx react-doctor@latest --verbose --diff
# 91/100. One warning in unchanged packages/ui/src/components/files/hooks/use-diff-viewer.ts:67.
```

Final additional receipts:

```sh
go run ./cmd/compozy-codegen check
# PASS: no generated drift.
/Users/pedronauck/.local/share/mise/installs/golangci-lint/2.15.0/golangci-lint-2.15.0-darwin-arm64/golangci-lint run --allow-parallel-runners --concurrency 2 ./internal/cli ./internal/daemon ./internal/tools/builtin ./internal/tools ./internal/api/core
# PASS: 0 issues.
bun run --cwd desktop build:e2e-update
# PASS: final-source Electron fixture build.
# From desktop/:
env -u COMPOZY_INTERNAL_RESTART_OPERATION_ID bun x playwright test --config playwright.config.ts --grep 'E2E-008: Open in'
# PASS: 1 test, 16.1 s (recording Cursor CLI, actual Electron bridge/viewer/transcript).
```

The focused `oxlint --deny-warnings` run over touched desktop/web files passed with no output. Test-shape heuristics pass for native daemon, integration and CLI helpers. The CLI session and builtin descriptor files retain identical pre-existing heuristic findings in untouched tests; the new tests follow canonical subtest conventions.

Intermediate runs exposed and repaired production issues: project `.env` confirmation, native public workspace identity, CLI domain exit status/code, and preserving the S4 OpenInButton children slot. No assertions were weakened. A three-file desktop run passed every assertion but lacked coverage of the imported login-shell resolver; adding its existing canonical suite produced the passing focused receipt above.

E2E execution initially selected an unrelated pre-existing E2E-008 zoom case as well as the new test. Both stopped before product startup because the supervisor's `COMPOZY_INTERNAL_RESTART_OPERATION_ID` leaked into the isolated fixture. The final run selected only the new test and unset that internal restart variable. The isolated fixture uses its browser's public session creation/prompt API: inherited managed CLI identity belongs to the supervisor daemon and correctly cannot operate as a nonexistent session in a different test daemon. No session identity was removed or replaced. Electron artifacts were rebuilt from final source before the successful walk.

## S10 handoff

Affected QA scenarios: `APP-open-transcript-paths-in-editor`, `APP-open-in-targets`, `ET-session-review-files`, `ET-session-review-changes`.

Update the official `skills/compozy/references/native-tools.md` session-tool inventory and runtime/session usage guidance with `compozy__session_changes`, the CLI flags, root/profile constraints, bounded patches and explicit sensitive reveal. Update desktop reference/site documentation for viewer-first navigation, Open in targets, remembered modifier-click target, native confirmations and reveal-only Finder behavior. Consolidate the owning impact audit and scenario docs after integration. No compatibility shim or migration is needed for the additive surfaces.

No repository-wide gates, full test suite, race run, full E2E, push or PR were requested or performed.

Final receipts: the real-daemon integration was rerun after shared payload extraction and passed in 17.296 s; the final filtered typecheck passed all five tasks in 56.725 s. Focused Go lint returned `0 issues.` Commit hooks use a temporary hook directory with focused Go formatting and the usual TS/JSON format/lint checks via `lint-staged --no-stash --no-revert`; commit-message validation remains enabled. This preserves the explicit prohibition on destructive Git operations and avoids repository-wide formatting.

A final native-boundary regression also requests a symlink into `.git/config` and asserts `path_denied` without any absolute-path disclosure. The daemon now preserves protected, missing, non-file and unavailable-root domain codes instead of flattening them to `changes_unavailable`. `go test -ldflags=-w ./internal/daemon -run '^TestNativeSessionChanges$' -count=1` passed in 2.635 s after this correction; the read-only test-shape checker passed again.

The final IT-008 fixture explicitly creates a project-root parent and a worktree-bound child, then dispatches the hosted tool from that child. The full real-daemon summary/patch/CLI journey passed again in 17.296 s. The final daemon-only pinned lint passed with `0 issues.` Final desktop confirmation rules match the S1 sensitive file-name catalog, retaining the existing home-credential policy; the existing table-driven opener suite additionally covers credential names and public environment samples.

UT-052 also explicitly sends both Meta-click and Ctrl-click without a desktop bridge, preserving the existing plain-click assertions and proving the viewer route still runs for each modifier.

The Electron fixture was rebuilt once more after credential-classifier parity; the final-source E2E-008 walk passed in 16.1 s. The final web modifier-parity rerun passed all 31 tests in the two owning suites. No required S7 check remains pending.
