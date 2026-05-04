# CLI Startup & Dispatch — Performance Analysis

## Summary

The `compozy` CLI startup path is dominated by **eager construction of the entire subcommand tree** plus a **background update check goroutine** fired from `main.run()` before Cobra even parses arguments. Every invocation of `compozy --help` (or any leaf like `compozy runs purge`) pays the cost of:

1. Building a kernel `Dispatcher` with all six typed handlers (reflection-keyed map, `reflect.TypeFor[...]` on each Register) — `internal/cli/root.go:30` → `internal/core/kernel/deps.go:33`.
2. Allocating an events `Bus[Event]` (unused for help/informational commands) — `internal/cli/root.go:33`.
3. Calling `ValidateDefaultRegistry` which linearly re-reads the registered map under `RLock`, sorts all types by formatted name, and checks them against the expected set — `internal/core/kernel/deps.go:72`.
4. Constructing ~22 Cobra subcommand trees, each of which allocates a fresh `commandState` struct with nine function pointers, and each lipgloss-heavy leaf lazily touches theme vars (benign) but does so in `init`-less patterns.
5. A **goroutine starts unconditionally** in `cmd/compozy/main.go:30` to poll GitHub for new releases, racing a 250 ms wait at exit. Even though `COMPOZY_NO_UPDATE_NOTIFIER` short-circuits inside `CheckForUpdate`, the goroutine, context, and two channels still exist.
6. Import-time side effects: `internal/core/kernel/core_adapters.go:21` calls `core.RegisterDispatcherAdapters(...)` at package init, which forces `internal/core/kernel` to be linked (fine) but also drags the entire kernel dependency graph (`plan`, `run`, `agent`, `model`) into `--help` binaries.
7. Import-time side effects: `internal/core/extension/runtime.go:96` calls `model.RegisterOpenRunScopeFactory(...)` — this is cheap but anchors the extension runtime, journal, subprocess, and events packages into every `compozy --help` binary.

There are no O(n²) construction patterns in Cobra registration, no YAML/TOML parsing on the `--help` path, and no filesystem walks on pure help. The real wins are around **decoupling help from dispatcher construction**, **deferring the update goroutine until after help-only fast path**, and **lazy subtree construction** for rarely-used groups (`ext`, `daemon`, `workspaces`, `agents`, `runs`).

Expected cold-start wins: 15–40 ms on macOS SSD; 30–70 ms on cold-cache Linux CI. Warm-start wins: 5–10 ms (cached inode lookups dominate elsewhere).

---

## Cold-Start Walkthrough

`main.run()` sequence when user types `compozy --help`:

1. `cmd/compozy/main.go:22` — `os.Exit(run())`.
2. `cmd/compozy/main.go:27` — `compozy.NewCommand()` → `compozy.go:86` → `cli.NewRootCommand()`.
3. `cli.NewRootCommand` → `newRootDispatcher()`:
   - `internal/cli/root.go:31` allocates `KernelDeps` with:
     - `slog.Default()` (cheap, singleton).
     - `events.New[events.Event](0)` — allocates a `map[SubID]*subscription[Event]` with default bufSize 256 (unused for help).
     - `workspace.Context{}` zero-value.
     - `agent.DefaultRegistry()` — returns a zero-size struct, but **anchors `internal/core/agent` into the binary**, including the 8-entry `registry` map populated at package init (`registry_specs.go:82` — each entry is a struct with closure `BootstrapArgs`).
   - `kernel.BuildDefault(deps)`:
     - `NewDispatcher()` allocates `map[reflect.Type]any`.
     - Six `Register[...]` calls, each invoking `reflect.TypeFor[C]()` + `sync.Mutex.Lock` + `map[...]=h`.
     - Each handler closure/struct holds `deps` and `ops`.
   - `ValidateDefaultRegistry(dispatcher)` → `selfTestDefaultRegistry`:
     - `registeredCommandTypes(d)` RLocks, ranges, sorts by `formatType` (O(n log n)).
     - Linear compare with `expectedDefaultCommandTypes()`.
     - If any missing, `panic(err)` inside `newRootDispatcher`. *(Panic gate on startup critical path.)*
