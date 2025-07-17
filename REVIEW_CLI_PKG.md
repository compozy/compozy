### Deep Analysis of the `cli` Package

After thoroughly reviewing the provided file tree, code snippets, and document contents, I've identified several systemic issues in the package. The `cli` package is a Cobra-based CLI tool for managing workflows, authentication, development servers, and proxies in the "Compozy" system. It supports JSON and TUI modes, integrates with configuration loading, and handles API interactions. However, it's indeed a "mess" due to:

#### 1. **Duplications**

- **Handler Logic**: Auth handlers are split into separate files for JSON/TUI and keys/users (e.g., `handlers_keys_json.go`, `handlers_keys_tui.go`, `handlers_users_json.go`, `handlers_users_tui.go`). Common patterns (e.g., flag parsing, error output, logging) are duplicated across these files. For instance, `outputJSONError` is defined in `handlers_common.go` but similar error handling appears in multiple places.
- **Execution Patterns**: `auth/executor.go` defines a `CommandExecutor` for auth commands, but `command_executor.go` seems like a general-purpose executor (possibly a leftover or duplicate). Workflow commands (e.g., in `workflow/list.go`) have their own ad-hoc execution logic without reusing this pattern.
- **Utilities**: `utils/` directory has files like `cliutils.go`, `http_client.go`, etc., but there's also a top-level `utils.go`. Functions like `parseFiltersFromFlags` in workflow code duplicate flag parsing logic from `flags/global.go` (e.g., `ResolveFormatFlag`, `ExtractCLIFlags`).
- **Tests**: `auth/client_test.go` and `auth/list_test.go` are separate, but `testing/example_test.go` and `testing/test_helpers.go` suggest scattered test helpers. Some tests (e.g., retry logic) are well-covered, but others (e.g., TUI interactions) are missing.
- **Configuration Handling**: Config loading appears in multiple places (e.g., `config.go`, `mcp_proxy.go`, `dev.go`), with duplicated env file loading (`loadEnvFile`) and source precedence logic.

#### 2. **Inconsistencies in Coding Standards and Patterns**