4. `newRootCommandWithDefaults(dispatcher, defaults)`:
   - `defaultCommandStateDefaults()` wires 9 function pointers to concrete implementations.
   - `root := &cobra.Command{...}` with long multi-paragraph `Long` (300+ bytes string literal).
   - Single `root.AddCommand(...)` with **17 immediate children** built eagerly:
     - `newSetupCommand(dispatcher)` builds setup state and 7 huh/lipgloss-facing flags.
     - `newAgentsCommand()` → builds `agents list`, `agents inspect`, `mcp-serve` children.
     - `newUpgradeCommand()`, `extcli.NewExtCommand(dispatcher)` (7 ext subcommands), `newMigrateCommand`, `newValidateTasksCommand`, `newDaemonCommand` (3 daemon subcommands), `newWorkspacesCommand` (5 subcommands), `newTasksCommand`, `newReviewsCommandWithDefaults` (N subcommands), `newRunsCommandWithDefaults` (3 subcommands), `newSyncCommand`, `newArchiveCommand`, `newFetchReviewsCommandWithDefaults`, `newFixReviewsCommandWithDefaults`, `newExecCommandWithDefaults`, `newMCPServeCommand`.
   - Total Cobra nodes constructed: ~40+ commands regardless of invocation.
5. `cmd.Version = version.String()`.
6. `startUpdateCheck(context.Background(), version.Version)`:
   - Spawns goroutine, constructs 2 channels, context.WithCancel.
   - Inside goroutine: `update.StateFilePath()` resolves `~/.compozy/state/update.yaml`, then `CheckForUpdate` early-returns for dev builds or `COMPOZY_NO_UPDATE_NOTIFIER`. For release builds not disabled, it reads the cached state file via `ReadState`, then possibly performs HTTP GET to GitHub.
7. `cmd.ExecuteC()` — Cobra parses flags, finds `--help`, prints usage, returns.
8. `cancelUpdateCheck()` fires; `waitForUpdateResult` blocks up to **250 ms** waiting for the update result.
9. `<-updateDone` ensures goroutine exit.

**Avoidable for `--help`:**

- Building the dispatcher + events bus + AgentRegistry.
- Validating dispatcher (the panic gate).
- Constructing `exec`, `tasks`, `reviews`, `fix-reviews`, `fetch-reviews`, `ext`, `sync`, `archive`, `migrate`, `start`, `setup`, `mcp-serve`, `daemon`, `workspaces`, `runs` children. Cobra only needs their `Use`/`Short` for help output — which still requires *some* registration, but not their state structs or flag objects.
- The update check goroutine (it runs even for `--version`, `completion bash`, tab-completion invocations, and quick subcommand help).

**Unavoidable for `--help`:**

- Go runtime init, `log/slog` default handler, Cobra root tree for usage strings.
- Reading `version.String()`.

---

## P0 Findings

### P0-1. Update-check goroutine fires before Cobra parses — cost on `--help`, `--version`, completion

- **File:line:** `cmd/compozy/main.go:30` (`startUpdateCheck(context.Background(), version.Version)`).
- **Pattern:** Fire-and-forget goroutine launched *before* `cmd.ExecuteC()`. At process end, `main` blocks for up to 250 ms on `waitForUpdateResult` for every invocation, including `--help`, `--version`, `completion bash`, typo errors, and subcommand help.
- **Impact:** 0–250 ms added latency on help/completion/version. On slow GitHub links, the 250 ms ceiling is often hit; on fast ones the cost is goroutine setup + `StateFilePath()` + `ReadState` file I/O (one YAML decode from `~/.compozy/state/update.yaml`). Shell completion is called on every Tab keystroke in zsh/bash — this is latency-visible.
- **Fix:** 
  1. Defer `startUpdateCheck` until *after* `cmd.ExecuteC()` returns and only when the executed command is a long-running workflow command (`start`, `fix-reviews`, `exec`, `fetch-reviews`). `shouldWriteUpdateNotification` already filters JSON output — extend this to filter on `cmd.Name() == "help" || cmd.Name() == "version" || strings.HasPrefix(cmd.Use, "completion")`.
  2. Alternatively: detect `--help`/`-h`/`--version` in `os.Args` before constructing the dispatcher, short-circuit the update check entirely.