- **Command Structure**: Auth commands use `ExecuteCommand` with `ModeHandlers` for JSON/TUI switching, but workflow commands (e.g., `runList` in `workflow/list.go`) implement mode switching inline. MCP proxy and dev commands have no mode switching at all (they're JSON-only by default).
- **Flag Handling**: Global flags are defined in `flags/global.go`, but subcommands add flags inconsistently (e.g., auth uses `--api-key`, workflow uses `--status`/`--tags`). Validation is spotty: `validateSortFlags` in workflow code uses `cliutils.ValidateEnum`, but auth flags (e.g., role validation) are inline and error-prone.
- **Error Handling**: Some places use `fmt.Errorf("... %w", err)` for wrapping, others use plain `errors.New` or `outputJSONError` (which mixes stderr output with error returns). Logging is inconsistent: some use `log.Debug` with fields, others print directly to stdout.
- **Naming and Conventions**:
  - Mixed casing: `keyCtrlC` (const) vs. `CtrlKKey` (in TUI).
  - Inconsistent types: `userFilters` struct in auth handlers vs. `services.WorkflowFilters` in workflow.
  - Magic constants: Hardcoded values like `maxWorkflowLimit` (workflow), `httpRequestTimeoutSeconds` (utils), without central definition.
- **Mode Detection**: `DetectMode` (in `auth/mode.go`) is auth-specific but should be global. TUI mode assumes interactive terminals, but no handling for non-TTY (e.g., CI).
- **API Interactions**: Auth uses a custom `Client` with retry logic, but workflow uses `resty` via `createAPIClient`. No shared rate limiting (despite `ratelimiter.go` existing).
- **TUI Implementation**: Components in `tui/components/` are modular (e.g., `navigation.go` handles multiple overlays), but workflow TUI in `workflow/list.go` reinvents table rendering instead of reusing `workflow_table.go`.
- **Output Formatting**: JSON output uses `json.NewEncoder` with indent, but table output (e.g., `displayWorkflowTable`) is ad-hoc. `formatters/json.go` exists but isn't used consistently.
- **Dependencies**: Heavy use of external libs (Cobra, Bubble Tea, Resty, etc.) without version pinning or consistent imports.

#### 3. **Potential Bugs and Issues**

- **Incomplete Features**:
  - Auth: `activeOnly` filter in `list-users` is parsed but ignored (TODO for KeyCount).
  - Workflow: Empty workflows handled with info log, but JSON mode doesn't output structured response.
  - Cascade deletion in `delete-user` is TODO and returns error if enabled.
- **Security Risks**:
  - API keys and passwords are logged in debug mode without redaction (e.g., in `runCreateUserJSON`).
  - No input sanitization for user-provided IDs (e.g., `userID` in `runUpdateUserJSON` could allow injection if not escaped in API calls).
  - Env file loading doesn't handle malformed files gracefully (e.g., invalid syntax crashes).
- **Race Conditions/Concurrency**:
  - TUI updates (e.g., in `navigation.go`) use `tea.Batch` for cmds, but no locking for shared state (e.g., `NavigationManager` fields).
  - Retry logic in `auth/client.go` doesn't backoff exponentially, risking overload.
- **Error Propagation**:
  - In `runConfigShow`, errors from `loadConfigWithSources` aren't wrapped, losing context.
  - JSON errors go to stderr but return `errors.New(message)`, duplicating output.
- **Performance**:
  - Sorting/filtering (e.g., `filterAndSortUsers`) is O(n log n) per call, inefficient for large lists.
  - No pagination in auth lists (flags exist but ignored in API calls).
- **Test Coverage Gaps**:
  - No tests for TUI interactions (e.g., key presses in Bubble Tea models).
  - Edge cases like context cancellation in API calls are tested in auth but not workflow.
  - No integration tests for full CLI flows (e.g., `dev` with watching).
- **Other**:
  - In `config.go`, `flattenConfig` hardcodes fields, breaking if `config.Config` changes.
  - README.md has truncated content and outdated examples (e.g., references non-existent flags like `--log-json`).
  - Date formats (e.g., `dateTimeFormat` in workflow) assume UTC, no timezone handling.

#### 4. **Overall Architecture Issues**

- **Modularity**: Subpackages (auth, workflow, tui) are siloed, leading to reinvention (e.g., separate clients for auth vs. workflow).
- **Extensibility**: New commands (e.g., future "task" subcommand) would require duplicating executor/mode logic.
- **Documentation**: README is comprehensive but has sections like "API Reference" that list functions without details. Help files in `help/` are Markdown but not integrated into Cobra help.
- **Build/Maintainability**: No central init (despite `init.go`), leading to scattered setup. Package is large; could split into submodules (e.g., `cli/auth`, `cli/tui` as separate Go packages).

### Suggested Improvements

To fix this, aim for **consistency, modularity, and testability**. Refactor in phases: standardize patterns, eliminate duplicates, add tests, then polish.

#### Phase 1: Standardize Patterns and Eliminate Duplicates

- **Command Execution**:
  - [x] Make `CommandExecutor` (from `auth/executor.go`) global in a new `cmd/executor.go`. Remove `command_executor.go`.
  - Update all commands (auth, workflow, dev, mcp-proxy, config) to use `ExecuteCommand` with `ModeHandlers`. For non-TUI commands (e.g., dev), provide a no-op TUI handler that errors gracefully.
  - [x] Centralize mode detection in `cmd/mode.go`.
- **Flags**:
  - [x] Centralize all flag validation in `flags/global.go` (add `ValidateAllFlags(cmd)` func called in PreRun).
  - [x] Use Viper for flag binding to config, reducing manual extraction (integrate with `pkg/config`).
- **Utilities**:
  - [ ] Merge `utils.go` into `utils/`. Consolidate HTTP clients into one `utils/http_client.go` with configurable retries/rate limiting.
  - [ ] Move redaction (e.g., `redactURL`) to `utils/security.go`.
- **Output**:
  - [ ] Use `formatters/` for all outputs: `json.go` for structured JSON, add `table.go` for tables. Deprecate ad-hoc prints (e.g., in `displayWorkflowTable`).
  - [ ] Standardize errors: Always wrap with `%w`, log at appropriate levels, output to stderr only in quiet mode.
- **Handlers**:
  - [ ] Merge auth handlers into fewer files: `handlers_keys.go` (with JSON/TUI funcs), `handlers_users.go`. Use interfaces (e.g., `Handler interface { ExecuteJSON(...); ExecuteTUI(...) }`).
  - [ ] Extract common logic (e.g., flag parsing, filtering/sorting) to `utils/cliutils.go`. Reuse `parseFiltersFromFlags` across auth/workflow.

#### Phase 2: Fix Bugs and Complete Features

- **Implement TODOs**: Add KeyCount to `UserInfo`, implement active filter. Support cascade deletion with API call chaining.
- **Security**: Add redaction to logs (e.g., use a logging wrapper). Validate IDs with regex (e.g., `^[a-zA-Z0-9]+$`).
- **Concurrency**: Add exponential backoff to retries (use `github.com/cenkalti/backoff`). Use mutexes in TUI shared state.
- **Pagination**: Add API support in auth client (e.g., `ListUsers` with params).
- **TUI**: Reuse `workflow_table.go` for all lists. Add integration tests with `github.com/charmbracelet/bubbletea/testing`.
- **Configs**: Make `flattenConfig` dynamic via reflection (like `collectSourcesRecursively`). Handle timezones in dates.

#### Phase 3: Improve Testing and Documentation

- **Tests**: Aim for 80%+ coverage. Add suite for each command (use `testing.TB`). Mock API with `httptest`.
- **Docs**: Update README with consistent CLI examples (use code blocks). Integrate `help/` Markdowns into Cobra's `Long` field.
- **Tools**: Run `golint`, `go vet`, `staticcheck`. Add linter config to `.golangci.yml`.

#### Phase 4: Architectural Refinements

- **Split Packages**: Move `auth/` to `internal/cli/auth`, `tui/` to `internal/cli/tui`. Make `services/` an interface for API clients.
- **Interfaces**: Define `APIClient` interface for auth/workflow to share.
- **Config Integration**: Use `pkg/config` everywhere; add CLI flag overrides in a central loader.
- **Performance**: Cache sorts/filters if lists are large.

This refactor should reduce code by removing duplicates, fix bugs, and make the package scalable. Estimated effort: 2-4 weeks for a single dev, starting with auth as a prototype. If you provide specific files or areas, I can suggest code snippets!