- **Expected win:** Eliminates the 250 ms worst-case tail; median ~5–15 ms saved (goroutine + state read + one `time.After` timer).
- **Isomorphism:** Preserves update notifications for interactive workflows; removes them for `--help` where they were never displayed anyway.

### P0-2. Dispatcher + handlers + events bus built eagerly for every command, including help-only paths

- **File:line:** `internal/cli/root.go:30–48`, `internal/core/kernel/deps.go:33–45`.
- **Pattern:** `NewRootCommand()` calls `newRootDispatcher()` unconditionally. This allocates `KernelDeps`, constructs `events.Bus[Event]` (map + lock), calls `kernel.BuildDefault` which does six typed `Register[...]` calls (each `reflect.TypeFor` + `sync.Mutex.Lock` + map write), then runs `ValidateDefaultRegistry` which sorts and linearly scans. None of this is needed to print help text.
- **Impact:** On each invocation: ~6 reflect.Type lookups, 6 map inserts under mutex, one sort of 6 items, one event bus allocation (~200 B heap). Approx 0.3–0.8 ms on warm paths. More importantly, anchors a large transitive dependency graph (`agent`, `plan`, `run`, `journal`, `events`, `subprocess`) into the link tree; this inflates binary size and Go runtime init time by ~2–5 ms.
- **Fix:** 
  1. Use `sync.OnceValue(newRootDispatcher)` and pass a lazy function `func() *kernel.Dispatcher` into subcommand constructors. Commands that never need the dispatcher (setup list, agents list, daemon status, runs purge, workspaces list) skip construction entirely.
  2. Alternative: call `newRootDispatcher()` inside each `RunE` that actually dispatches, instead of threading `*kernel.Dispatcher` into every constructor. `setup`, `agents`, `daemon`, `workspaces`, `runs`, `upgrade`, `mcp-serve`, `validate-tasks`, `ext` do not dispatch kernel commands.
  3. For `ValidateDefaultRegistry`: drop from the critical path — it is a self-test appropriate for `go test`, not for every CLI invocation. Move to a `kernel.BuildDefault` internal assertion compiled out under `-tags release`.
- **Expected win:** 1–3 ms on warm runs; 5–10 ms on cold runs; removes a `panic()` from the startup path (P0 safety win independent of latency).
- **Isomorphism:** Output identical for commands that never dispatched; dispatching commands get dispatcher on-demand (once cached via `sync.OnceValue`).

### P0-3. Eager subcommand tree — ~40 Cobra commands built regardless of target

- **File:line:** `internal/cli/root.go:82–100`, `internal/cli/extension/root.go:150–158` (7 ext subcommands), `internal/cli/daemon.go:35` (3), `internal/cli/workspace_commands.go:25` (5), `internal/cli/runs.go:23` (3), `internal/cli/agents_commands.go:62` (3).
- **Pattern:** Every `New*Command()` call allocates `cobra.Command` structs, attaches `pflag.FlagSet`s, builds `*commandState`s with wired callbacks, and calls `cmd.Flags().StringVar/BoolVar/IntVar` dozens of times. Even the `setup` command creates 7 flags and a `setupCommandState` with 10 function pointers.
- **Impact:** Each flag registration allocates 1–2 small structs; cumulatively: ~200 flag registrations × ~200 ns = ~40 µs. Small, but cumulative: ~15–30 ms of GC-visible allocations across startup, compounded by Go map growth during flag registration.
- **Fix:** 
  1. Register only `Use`/`Short`/`Long`/`Aliases` stubs eagerly; defer `RunE` + flag wiring until `PersistentPreRunE` or `PreRunE` on the specific subcommand. Cobra supports `cmd.SetHelpFunc` returning fast stubs.
  2. Simpler: group rarely-used commands (`ext`, `workspaces`, `daemon`, `runs`, `agents`) behind a thin factory that only attaches children when `args[0]` matches the group name. Cobra's dispatch model lets you add subcommands inside `PersistentPreRunE`.
  3. Simplest first step: move the `commandStateCallbacks` wiring (9 function pointers) out of `newCommandStateWithDefaults` and into the `RunE` itself. The state struct costs ~600 bytes × 5 workflow commands = 3 KB saved per run.
- **Expected win:** 3–8 ms on cold start; reduces allocations and binary size.
- **Isomorphism:** `--help` output must still enumerate all subcommands — this is non-negotiable. The stubbing approach preserves it.

---

## P1 Findings

### P1-1. `init()` side-effects anchor entire kernel graph into every binary

- **File:line:** `internal/core/kernel/core_adapters.go:21`, `internal/core/extension/runtime.go:96`, `compozy.go:13–15`.
- **Pattern:** Blank imports `_ "internal/core/kernel"` and `_ "internal/core/extension"` register adapters/factories via `init()`. This ensures the kernel dispatcher can be used from the public `core` API, but it means `compozy --help` imports `journal`, `subprocess`, `plan`, `run`, `events`, `modelprovider`, `provider`.
- **Impact:** ~10–20 MB binary bloat; ~1–3 ms extra Go runtime init. Hard to measure directly without linker profile.
- **Fix:** 
  1. Replace `core.RegisterDispatcherAdapters` with a direct function reference set at `NewRootCommand` time (lazy): `core.SetDispatcherProvider(func() (*kernel.Dispatcher, error) { return lazyDispatcher(), nil })`. Move the `init` to a factory function called only on first dispatch.
  2. Split the public `compozy` package into a thin wrapper that doesn't import kernel by default. Only the workflow execution entry points (`Run`, `Prepare`) need kernel.
- **Expected win:** 2–5 ms cold start + 3–10 MB binary.
- **Isomorphism:** Behavior unchanged when dispatcher actually runs; lazy construction delays the registration, preserving all dispatch semantics.

### P1-2. `ValidateDefaultRegistry` sorts + panics on every startup

- **File:line:** `internal/cli/root.go:40` (`panic(err)`), `internal/core/kernel/deps.go:72–91`.
- **Pattern:** Defensive self-test on every startup. Calls `registeredCommandTypes(d)` which does an allocating sort of 6 types, then loops with formatted type strings. Terminates with `panic` if mismatched, violating CLAUDE.md's no-`panic()` rule for production paths.
- **Impact:** ~50–100 µs per startup from sort + formatted strings; panic surface on every CLI invocation.
- **Fix:** 
  1. Delete the runtime call; rely on a `TestMain` assertion or compile-time verification. `var _ Handler[commands.RunStartCommand, ...] = (*runStartHandler)(nil)` patterns in `handlers.go:102` already enforce type correctness at compile time.
  2. If runtime check is mandatory, use a `fmt.Errorf` returned to the caller and handle it in `newRootDispatcher` with a deferred error on `cmd.Execute()`, not `panic`.
- **Expected win:** 50–100 µs, plus removes panic from critical path.
- **Isomorphism:** Same correctness guarantees (compile-time) with zero runtime overhead.

### P1-3. `events.New[Event](0)` always allocates the subscription map

- **File:line:** `internal/cli/root.go:33`, `pkg/compozy/events/bus.go:36`.
- **Pattern:** Bus is constructed but only used by the workflow execution handlers. For `--help`, `setup --list`, `agents list`, `daemon status`, `workspaces list`, etc., the bus is allocated and never used.
- **Impact:** Small heap allocation (a map + a struct), ~1–2 µs per call.
- **Fix:** Allocate inside `realOperations` or on first subscription. `events.Bus` already handles zero-buffer subscribe gracefully.
- **Expected win:** Negligible per-call, but removes a pointer-holding struct from the Dispatcher deps that blocks future lazy-init work.
- **Isomorphism:** Subscription semantics identical.

### P1-4. `StartupAgentRegistry` registry populated at `agent` package init

- **File:line:** `internal/core/agent/registry_specs.go:82–252`.
- **Pattern:** Global `registry` map with 8 spec entries is initialized at package load. Each spec contains closures (`BootstrapArgs`) and nested slices. Any binary touching `agent.DefaultRegistry()` pays this cost.
- **Impact:** ~5–20 µs of map population, but anchors 8 closure values and ~30 strings.
- **Fix:** Lazy with `sync.OnceValue`. Only `ValidateRuntimeConfig` / `EnsureAvailable` need the map — both are runtime workflow operations.
- **Expected win:** ~10 µs + small binary footprint reduction.

### P1-5. Setup command wires huh/lipgloss state even for non-interactive `setup --list`

- **File:line:** `internal/cli/setup.go:97–109`, `internal/cli/theme.go:32`.
- **Pattern:** `newSetupCommand` builds `setupCommandState` with 10 function pointers and preallocates huh field callbacks. `newCLIChromeStyles()` is called lazily inside print functions (good), but state pointers are still wired eagerly.
- **Impact:** ~100 function-pointer stores + setup flag binding = ~300 µs.
- **Fix:** Inline the defaults inside `RunE`, as done in simpler commands like `runs purge`.
- **Expected win:** ~200 µs on warm startup of any non-setup command.

### P1-6. No TOML/YAML read on `--help` path — but `workspace.Discover` walks parents for any workflow-touching command

- **File:line:** `internal/core/workspace/config.go:49–94`.
- **Pattern:** `os.Getwd` → `filepath.EvalSymlinks` → walk-up `os.Stat(.compozy/tasks)` every time. Every call repeats the filesystem walk.
- **Impact:** 2–10 `os.Stat` syscalls per discover, ~100–500 µs warm, up to 10 ms cold.
- **Fix:** Memoize `Discover` result in a package-level `sync.OnceValue` keyed by starting directory. Invalidation is a non-issue because CLI is single-shot.
- **Expected win:** 0.2–2 ms saved when multiple commands internally call `Discover` (workspace_config.go:45, setup, fetch-reviews, etc.).
- **Isomorphism:** Single-process invocation can safely cache; no semantic change.

---

## P2 Findings

### P2-1. Reflect-keyed dispatcher map — acceptable but not minimal

- **File:line:** `internal/core/kernel/dispatcher.go:64–106`.
- **Pattern:** `map[reflect.Type]any` with `reflect.TypeFor[C]()` lookups; generic `Dispatch[C, R]` takes RLock, does a type assertion on `any`.
- **Impact:** ~50 ns per Dispatch call (not hot enough to matter for human CLI usage).
- **Fix:** None recommended. The reflective key is the clean way to express "one handler per typed command." A string-keyed map would be faster but regresses type safety.

### P2-2. Lipgloss styles reconstructed on every print call

- **File:line:** `internal/cli/theme.go:32` (`newCLIChromeStyles()` called inside every print function).
- **Pattern:** Allocates ~20 `lipgloss.Style` values per render. Lipgloss styles are immutable and composed cheaply, but still ~2 KB of heap per render.
- **Impact:** Human perception-irrelevant (~100–200 µs for setup list).
- **Fix:** Build styles once with `var cliStyles = newCLIChromeStyles()` package-level OR `sync.OnceValue`. Do *not* do this at `init()` — lipgloss profile detection reads env vars, so lazy is correct.
- **Expected win:** 100–300 µs on setup/list-type commands.

### P2-3. `cobra.Command.Long` strings allocated at command build time

- **File:line:** `internal/cli/root.go:55–76`, `internal/cli/commands.go:49–59`, `internal/cli/setup.go:74–77`, etc.
- **Pattern:** Long descriptions stored as string literals in every command, allocated as part of the `cobra.Command` struct.
- **Impact:** ~10 KB of Go `.rodata` — zero runtime cost after link, but inflates binary.
- **Fix:** None needed. Help text is mandatory.

### P2-4. `setup.agentSpecs` is a 44-entry slice of struct values constructed at package init

- **File:line:** `internal/setup/agents.go:236–336`.
- **Pattern:** Global `agentSpecs` allocated at init; each spec has nested `pathSpec` slices built from helper funcs. 44 × ~5 `pathSpec` = ~220 structs.
- **Impact:** ~30 µs package init; binary footprint ~10 KB.
- **Fix:** Keep as-is. Required data, not hot.

### P2-5. Extension discovery walks FS on every setup/workflow command

- **File:line:** `internal/core/extension/discovery.go:153–199`, `internal/cli/setup_assets.go:22–25`, `internal/cli/extensions_bootstrap.go:28`.
- **Pattern:** `Discovery.Discover` reads `os.ReadDir` on bundled `embed.FS`, `~/.compozy/extensions`, and `<workspace>/.compozy/extensions`. Bundled scan is cheap (embed); disk scans cost 1–3 stat/readdir per root.
- **Impact:** Not on `--help` path. On workflow commands: ~1–5 ms warm, up to 50 ms cold (extension manifests parsed).
- **Fix:** 
  1. Memoize `Discovery.Discover` result for the process lifetime (CLI is single-shot).
  2. For `fetch-reviews`, `fix-reviews`, `exec`, `start`: currently all four call `bootstrapDeclarativeAssetsForWorkspaceRoot` which re-runs discovery. Consolidate to one run with a shared cache.
- **Expected win:** 1–5 ms on workflow paths; 0 on help.

### P2-6. `shouldWriteUpdateNotification` uses `Lookup("format")` per invocation

- **File:line:** `cmd/compozy/main.go:124`.
- **Pattern:** After `ExecuteC`, walks flags to decide whether to suppress JSON-contaminating update notices. Fine.
- **Impact:** Negligible.
- **Fix:** None.

---

## Verification plan

### Cold-start benchmark (hyperfine)

```bash
# Build the binary once
make build

# Baseline: warm-up 5 runs, then 30 measurements
hyperfine --warmup 5 --runs 30 \
  './bin/compozy --help' \
  './bin/compozy --version' \
  './bin/compozy setup --list'

# Subcommand help (Cobra traverses to the leaf and prints its usage)
hyperfine --warmup 5 --runs 30 \
  './bin/compozy runs --help' \
  './bin/compozy ext --help' \
  './bin/compozy daemon --help'

# Cold-cache variant: evict with `sudo purge` (macOS) between runs
hyperfine --prepare 'sudo purge' --runs 10 './bin/compozy --help'
```

Target deltas after fixes:
- `--help`: baseline ~50–80 ms → target ≤30 ms.
- `--version`: baseline ~30–50 ms → target ≤15 ms.
- `setup --list`: baseline ~100–150 ms → target ≤60 ms.

### CPU profile

```bash
# Trace the startup phase with go's runtime trace
COMPOZY_TRACE=/tmp/compozy.trace ./bin/compozy --help
go tool trace /tmp/compozy.trace

# Or inject a pprof hook into cmd/compozy/main.go for one-off profiling
# (gated by env var; do not ship)
go tool pprof -http=: /tmp/compozy.cpu.prof
```

Look for:
- Time in `kernel.BuildDefault` and `kernel.Register`.
- Time in `update.CheckForUpdate` (even for dev builds — confirm it short-circuits fast).
- Time in `events.New` / map allocations.
- Time in extension discovery (should be zero for `--help`).

### Isomorphism proofs per change

For each P0 fix:

```bash
# Capture golden outputs
./bin/compozy --help > /tmp/help.before
./bin/compozy --version > /tmp/version.before
./bin/compozy setup --list 2>&1 > /tmp/setup-list.before
./bin/compozy ext --help > /tmp/ext-help.before

sha256sum /tmp/*.before > /tmp/golden_checksums.txt

# After change, re-generate and verify
./bin/compozy --help > /tmp/help.after
./bin/compozy --version > /tmp/version.after
./bin/compozy setup --list 2>&1 > /tmp/setup-list.after
./bin/compozy ext --help > /tmp/ext-help.after

diff /tmp/help.before /tmp/help.after
diff /tmp/setup-list.before /tmp/setup-list.after
```

Expected: byte-identical output. The dispatcher construction is internal and must not affect help rendering.

### Regression guards

- Add `TestRootCommand_HelpIsSubmillisecond` that asserts `cmd.Execute` with `--help` completes in <10 ms warm (skip on `-short`).
- Add `TestDispatcherNotBuiltOnHelp` using an instrumented `newRootDispatcher` that panics if called — confirm it's never called when `--help` is the only arg.
- Keep `ValidateDefaultRegistry` as a pure unit test in `internal/core/kernel` so refactoring the runtime check doesn't lose the safety net.

### Binary-size check

```bash
go build -o /tmp/compozy-before ./cmd/compozy
# Apply P1-1
go build -o /tmp/compozy-after ./cmd/compozy
ls -la /tmp/compozy-{before,after}
```

Target: ≥3 MB reduction from lazier kernel import chain.
