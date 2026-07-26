---
name: hermes-qa-patterns
description: hermes QA framework reference — pytest fixtures, live-vs-mocked split, async-bridge tests, OAuth/credential testing, subprocess/process-group tests, install-wizard probes, plugin/skill tests. Adopted as inspiration for AGH final-qa.
type: reference
source: .resources/hermes/
---

# hermes QA Framework — Reference Patterns

## 1. Operating Model (philosophy)

hermes' testing philosophy is "**hermetic parity with CI, behaviour over snapshots**." Every test runs with **all credential-shaped env vars unset**, **HERMES_HOME redirected to a per-test tempdir**, **TZ=UTC / LANG=C.UTF-8 / PYTHONHASHSEED=0**, and **module-level singletons cleared between tests** so a 16-core developer machine cannot diverge from a 4-core CI runner. The contract is enforced both by `tests/conftest.py` (autouse fixtures) and by a wrapper script `scripts/run_tests.sh` (belt-and-suspenders).

> "Hermetic-test invariants enforced here ... 1. **No credential env vars.** ... 2. **Isolated HERMES_HOME.** ... 3. **Deterministic runtime.** ... 4. **No HERMES_SESSION_* inheritance**"
>
> source: `.resources/hermes/tests/conftest.py:1-20`

> "**ALWAYS use `scripts/run_tests.sh`** — do not call `pytest` directly. The script enforces hermetic environment parity with CI (unset credential vars, TZ=UTC, LANG=C.UTF-8, 4 xdist workers matching GHA ubuntu-latest). Direct `pytest` on a 16+ core developer machine with API keys set diverges from CI in ways that have caused multiple 'works locally, fails in CI' incidents (and the reverse)."
>
> source: `.resources/hermes/AGENTS.md:674-678`

The matching anti-pattern doctrine — **no change-detector tests** — is also written into AGENTS.md:

> "A test is a **change-detector** if it fails whenever data that is **expected to change** gets updated — model catalogs, config version numbers, enumeration counts, hardcoded lists of provider models. These tests add no behavioral coverage; they just guarantee that routine source updates break CI ... The rule: if the test reads like a snapshot of current data, delete it. If it reads like a contract about how two pieces of data must relate, keep it."
>
> source: `.resources/hermes/AGENTS.md:717-758`

### Test markers and pytest config

`pyproject.toml` declares **only one marker**:

> ```toml
> [tool.pytest.ini_options]
> testpaths = ["tests"]
> markers = [
>     "integration: marks tests requiring external services (API keys, Modal, etc.)",
> ]
> addopts = "-m 'not integration' -n auto"
> ```
>
> source: `.resources/hermes/pyproject.toml:146-151`

So **by default** the suite runs everything **except** `integration`-marked tests, with auto xdist parallelism (overridden to `-n 4` by the wrapper). Live/stress/e2e are **not pytest markers** — they are **directories the runner explicitly ignores**:

> ```bash
> exec "$PYTHON" -m pytest \
>   -o "addopts=" \
>   -n "$WORKERS" \
>   --ignore=tests/integration \
>   --ignore=tests/e2e \
>   -m "not integration" \
>   "${ARGS[@]}"
> ```
>
> source: `.resources/hermes/scripts/run_tests.sh:98-104`

`tests/stress/` is gated behind a custom CLI flag `--run-stress` (declared in `tests/stress/conftest.py:24-30`). `tests/e2e/` and `tests/integration/` carry `pytestmark = pytest.mark.integration` per file and are intentionally excluded from the default run. The result is **four test lanes** with different invocation contracts:

| Lane | Path | Default? | How to run |
|---|---|---|---|
| Unit (default) | `tests/*.py`, `tests/<subpkg>/` | yes | `scripts/run_tests.sh` |
| Integration (real services) | `tests/integration/` | no | `pytest -m integration tests/integration/` (manual) |
| E2E (gateway adapters with mocked transport) | `tests/e2e/` | no | manual; not in run_tests.sh |
| Stress / property fuzzing | `tests/stress/` | no | `pytest --run-stress tests/stress/` or `python tests/stress/<file>.py` |

There is no `live` marker — credential-bearing live tests are simply integration-marked, gated by `pytest.importorskip` and explicit env-var probes inside fixtures.

## 2. Test Anatomy

### conftest.py — autouse hermetic shield

The root `tests/conftest.py` installs **five autouse fixtures** that run before every test in every file (file: `.resources/hermes/tests/conftest.py:244-548`):

1. **`_hermetic_environment(tmp_path, monkeypatch)`** (autouse) — does the heavy lifting:
   - Iterates `os.environ` and `monkeypatch.delenv`s anything matching `_API_KEY`, `_TOKEN`, `_SECRET`, `_PASSWORD`, `_CREDENTIALS`, `_ACCESS_KEY`, `_SECRET_ACCESS_KEY`, `_PRIVATE_KEY`, `_OAUTH_TOKEN`, `_WEBHOOK_SECRET`, `_ENCRYPT_KEY`, `_APP_SECRET`, `_CLIENT_SECRET`, `_CORP_SECRET`, `_AES_KEY` (`conftest.py:46-62`).
   - Plus a frozen explicit list of ~70 named credentials (`AWS_ACCESS_KEY_ID`, `OPENAI_API_KEY`, `MATRIX_ACCESS_TOKEN`, `MODAL_TOKEN_ID`, `WECOM_CALLBACK_ENCODING_AES_KEY`, ...) — `conftest.py:65-151`.
   - Plus a frozen list of ~50 `HERMES_*` behavioural vars (`HERMES_YOLO_MODE`, `HERMES_INTERACTIVE`, `HERMES_GATEWAY_SESSION`, ...) and platform allowlists (`TELEGRAM_ALLOWED_USERS`, `SLACK_REQUIRE_MENTION`, ...) — `conftest.py:163-241`.
   - Creates `tmp_path / "hermes_test"` with the canonical layout (`sessions/`, `cron/`, `memories/`, `skills/`) and points `HERMES_HOME` at it (`conftest.py:271-277`). **Does NOT redirect `HOME`** because it broke subprocesses in CI (`conftest.py:265-270`).
   - Pins `TZ=UTC`, `LANG=C.UTF-8`, `LC_ALL=C.UTF-8`, `PYTHONHASHSEED=0` (`conftest.py:281-284`).
   - Disables AWS IMDS lookups (`AWS_EC2_METADATA_DISABLED=true`, `AWS_METADATA_SERVICE_TIMEOUT=1`) so any module that probes EC2 doesn't burn 2s per test (`conftest.py:286-293`).
   - Resets the plugin-manager singleton (`hermes_cli.plugins._plugin_manager = None`) so the test never inherits another test's plugin set (`conftest.py:298-302`).

2. **`_isolate_hermes_home(_hermetic_environment)`** — backwards-compat alias preserved as a no-op so legacy tests that yield it explicitly don't break (`conftest.py:311-314`).

3. **`_reset_module_state()`** — per-test, clears 8 distinct module-global state buckets. The list is exhaustive and authoritative:

> "The skill `test-suite-cascade-diagnosis` documents the concrete patterns this closes; the running example was `test_command_guards` failing 12/15 CI runs because ``tools.approval._session_approved`` carried approvals from one test's session into another's."
>
> source: `.resources/hermes/tests/conftest.py:329-332`

   The reset sweeps: `logging` levels for the named loggers, `tools.approval._session_approved/_session_yolo/_permanent_approved/_pending/_gateway_queues/_gateway_notify_cbs` plus `_approval_session_key` ContextVar; `tools.interrupt._interrupted_threads`; the 9 `gateway.session_context` ContextVars; `tools.env_passthrough._allowed_env_vars_var`; `tools.terminal_tool._active_environments/_last_activity/_creation_locks` (with `.cleanup()` called on each evicted env); `tools.credential_files._registered_files_var`; `tools.file_tools._read_tracker/_file_ops_cache` (`conftest.py:334-441`).

4. **`_ensure_current_event_loop(request)`** — gives every sync test a working `asyncio.get_event_loop()` (Python 3.11+ removed the auto-loop guarantee). Skips if the test has the `asyncio` marker so pytest-asyncio is not disturbed (`conftest.py:478-508`).

5. **`_enforce_test_timeout()`** — installs a SIGALRM 30-second kill switch on every Unix test. **Windows is skipped explicitly** (`conftest.py:511-522`).

6. **`_reset_tool_registry_caches()`** — last-line defence against the registry's 30-second `check_fn()` cache bleeding into the next test (`conftest.py:525-548`).

The conftest also exports two helper fixtures: **`tmp_dir`** (alias for `tmp_path`) and **`mock_config`** — a minimal hermes config dict for unit tests (`conftest.py:446-468`).

### conftest.py — sub-suite specialisation

Per-area conftests **add** to the global one:

- `tests/e2e/conftest.py:1-100+` — installs `MagicMock`-shaped substitutes for `telegram`, `discord`, `slack_bolt`, `slack_sdk` modules **before** their adapters are imported, so the gateway adapter classes load even when the optional libraries aren't installed. Defines `E2E_MESSAGE_SETTLE_DELAY = 0.3` so async tests have a deterministic settle window.
- `tests/stress/conftest.py:1-40` — adds the `--run-stress` CLI option, default-skips everything in `tests/stress/`, sets `collect_ignore_glob` so stress files (which have top-level main code) are not collected as pytest modules.

### Naming + structure

- File names: `tests/test_<subject>.py` for the flat top-level set, `tests/<package>/test_<unit>.py` for per-module suites (e.g. `tests/agent/test_credential_pool.py`).
- Inside a file, behaviour is grouped into **plain `class TestX:` containers** (no inheritance from `unittest.TestCase` in the modern path; older files like `test_yuanbao_markdown.py:24-50` still use `unittest.TestCase`).
- Test functions: `test_<verb_phrase>_<expected>` — typical examples: `test_request_user_code_state_mismatch_raises` (`test_minimax_oauth.py:140`), `test_two_profiles_get_different_homes` (`test_subprocess_home_isolation.py:56`), `test_atomic_replace_preserves_symlink` (`test_atomic_replace_symlinks.py:40`).
- Async tests use **`@pytest.mark.asyncio`** declaratively (`test_yuanbao_pipeline.py:117`); pytest-asyncio is the dev-dependency (`pyproject.toml:45`).
- Parametric loops use **`@pytest.mark.parametrize`** at class level with a class constant list — see `TestMiddlewareClasses.MIDDLEWARE_CLASSES` and the three `@pytest.mark.parametrize("cls,expected_name", MIDDLEWARE_CLASSES)` declarations (`test_yuanbao_pipeline.py:918-948`). Parametric markers at file level, e.g. `tests/test_install_sh_setup_wizard_tty_probe.py:47, 73`, are also common.

### Canonical "good" test (annotated)

This is `test_atomic_replace_preserves_symlink` from `.resources/hermes/tests/test_atomic_replace_symlinks.py:40-53`:

```python
def test_atomic_replace_preserves_symlink(tmp_path: Path) -> None:
    real = tmp_path / "real.yaml"
    link = tmp_path / "link.yaml"
    real.write_text("original\n", encoding="utf-8")
    link.symlink_to(real)

    tmp = _write_tmp(tmp_path, "updated\n")
    returned = atomic_replace(tmp, link)

    assert link.is_symlink(), "symlink must not be replaced with a regular file"
    assert real.read_text(encoding="utf-8") == "updated\n"
    assert Path(returned) == real
    # Follow the symlink — same content.
    assert link.read_text(encoding="utf-8") == "updated\n"
```

What each block accomplishes:

- **`tmp_path: Path`** — pytest's per-test isolated tempdir; pairs with the autouse `_hermetic_environment` fixture that already pinned `HERMES_HOME` somewhere else, so this test can use a separate scratchpad without collision.
- **`real / link / link.symlink_to(real)`** — sets up the exact filesystem topology the regression covers (a symlinked target). The test pins **the topology**, not the spelling — so a refactor that changes `atomic_replace` internals can still ship as long as symlinks stay alive.
- **Single act (`atomic_replace(tmp, link)`)** — one operation under test; everything else is arrange/assert.
- **Multiple assertions per "what to check" axis**: symlink survives, content updated, returned path equals real path, follow-the-link content also updated. The docstring at file head (`test_atomic_replace_symlinks.py:1-12`) documents WHY the regression exists and WHAT the helper guarantees, with a GitHub issue back-reference (`#16743`). This combination — issue-anchored docstring, topology-precise arrange, behaviour-precise assert — is hermes' house style.

Other reference shapes worth copying verbatim:

- **Issue-tracked regression test** with full root cause in module docstring: `test_model_tools_async_bridge.py:1-12` (issue #2104, "Event loop is closed").
- **Source-text regression test** that asserts a file contains a pattern: `test_install_sh_setup_wizard_tty_probe.py:47-91` — uses `re` against the install script body, no Python execution; perfect for shell-script invariants.
- **Property-style invariant**: `test_get_tool_definitions_cache_isolation.py:55-72` — prove "caller mutation does not pollute cache" by mutating and re-fetching.

## 3. Live-LLM Tests

There are **no `pytest.mark.live` tests** in hermes. The repo treats real-LLM calls as a special form of `pytest.mark.integration` and **most of those mock the LLM endpoint**. The actual live coverage comes through:

- **OAuth + credential flows** (`test_minimax_oauth.py`, OAuth provider in `plans/gemini-oauth-provider.md`) — exercised end-to-end against an in-process `MagicMock` httpx client, not the real provider.
- **Local fake servers** for protocol-heavy paths (Home Assistant: `tests/fakes/fake_ha_server.py` + `tests/integration/test_ha_integration.py`) — real TCP, real WebSockets, real auth handshake; the LLM is not in the loop.
- **Pipeline-level adapters** (`tests/test_yuanbao_pipeline.py`, `tests/test_yuanbao_integration.py`, `tests/test_yuanbao_proto.py`, `tests/test_yuanbao_markdown.py`) — exercise WebSocket frame decoding, protobuf round-trips, dedup, access-guard, markdown chunking. **No real Yuanbao tokens** at any point — `make_config()` injects `app_id="test_key"`, `app_secret="test_secret"`, `ws_url="wss://test.example.com/ws"` (`test_yuanbao_pipeline.py:53-62`).
- **Provider-specific fixed-input regression tests** (`test_minimax_model_validation.py`, `test_ollama_num_ctx.py`, `test_minimax_oauth.py`, `test_anthropic_keychain.py`) — assert classifier behaviour, prompt-shape behaviour, retry semantics. The provider HTTP calls are always mocked with `unittest.mock.MagicMock` or a hand-rolled stub class.

What guards an integration-marked test from running without credentials:

- **`pytest.importorskip("library", reason="...")`** at module top — used in `tests/integration/test_voice_channel_flow.py:17-18` (PyNaCl, discord.py).
- **`pytestmark = pytest.mark.integration`** at module top — every file in `tests/integration/` carries this. Combined with `addopts = "-m 'not integration'"` in `pyproject.toml`, the file is excluded from default runs.
- **In-test runtime probes**: e.g. `tests/integration/test_modal_terminal.py:69-84` reads `MODAL_TOKEN_ID` and `~/.modal.toml`, prints a skip-explanation message, returns `False` if neither is set. Modal-specific files use the test-script-with-`pytest.mark.integration` style — they are runnable both as `python tests/integration/test_modal_terminal.py` and as `pytest -m integration`.
- **`monkeypatch.setattr` to swap a real client for a fake** — `test_account_usage.py:53-67` patches `agent.account_usage.resolve_codex_runtime_credentials` and `agent.account_usage.httpx.Client` so the test exercises real parsing logic without real network.

The closest hermes ships to "real LLM under test" is **`tests/run_interrupt_test.py`**, which is documented as **NOT a pytest test**:

> ```python
> #!/usr/bin/env python3
> """Run a real interrupt test with actual AIAgent + delegate child.
>
> Not a pytest test — runs directly as a script for live testing.
> """
> ```
>
> source: `.resources/hermes/tests/run_interrupt_test.py:1-5`

It builds a stub `AIAgent` instance via `__new__`, mocks `OpenAI` with a `slow_create` that sleeps 3s, runs `_run_single_child` in a worker thread, then triggers `parent.interrupt(...)` and asserts the child observed the interrupt in `< 2.0s`. This is hermes' **only "live integration with the real production code path"** test — and it's runnable as a script, not as part of CI.

### Evidence capture style

When live or quasi-live tests fail, hermes' style is **failure messages that explain WHY in production terms**, not just "expected X got Y":

```python
assert all_open, "At least one worker thread's loop was closed"
# ...
assert len(thread_ids) == 3, f"Expected 3 threads, got {len(thread_ids)}"
# ...
assert len(loop_ids) == 3, (
    f"Expected 3 distinct loops for 3 parallel workers, "
    f"got {len(loop_ids)} — workers may be contending on a shared loop"
)
```

source: `.resources/hermes/tests/test_model_tools_async_bridge.py:153-160`

The trifecta — **issue anchor in module docstring**, **WHY in the assert message**, **deterministic `tmp_path` for any artefact** — is the recipe.

## 4. Async Bridge / Concurrency Tests

`tests/test_model_tools_async_bridge.py` (`447 lines`) is the canonical hermes pattern for async/cancel rigor.

### What it proves

`_run_async()` in `model_tools.py` keeps a **persistent, never-closed event loop** so cached `httpx`/`AsyncOpenAI` clients survive across calls. The tests pin three distinct invariants:

1. **Loop survives** the call (`test_loop_not_closed_after_run_async`, `:50-59`).
2. **Same loop reused** across consecutive calls (`test_same_loop_reused_across_calls`, `:61-71`).
3. **Cached transport survives** (asyncio futures created in call 1 still resolve in call 2; `:73-84`).
4. **Per-thread loops in worker pools**: `ThreadPoolExecutor` with `max_workers=3`, `threading.Barrier(3)` to force three live threads simultaneously, then assert `len(loop_ids) == 3` and `len(thread_ids) == 3` (`:129-160`).
5. **Worker loop is NOT the main loop** — prevents cross-thread contention (`:162-180`).
6. **Calling from inside an already-running loop falls back to a worker thread** (`@pytest.mark.asyncio` test bouncing through `run_in_executor`, `:186-198`).
7. **Timeout uses non-blocking executor shutdown** (`shutdown(wait=False)`) so a stuck coroutine doesn't hang the caller — `:200-276`. The test installs a `FakeExecutor` via monkeypatch, records every shutdown call, and asserts `wait is False` and the submitted function is not bare `asyncio.run`.
8. **Cancellation actually reaches the worker loop**: `test_timeout_cancels_coroutine_in_worker_loop` (`:278-337`) — patches `concurrent.futures.Future.result` to time-out after 1s, then asserts `_slow_cancellable()` saw a `CancelledError` event (`cancel_observed.wait()`).

### Idioms in use

- **`@pytest.mark.asyncio`** for tests that need an event loop natively.
- **`AsyncMock`** alongside `MagicMock` (`test_yuanbao_pipeline.py:24, 354, 405`) — `next_fn = AsyncMock()` then `next_fn.assert_awaited_once()` pattern is used pervasively in pipeline middleware tests.
- **`threading.Barrier`** to deterministically force concurrency (`test_model_tools_async_bridge.py:136`).
- **`threading.Event`** for cross-thread "did this happen?" signalling (`cancel_observed = threading.Event()` at `:308`).
- **`time.monotonic()` for deadline assertions** so `time.sleep` mocks can advance "fake time" without affecting wall clock — `test_minimax_oauth.py:267-301`.

### Trajectory compressor (`test_trajectory_compressor_async.py:1-202`)

Same pattern, different bug class: the `AsyncOpenAI` client used to be created at `__init__` time and bound to whatever event loop existed then; `asyncio.run()` later closed that loop. The fix made client creation lazy via `_get_async_client()`. The tests pin:

- `async_client` is `None` after `__init__` (`:23-35`).
- `_get_async_client()` calls `openai.AsyncOpenAI(api_key=..., base_url=...)` exactly once (`:37-55`).
- **Two consecutive calls construct two separate instances** (loop-bind regression guard) (`:57-84`).
- **Source-text invariant** (`TestSourceLineVerification`): re-reads `trajectory_compressor.py` from disk and pattern-matches that no eager `self.async_client = AsyncOpenAI(` line exists outside `_get_async_client` (`:87-116`). This is hermes' style for catching regressions that aren't caught by behaviour tests when fakes mask the bug.
- Provider-specific knobs: `test_generate_summary_async_kimi_omits_temperature` and the moonshot CN/EN variants (`:119-202`) — assert `temperature` is **not** in the kwargs passed to `chat.completions.create` for Kimi-family models.

`test_run_interrupt_test.py` (script, not pytest) is the same family of test: prove cancellation propagates from `parent.interrupt()` to a child agent's running tool call within `< 2.0s`.

## 5. Subprocess / Sandbox / Home Isolation Tests

### `test_subprocess_home_isolation.py`

Pins issue #4426: subprocesses must get a per-profile HOME without affecting the Python process's HOME. Five test classes (`:21-198`):

- **`TestGetSubprocessHome`** — `get_subprocess_home()` returns `None` when HERMES_HOME unset, `None` when home/ subdir missing, the explicit profile path otherwise. **Cross-profile distinctness**: `test_two_profiles_get_different_homes` (`:56-73`) creates `alpha/` and `beta/` profiles in one tmp_path, switches `HERMES_HOME` between them, asserts the resolved subprocess homes both end with the right suffix.
- **`TestMakeRunEnvHomeInjection`** — verifies `tools.environments.local._make_run_env({})` injects `HOME=<profile>/home` when profile-home exists, leaves the parent `HOME` untouched otherwise.
- **`TestSanitizeSubprocessEnvHomeInjection`** — same property for `_sanitize_subprocess_env` (background-process path, distinct from the run-env path).
- **`TestProfileBootstrap`** — `_PROFILE_DIRS` includes `"home"`; `create_profile()` creates `home/` inside the profile dir (`:155-171`).
- **`TestPythonProcessUnchanged`** (`:178-198`) — captures `os.environ.get("HOME")` and `Path.home()` before resolving subprocess home, asserts both are unchanged after — proves the subprocess injection never leaks back to the parent.

### `test_install_sh_setup_wizard_tty_probe.py`

Pins issue #16746: `[ -e /dev/tty ]` returns true in Docker builds (the device node exists in the mount namespace) but `< /dev/tty` redirection then fails with ENXIO. Two parametric tests (`:47-91`):

- **`test_tty_gate_does_not_use_existence_only_check`** — for each of `("run_setup_wizard", "install_system_packages", "maybe_start_gateway")`, extract the function body via regex from `scripts/install.sh`, search for any spelling of `[ -e /dev/tty ]` or `test -e /dev/tty`, fail if found.
- **`test_tty_gate_uses_open_based_probe`** — same extraction, then assert the function body contains an `if`/`elif` whose condition redirects stdin from `/dev/tty` (`(: </dev/tty)`, `exec 3</dev/tty`, etc.).

The pattern: **assert higher-level invariants over source-text** when the actual environment (Docker build with no real TTY) is hard to reproduce in CI.

### `test_atomic_replace_symlinks.py`

Pins issue #16743: `os.replace(tmp, target)` swaps a symlink for a regular file, silently detaching managed-deployment configs that symlink `~/.hermes/config.yaml` to a git-tracked profile package. Five tests (`:40-160`):

- **`test_atomic_replace_preserves_symlink`** — symlink survives, real file gets new content.
- **`test_atomic_replace_regular_file`** — non-symlink target works normally.
- **`test_atomic_replace_first_time_create`** — target doesn't exist yet, write succeeds.
- **`test_atomic_replace_accepts_pathlike_and_str`** — duck-typing on the path arg.
- **`test_atomic_json_write_preserves_symlink`** + **`test_atomic_yaml_write_preserves_symlink`** — same property at the wrapper layer.
- **`test_atomic_json_write_preserves_symlink_permissions`** (`:123-138`) — POSIX-only (`pytest.skip("POSIX-only")`); writes through symlink, asserts the real file's `0o644` permission bits are preserved.
- **`test_atomic_replace_broken_symlink_creates_target`** (`:144-160`) — broken symlink (target missing) — write should create the real file via `realpath` resolution rather than turn the symlink into a regular file.

### `test_packaging_metadata.py` (`:1-22`)

Tiny but important. Loads `pyproject.toml` via `tomllib` and asserts:

- `faster-whisper` is **not** in `project.dependencies` (it's a wheel-only Android-incompatible dep).
- `faster-whisper` **is** in the `voice` optional extra.
- `MANIFEST.in` contains `graft skills` and `graft optional-skills`.

This is the canonical "we got bitten once by accidentally shipping faster-whisper as a base dep on Termux/Homebrew" regression test.

## 6. Tool / MCP / Plugin Tests

### `test_model_tools.py`

`360 lines`, dispatches by name through `handle_function_call`. Major test classes:

- **`TestHandleFunctionCall`** (`:22-114`): `_AGENT_LOOP_TOOLS` (e.g. `todo`, `memory`, `session_search`, `delegate_task`) return an "agent loop" error when dispatched directly (they're meant to be invoked from inside `run_agent`); unknown tools return JSON errors; **plugin hooks fire in canonical order** (`pre_tool_call`, `post_tool_call`, `transform_tool_result`) with `tool_name`, `args`, `result`, `task_id`, `session_id`, `tool_call_id`, `duration_ms` kwargs (`test_tool_hooks_receive_session_and_tool_call_ids`, `:43-86`).
- **`test_post_tool_call_receives_non_negative_integer_duration_ms`** (`:88-113`) — `duration_ms` is a non-negative integer, `pre_tool_call` does NOT receive it (nothing has run yet), `post_tool_call` and `transform_tool_result` see the same value.
- **`TestPreToolCallBlocking`** (`:136-...`): a hook returning `[{"action": "block", "message": "..."}]` short-circuits dispatch entirely; malformed hook returns are ignored; `skip_pre_tool_call_hook=True` prevents double-fire.

### `test_get_tool_definitions_cache_isolation.py` — cache invariants

Pins issue #17335: `get_tool_definitions(quiet_mode=True)` memoised the schema list, and the Gateway then mutated it in place (`run_agent` appended LCM tool schemas), poisoning every subsequent agent init with duplicate tool names that DeepSeek/Kimi/Moonshot rejected with HTTP 400. Five tests (`:32-94`):

- `test_first_uncached_call_returns_fresh_list` — first call's return value `is not` the cached object (identity check).
- `test_cache_hit_returns_fresh_list` — pre-fix behaviour (cache-hit copy) is pinned alongside the new fix.
- `test_caller_mutation_does_not_poison_cache` — caller appends two entries; second fetch returns baseline length.
- `test_repeated_caller_mutation_does_not_accumulate` — five iterations of mutate-then-refetch; final length still equals baseline.
- `test_non_quiet_mode_does_not_use_cache` — `quiet_mode=False` skips the cache entirely (`assert len(model_tools._tool_defs_cache) == 0`).

The autouse fixture `_clear_cache` (`:24-29`) clears `model_tools._tool_defs_cache` at the start AND end of every test.

### `test_mcp_serve.py` — three-layer test pyramid

`1111 lines`. The file's docstring explicitly names the layers:

> "Three layers of tests:
> 1. Unit tests — helpers, content extraction, attachment parsing
> 2. EventBridge tests — queue mechanics, cursors, waiters, concurrency
> 3. End-to-end tests — call actual MCP tools through FastMCP's tool manager with real session data in SQLite and sessions.json"
>
> source: `.resources/hermes/tests/test_mcp_serve.py:1-9`

Layer 1 (`:214-302`) — `_load_sessions_index` corrupt JSON returns `{}`; `_extract_message_content` handles text/multipart/None; `_extract_attachments` parses `image_url` blocks, `MEDIA: /path` text tags, multiple media tags.

Layer 2 (`:308-434`) — `EventBridge`: empty/poll/cursor-filter/session-filter/queue-limit/concurrent-enqueue/wait-immediate/wait-timeout/wait-wakes-on-enqueue. The concurrency test spawns 5 threads × 100 enqueues, asserts `len(b._queue) == 500` and `b._cursor == 500`.

Layer 3 (`:441-...`) — sets up a **real SQLite DB with a populated schema** (`_create_test_db`, `:124-167`) and a real `sessions.json` (`populated_sessions_dir`), then runs MCP tools through `server._tool_manager.call_tool(name, args or {})`. Tools tested: `conversations_list`, `conversation_get`, `messages_read`, `attachments_fetch` — sorting, filter-by-platform, search, limit, role exclusion (no `role=tool` in `messages_read` results).

Module-level autouse `_isolate_hermes_home` (`:27-36`) overrides `hermes_constants.get_hermes_home` to return `tmp_path` directly — defence-in-depth on top of the global conftest.

### `test_plugin_skills.py`

Covers the plugin namespacing rules — `parse_qualified_name`, `is_valid_namespace`, `register_skill` invariants:

- `:21-68` — `parse_qualified_name("a:b:c")` splits on the first colon, returns `("a", "b:c")`; `is_valid_namespace` rejects `bad.name`, `bad/name`, `bad name`, empty, None.
- `:73-119` — `PluginManager.find_plugin_skill` resolves `"myplugin:foo"` → `Path`; `list_plugin_skills` returns sorted bare names; `remove_plugin_skill` is idempotent.
- `:122-160` — `PluginContext.register_skill` rejects `:` in name, invalid chars, missing files.
- **Stale-entry self-heal** (`:244-253`) — register a skill, delete the file behind the registry's back, then `skill_view("superpowers:writing-plans")` fails AND the registry entry is removed.
- **Plugin guards** (`:256-312`):
  - `disabled` plugins return error from `skill_view` (`:281-289`).
  - Platform-mismatch (`platforms: [linux]` on macOS) returns `"not supported on this platform"` (`:291-299`).
  - Prompt-injection content (`Ignore previous instructions`) is **served** but **logged at WARNING** with `caplog.at_level(logging.WARNING, logger="tools.skills_tool")` (`:301-312`).
- **Bundle context banner** (`:315-373`) — sibling skills appear in the `Bundle context` banner, self does not; single-skill bundle has no `Sibling skills:` line; original SKILL.md body content is preserved.

The pattern: **registry invariants + adversarial inputs + observability checks** all in one file.

### `test_toolsets.py`

Asserts `validate_toolset` against the live registry (`test_registry_membership_is_live`, `:170-185`) — registers a fake toolset at runtime, validates it, then verifies cleanup. This is hermes' answer to "test the registry without monkey-patching the registry".

## 7. Security Tests

### `test_sql_injection.py`

Tiny (`44 lines`) but encodes the contract precisely:

> ```python
> def test_session_cols_no_injection_chars():
>     """_SESSION_COLS must not contain SQL injection vectors."""
>     cols = InsightsEngine._SESSION_COLS
>     assert ";" not in cols
>     assert "--" not in cols
>     assert "'" not in cols
>     assert "DROP" not in cols.upper()
> ```
>
> source: `.resources/hermes/tests/test_sql_injection.py:8-14`

Plus parametric probes that the prepared queries `_GET_SESSIONS_ALL` and `_GET_SESSIONS_WITH_SOURCE` use `?` placeholders, contain `started_at >= ?`, contain `source = ?`, and **do not contain `{`** (regression guard against accidentally f-stringing a runtime value back in). And a regex `^[a-zA-Z_][a-zA-Z0-9_]*$` over each column identifier — every column name must match the safe SQL identifier shape.

The pattern: **assert what is forbidden in the static text, not what is filtered at the boundary**. Cheap, fast, catches the regression class outright.

The stress suite extends this with adversarial inputs at the kernel level:

> "**test_atypical_scenarios.py** — 28 scenarios covering atypical user inputs: unicode/emoji/RTL, 1 MB strings, SQL injection attempts, cycles, self-parents, wide fan-in/out, clock skew, HERMES_HOME with spaces/unicode/symlinks, 1000 runs on one task, idempotency-key race across processes, terminal-state resurrection attempts, dashboard REST with weird JSON."
>
> source: `.resources/hermes/tests/stress/README.md:32-38`

### `test_minimax_oauth.py`

The most thorough credential-flow test in the repo. Coverage (`:74-466`):

- **PKCE pair generation** — `_minimax_pkce_pair()` returns (verifier, challenge, state); assert verifier ≥ 32 chars, challenge contains no `=` (URL-safe base64), challenge equals `urlsafe_b64encode(sha256(verifier))`, state ≥ 8 chars, two calls return different values.
- **Authorization request happy path** — POST to `/oauth/code`, body matches `state`, response parsed correctly, custom `x-request-id` header present.
- **CSRF state-mismatch** — server returns `state="wrong-state"` while caller sent `"correct-state"`; `_minimax_request_user_code` raises `AuthError(code="state_mismatch")` with `"CSRF" in str(exc) or "mismatch" in str(exc).lower()`.
- **HTTP 400 path** — `_make_httpx_response(400, text="Bad Request")` raises `AuthError(code="authorization_failed")`.
- **Polling: pending→success** — `client.post.side_effect = [pending_resp, pending_resp, success_resp]`; `with patch("time.sleep")` so the poll loop doesn't actually wait; assert `call_count == 3`, `result["status"] == "success"`.
- **Polling: error status** — `error_resp` with `"status": "error"` raises `AuthError(code="authorization_denied")`.
- **Polling: timeout** — `expired_in=past_deadline_ms`; `AuthError(code="timeout")`.
- **Refresh skip when not expired** — token has `expires_at = future + 3600s`, `_refresh_minimax_oauth_state(state) is state` (no refresh).
- **Refresh updates access_token** — token close to expiry, `httpx.Client` is patched at the module level with a context-manager mock, `_minimax_save_auth_state` is also patched out.
- **Reuse of refresh-token triggers `relogin_required`** — bad response with `text="invalid_grant"`; `AuthError(code="refresh_failed", relogin_required=True)`.
- **Resolve credentials requires login** — `get_provider_auth_state` returns `None`; `resolve_minimax_oauth_runtime_credentials()` raises `AuthError(code="not_logged_in", relogin_required=True)`.
- **Provider registry self-consistency** — `"minimax-oauth" in PROVIDER_REGISTRY`, `auth_type == "oauth_minimax"`, both global and CN base/inference URLs present.
- **Auth status (logged-out vs logged-in)** — separate tests, separate `with patch(...)` blocks.

All of this with **no real HTTP**, **no real time** (`with patch("time.sleep")`), **no real auth store** (`patch("hermes_cli.auth._minimax_save_auth_state")`). Every test is < 1ms.

### `test_ipv4_preference.py`

Network safety. `apply_ipv4_preference(force=True)` monkey-patches `socket.getaddrinfo` to rewrite `AF_UNSPEC` (the default) to `AF_INET`, with fallback to AF_UNSPEC on `gaierror`. Tests (`:17-114`):

- `setup_method`/`teardown_method` (xUnit-style for state isolation across the same class) — save and restore `socket.getaddrinfo` since the module-level patch otherwise leaks across tests in the same xdist worker.
- `test_noop_when_force_false`, `test_double_patch_is_safe`, `test_af_unspec_becomes_af_inet`, `test_explicit_family_preserved`, `test_fallback_on_gaierror`.
- `test_network_section_in_default_config` — config DEFAULT_CONFIG has `network: {force_ipv4: False}`.

The pattern: **monkey-patches that survive across tests must have explicit setup/teardown**, even if the autouse fixtures handle most of it.

## 8. UI / TUI Tests

`ui-tui/vitest.config.ts` is intentionally minimal — `defineConfig({ test: { exclude: ['dist/**', 'node_modules/**'] } })`. Vitest finds the `*.test.ts` files in `ui-tui/src/__tests__/` and `ui-tui/packages/hermes-ink/src/ink/`.

Sample TUI test files:
- `ui-tui/src/__tests__/virtualHeights.test.ts` — virtualization invariants for the chat history.
- `ui-tui/src/__tests__/scroll.test.ts` — wheel/keyboard scroll bookkeeping.
- `ui-tui/src/__tests__/streamingMarkdown.test.ts` — streaming chunk → rendered markdown.
- `ui-tui/src/__tests__/textInputWrap.test.ts` — multiline input wrap-at-column behaviour.
- `ui-tui/src/__tests__/createGatewayEventHandler.test.ts` — gateway-event router for the TUI.
- `ui-tui/packages/hermes-ink/src/ink/parse-keypress.test.ts` — raw-mode keypress parsing.
- `ui-tui/packages/hermes-ink/src/ink/selection.test.ts` — text selection logic.
- `ui-tui/packages/hermes-ink/src/ink/colorize.test.ts` — ANSI colour writer.

The Python-side TUI tests live in `tests/test_tui_gateway_server.py` (`4133 lines`) — they exercise the JSON-RPC gateway between the TUI and `run_agent`. Coverage includes:

- `test_write_json_serializes_concurrent_writes` (`:35-53`) — 8 threads write through a `_ChunkyStdout` (1-char-at-a-time write), assert 8 valid JSON lines came out — proves writes are atomic across threads.
- `test_write_json_returns_false_on_broken_pipe` (`:56-59`) — `_BrokenStdout` raises `BrokenPipeError`; `server.write_json` catches it and returns `False` rather than crashing the agent.
- `test_dispatch_rejects_non_object_request` (`:62-69`) — `dispatch([])` returns the canonical JSON-RPC `-32600 invalid request: expected an object` shape.
- `test_dispatch_rejects_non_object_params` (`:72-81`) — `params: []` returns `-32602`.
- `test_load_enabled_toolsets_*` (`:84-200`) — env-var resolution priority: `HERMES_TUI_TOOLSETS` over plugin-discovered tools over config; malformed entries are filtered with a warning to stderr; `all` means `None` (sentinel meaning "all tools").
- `test_session_create_no_race_keeps_worker_alive` (`:2725`) — race between session creation and worker installation.
- `test_handle_skin_command_refreshes_live_tui` (`test_cli_skin_integration.py:82-91`) — `/skin ares` updates the running prompt_toolkit Application's style and triggers `_invalidate(min_interval=0.0)` so the screen redraws immediately.

### `test_model_picker_scroll.py`

Pure-logic test of a scrolling viewport in `_curses_prompt_choice`, with no real curses. Defines `_compute_scroll_offset(cursor, scroll_offset, visible, n_choices)` as a Python copy of the production block, then exhaustively tests:

- Cursor at zero ⇒ no scroll.
- Cursor wraps from 0 to last ⇒ last item visible.
- Cursor above current window ⇒ offset tracks cursor.
- Window larger than list ⇒ offset clamped to 0.
- **Invariant simulation**: `test_full_navigation_down_cursor_always_visible` simulates 13+2 down-keypresses, asserts cursor in rendered window after every step.

The pattern: **extract pure logic from a UI controller into a free function, exhaustively unit-test the function**. The controller's keypress dispatch is a thin trivially-correct wrapper.

## 9. Patterns AGH Should Adopt (verbatim or close)

For each pattern below, the **AGH equivalent** column names the package or surface to reuse — not just where to put the test, but what production code it constrains.

- **Hermetic env shield (autouse global)** — *Why it matters*: every "works locally fails CI" incident hermes documents traces back to a credential or behavioural env var leaking into a test. AGH has the same surface: provider keys, `AGH_HOME`, `AGH_TOKEN`, MCP and bundle env, peer identity, daemon ports. *AGH equivalent*: a single `internal/test/env` helper used as `t.Cleanup`/`t.Setenv` block at the start of every package's `TestMain` — see `.resources/hermes/tests/conftest.py:244-307` for the canonical list shape. Do **NOT** also redirect `$HOME` (hermes documented why at `:265-270`); only redirect `AGH_HOME` and `AGH_CONFIG_HOME`.

- **Module-state reset between tests** — *Why*: Go has package-level `var`s that survive across tests in the same `go test` binary. hermes' `_reset_module_state` resets 8 buckets per test (`conftest.py:333-441`). *AGH equivalent*: registry resets in `internal/runtime/registry`, autonomy queue resets in `internal/autonomy/kernel`, hook bus reset in `internal/hooks`, ContextVar-equivalent (`context.WithValue` chains) cleanup. Add a `pkg.ResetForTesting()` per package with a build tag `//go:build testing` — the AGH `make test` target should set the tag.

- **Pytest markers ⇒ Go build tags** — `integration` ⇒ `//go:build integration`, `e2e` ⇒ `//go:build e2e`, `live` ⇒ `//go:build live`, `stress` ⇒ `//go:build stress`. Default `make test` runs only the no-tag set (matches hermes `addopts = "-m 'not integration'"` at `pyproject.toml:151`). `make test-integration`, `make test-e2e-runtime`, `make test-stress` are explicit, opt-in. *AGH equivalent*: extends the existing `make test-integration` / `make test-e2e-runtime` / `make test-e2e-web` lanes — add `make test-live` for credential-bearing real-LLM tests so AGH has a fourth lane that mirrors hermes' integration lane behaviour.

- **Credential-gated live tests** — *Why*: real LLM coverage is the reason this whole study exists. hermes' move is **integration-marked + in-test runtime probe**: `if os.getenv("MODAL_TOKEN_ID") is None: pytest.skip(...)` (`tests/integration/test_modal_terminal.py:69-84`). *AGH equivalent*: `internal/llm/...` real-provider tests under `//go:build live`, with `t.Skip("AGH_LIVE_OPENAI_KEY not set")` if the env var is missing. **Always log the absent-credential reason** so a CI pass without the secret is visibly partial.

- **Subprocess HOME isolation** — *Why*: the AGH daemon spawns ACP subprocesses (Claude Code, OpenClaw, Hermes). Each must see a deterministic `HOME` that doesn't leak between sessions. *AGH equivalent*: tests under `internal/acp/process` and `internal/runtime/sandbox` mirroring `tests/test_subprocess_home_isolation.py:80-148` — for `MakeRunEnv()` and `SanitizeSubprocessEnv()`. Pin `os.Getenv("HOME")` and `os.UserHomeDir()` are unchanged after subprocess env construction (`TestPythonProcessUnchanged`, `:178-198`).

- **Cache-isolation test** — *Why*: AGH already has at least three caches (tool-definition cache, capability cache, peer-card cache, OpenAPI codegen cache). Each of them is a candidate for the issue #17335 class of bug (mutation poisoning the cache for the next caller). *AGH equivalent*: for every `package-level cache`, a `TestCache_CallerMutationDoesNotPoisonCache_<name>` that fetches, mutates the result, refetches, asserts identity NOT EQUAL and length unchanged (mirror `test_get_tool_definitions_cache_isolation.py:32-94`).

- **Async/cancel rigor** — *Why*: AGH is heavy on goroutines, channel orchestration, and context cancellation. The hermes pattern is: prove (a) cancellation propagates within a deadline you assert on (e.g. `< 2.0s` from `parent.interrupt()` in `run_interrupt_test.py:135-138`), (b) shutdown does not block the caller (`shutdown(wait=False)` invariant), (c) per-thread/per-goroutine resources don't share a closed loop. *AGH equivalent*: `internal/autonomy/kernel` cancellation tests, `internal/acp/session` interrupt tests; assert wall-clock bound on cancel propagation; assert no goroutine leak via `goleak.VerifyNone(t)`.

- **Source-text invariant tests** — *Why*: when a regression is in shell, SQL, or generated code, the easiest defence is a regex that fails if the bad pattern reappears. `test_install_sh_setup_wizard_tty_probe.py:47-91` and `test_sql_injection.py:8-14` are the two reference shapes. *AGH equivalent*: contract regression tests for `openapi/agh.json` (no fields with empty descriptions; every operation has an `operationId`), shell regression tests for `scripts/install.sh` if AGH ships one, codegen drift tests for `web/src/generated/agh-openapi.d.ts`.

- **Three-layer test pyramid for I/O-heavy modules** — `test_mcp_serve.py` is the model: **(1) unit helpers + extraction** → **(2) data-structure mechanics under concurrency** → **(3) end-to-end with a real SQLite DB and real fixture files**. *AGH equivalent*: `internal/api/mcp` (the AGH MCP surface), `internal/api/uds` (the UDS protocol surface), `internal/network/protocol` should each have this three-tier shape with the third tier using a temp BadgerDB / SQLite.

- **OAuth/credential round-trip with mocked transport** — *Why*: the real auth provider is unreliable in CI and slow. hermes proves correctness of every error code (`state_mismatch`, `authorization_failed`, `authorization_denied`, `timeout`, `refresh_failed` with `relogin_required=True`, `not_logged_in`) against a `MagicMock`-shaped response object. *AGH equivalent*: AGH-Network identity flow, peer card refresh, federated auth — every error code path in `internal/network/identity` and `internal/auth` deserves the same coverage.

- **Property-style invariant fuzzing for state machines** — *Why*: hermes' `tests/stress/test_property_fuzzing.py` runs 500 random op sequences × 100 ops each (~50k ops total) and asserts 9 invariants after every step. *AGH equivalent*: AGH autonomy kernel has the same shape (task statuses, claim_lock, current_run_id, parent-child completion). Adopt the I1..I9 invariants under `internal/autonomy/kernel/stress` with a `//go:build stress` tag.

- **Atomic-replace symlink pin** — *Why*: AGH's config and bundle persistence likely uses atomic write helpers. The exact bug class hermes pinned at `tests/test_atomic_replace_symlinks.py` is real for any tool that supports symlinked configs (the standard managed-deployment shape). *AGH equivalent*: `internal/persist/atomic` tests — test the symlink-survives, broken-symlink-creates-target, permissions-preserved cases.

- **Packaging drift test** — *Why*: hermes' `test_packaging_metadata.py:1-22` catches the "we put faster-whisper in base deps" regression in 0.05ms. *AGH equivalent*: `cmd/agh/build_test.go` that loads `go.mod` and asserts (a) no `replace` directives in non-dev mode, (b) no `=> ../` local replaces, (c) `go.sum` and `go.mod` are consistent, (d) for the bun side, `package.json` workspaces don't pull in dev-only packages at runtime.

- **TTY/Docker-build regression for installers** — *Why*: AGH ships an installer; the install.sh `[ -e /dev/tty ]` ENXIO bug is generic. *AGH equivalent*: if AGH has a `scripts/install.sh`, port `test_install_sh_setup_wizard_tty_probe.py:47-91` verbatim — it's a regex test, no Python execution required.

- **Failure messages that explain WHY** — `assert all_open, "At least one worker thread's loop was closed"` (`test_model_tools_async_bridge.py:153`) instead of bare `assert`. *AGH equivalent*: every Go assertion uses `require.Truef(t, cond, "<reason>: got=%v want=%v")` rather than the unhelpful default. Hermes' style of pasting an issue number into the error string also pays back during triage: `# (issue #2104)`.

- **Hermetic test runner wrapper** — *Why*: belt-and-suspenders against direct-pytest invocations. `scripts/run_tests.sh` unsets credentials and pins worker count even before pytest starts (`run_tests.sh:50-79`). *AGH equivalent*: `scripts/run_tests.sh` (or extend `make test`) that exports `TZ=UTC`, `LANG=C.UTF-8`, **scrubs every `*_API_KEY`/`*_TOKEN`/`*_SECRET`** from the calling shell, then `exec go test -race -count=1 ./...`.

- **Worker-count parity** — hermes pins `-n 4` to match GHA (`run_tests.sh:81-86`). *AGH equivalent*: pin `GOMAXPROCS=4` in `make test` to match a 4-core CI runner — flaky-ordering bugs only surface when local and CI use the same parallelism. Same for vitest workers in `web/`.

- **`importorskip` for optional dependencies** — *Why*: hermes' `pytest.importorskip("nacl.secret")` (`tests/integration/test_voice_channel_flow.py:17`) lets the test stay green when an optional extra isn't installed. *AGH equivalent*: build-tag-gated test files (`//go:build voice`, `//go:build mcp_native`) for optional bridges, plus a CI matrix that runs the build-tagged subset only on platforms that ship the dep.

- **Evidence capture on failure** — every assert message with run-time values, every test docstring with the issue reference, every regression file with the root cause in the module docstring. *AGH equivalent*: `cy-final-verify` discipline plus an internal convention: every regression PR adds a Go test with `// regression: github.com/compozy/agh/issues/NNN` in the file header and a `t.Logf(...)` of the relevant production state on failure.

## 10. Patterns AGH Should NOT Adopt

- **Autouse fixtures with no opt-out** — pytest's autouse machinery is fine in Python, but Go has no direct equivalent. Doing the same with `TestMain` per package is acceptable; doing it via init() side-effects is **forbidden** (it makes test-binary load order observable). Keep AGH's hermetic shield as a `t.Helper()` you call explicitly at the top of every test, even at the cost of some repetition.

- **Module-level singletons reset by hand-curated lists** — hermes' `_reset_module_state` (`conftest.py:333-441`) is **necessary** because Python modules are per-process singletons; that's a Python language constraint. In Go, the equivalent — a hand-curated list of `pkg.X.Reset()` calls in every test — is a **design smell**. Each package should expose a `New(opts...)` constructor; tests should construct fresh instances. AGH should treat the need for `pkg.ResetForTesting()` as a sign the package has hidden global state to refactor.

- **Monkey-patching transport layers per-test** — `with patch("hermes_cli.auth._minimax_save_auth_state"): ...` (`test_minimax_oauth.py:357`) is fine in Python where dynamism is free. The Go equivalent (`monkey` package, function pointer swapping) is forbidden by `agh-code-guidelines` and `testing-boss`. Use **interfaces + dependency injection at the call site** instead — pass an `AuthStore` interface, swap a `fakeAuthStore` in tests.

- **Source-text regex tests for Go production code** — hermes uses regex-over-`install.sh` because shell has no AST. Go has `go/parser` and `go/ast`. If AGH ever needs to assert "no eager `AsyncOpenAI` constructor outside `_get_async_client`" (`test_trajectory_compressor_async.py:97-111`), use `go/ast` to walk the AST — far more robust than regex.

- **`unittest.TestCase` mixed with pytest** — `test_yuanbao_markdown.py` still uses `unittest.TestCase`; the rest of hermes uses plain `class TestX:`. The mixed style is a hermes-specific accumulation and is not a pattern to copy. AGH should pick one style (`*testing.T` plus `t.Run`) and stick to it (this is already enforced by `agh-test-conventions`).

- **`SIGALRM`-based per-test timeouts** — `_enforce_test_timeout` (`conftest.py:511-522`) uses `signal.alarm(30)`; it's Unix-only and skipped on Windows. Go has `t.Deadline()` and `context.WithTimeout` — use them. Don't try to install signal handlers in tests.

- **`pytest.importorskip` semantics** — Go doesn't have dynamic imports; build tags are the equivalent. Don't try to fake `importorskip` with a runtime `if _, err := exec.LookPath("foo"); err != nil { t.Skip() }` — gate at build time.

- **`addopts = "-n auto"` at config level overridden by a wrapper script** — hermes does this because the default `pytest` invocation has to be sane for IDE integrations. Go's `go test` is already deterministic in worker count; just pass `-p N -parallel M` explicitly. Don't add a wrapper that changes flags behind the user's back.

- **Returning `MagicMock` shapes with `.json.return_value = body` and asserting on `.json.side_effect`** — Python-only style. The Go answer is a fake server (`httptest.NewServer`) with a `http.HandlerFunc`. `tests/fakes/fake_ha_server.py` is closer to the right idea (real TCP, real WS) and is what AGH should copy, not the MagicMock approach.

## 11. Open Questions for AGH

1. **Where does AGH draw the line between "live" and "integration"?** hermes has just one marker (`integration`) and treats live-LLM as in-test runtime probes. AGH already has `make test-integration`, `make test-e2e-runtime`, `make test-e2e-web`, `make test-e2e-nightly`. Should `live` be its own lane (with credential probes inside) or rolled into `e2e-nightly`?

2. **How is per-test `AGH_HOME` isolation reconciled with parallel worktrees?** hermes uses `tmp_path` per test; the worktree-isolation skill mandates unique `AGH_HOME` + ports + tmux-bridge sockets per worktree. Do unit tests need a port allocator, or is `httptest.NewServer` (random port) enough?

3. **What is AGH's equivalent of hermes' `tests/fakes/`?** hermes has `fake_ha_server.py` for the Home Assistant adapter — an in-process real-TCP fake. AGH has `internal/network` and `internal/api/mcp` — should these get parallel `fake_*_server.go` test helpers, or is `httptest` + `pion/webrtc` enough?

4. **How are `live`-tagged tests gated in CI when secrets are not available on PRs from forks?** GHA's secret-injection rules block forks. hermes' answer is "live tests live in `tests/integration/` and just don't run on fork PRs". AGH's existing CI (no cron, heavy tests in release-PR dry-run) needs a precise rule for `live`.

5. **Is there an equivalent to hermes' `test_packaging_metadata.py` for Go modules?** hermes pins "no faster-whisper in base deps" in 4 lines. The AGH equivalent is harder because Go modules don't have optional dependencies — but `go.mod` `replace` directives, `tools.go`-only imports, and Bun workspace boundaries are real risks worth pinning.

6. **Should AGH adopt hermes' "no change-detector tests" rule (AGENTS.md:717-758) as a contributor-facing rule?** It's a strong signal for what to write and what to delete; it would be a natural addition to `internal/CLAUDE.md` and `agh-test-conventions`.

## Citations

- `.resources/hermes/README.md` — agent identity, supported providers, install model. Anchor for "what runtime is being tested".
- `.resources/hermes/AGENTS.md` (lines 56, 654-765) — testing rules, hermetic-runner wrapper, change-detector ban, profile-test patterns.
- `.resources/hermes/CONTRIBUTING.md` (lines 106-119, 462, 578-635) — entry-point test command, project structure, commit-message convention.
- `.resources/hermes/pyproject.toml:146-176` — single `integration` marker, default `addopts`, `dev` extras (pytest, pytest-asyncio, pytest-xdist).
- `.resources/hermes/scripts/run_tests.sh` — canonical wrapper that pins TZ/LANG/PYTHONHASHSEED, scrubs credentials, pins `-n 4`, ignores `tests/integration` and `tests/e2e`.
- `.resources/hermes/tests/conftest.py` — five autouse fixtures, ~70-name credential blocklist, ~50-name HERMES_* behavioural blocklist, 8-bucket module-state reset, SIGALRM 30s timeout, registry-cache reset.
- `.resources/hermes/tests/e2e/conftest.py` — `MagicMock` substitutes for telegram/discord/slack libraries so adapter classes import without optional libraries; `E2E_MESSAGE_SETTLE_DELAY = 0.3`.
- `.resources/hermes/tests/stress/conftest.py` — `--run-stress` opt-in flag, default-skip everything in `tests/stress/`, `collect_ignore_glob = ["*.py"]`.
- `.resources/hermes/tests/stress/README.md` — six stress files: concurrency (5 workers × 100 tasks), mixed (10 + 1 reclaimer × 500 tasks), reclaim-race (TTL < work duration), subprocess-e2e (real Python subprocess workers), property-fuzzing (500 sequences × 100 ops × 9 invariants), benchmarks (latency at 100/1k/10k tasks).
- `.resources/hermes/tests/stress/test_property_fuzzing.py:1-100` — exhaustive 9-invariant Kanban-kernel test (I1..I9) referenced for AGH autonomy kernel.
- `.resources/hermes/tests/test_account_usage.py` — provider-account fetch with `_RoutingClient` URL→payload mapping fake.
- `.resources/hermes/tests/test_atomic_replace_symlinks.py:1-160` — 7 tests pinning issue #16743 atomic-write symlink-preservation.
- `.resources/hermes/tests/test_get_tool_definitions_cache_isolation.py:1-94` — 5 tests pinning issue #17335 cache mutation; the `_clear_cache` autouse fixture pattern.
- `.resources/hermes/tests/test_hermes_state.py:1-200` — `SessionDB` fixture, session lifecycle, end-session-preserves-original-end-reason invariant, parent-child sessions, FTS5 search (later in file).
- `.resources/hermes/tests/test_install_sh_setup_wizard_tty_probe.py:1-91` — regex tests over `scripts/install.sh` for issue #16746 ENXIO bug; parametric over three gated functions.
- `.resources/hermes/tests/test_ipv4_preference.py:1-114` — `socket.getaddrinfo` monkey-patch with explicit setup_method/teardown_method to stop leaks across tests in the same xdist worker.
- `.resources/hermes/tests/test_mcp_serve.py:1-600` — three-layer test pyramid (unit, EventBridge, FastMCP E2E with real SQLite); autouse `_isolate_hermes_home` over the global one for defence in depth.
- `.resources/hermes/tests/test_minimax_oauth.py:1-466` — exhaustive OAuth flow coverage with `_make_httpx_response` helper, `_future_iso`/`_past_iso` time helpers, every error code (state_mismatch / authorization_failed / authorization_denied / timeout / refresh_failed / not_logged_in).
- `.resources/hermes/tests/test_model_picker_scroll.py:1-119` — `_compute_scroll_offset` extracted from production for pure-logic invariant tests.
- `.resources/hermes/tests/test_model_tools.py:1-200` — `handle_function_call` dispatch, agent-loop tool detection, plugin-hook firing order with `tool_name`/`args`/`result`/`task_id`/`session_id`/`tool_call_id`/`duration_ms` kwargs.
- `.resources/hermes/tests/test_model_tools_async_bridge.py:1-447` — issue #2104 event-loop persistence; 8 distinct invariants (loop survives, same loop reused, transport survives, per-thread loops, worker ≠ main, async-context fallback, non-blocking shutdown, cancellation reaches worker).
- `.resources/hermes/tests/test_ollama_num_ctx.py:1-136` — `_mock_httpx_client` helper; tests prefer-explicit-num_ctx-over-architecture-default; provider-prefix stripping; arch-key variants (`llama.context_length`, `qwen2.context_length`).
- `.resources/hermes/tests/test_packaging_metadata.py:1-22` — `pyproject.toml` deps validation + `MANIFEST.in` skill grafts.
- `.resources/hermes/tests/test_plugin_skills.py:1-373` — namespace parsing, valid-namespace regex, plugin-context register-skill, `skill_view` qualified-name dispatch with stale-entry self-heal, prompt-injection-served-but-logged, bundle-context banner with sibling listing.
- `.resources/hermes/tests/test_retry_utils.py:1-118` — `jittered_backoff` exponential growth, max_delay clamp, jitter randomness, thread safety with `threading.Barrier`, monkey-patched seed-derivation.
- `.resources/hermes/tests/test_sql_injection.py:1-44` — `_SESSION_COLS` adversarial-character ban; placeholder-shape pin; safe-identifier regex per column.
- `.resources/hermes/tests/test_subprocess_home_isolation.py:1-199` — issue #4426 per-profile HOME isolation, two-profile distinctness, `_make_run_env`/`_sanitize_subprocess_env` injection, `Path.home()`/`os.environ["HOME"]` unchanged after subprocess env construction.
- `.resources/hermes/tests/test_trajectory_compressor_async.py:1-202` — lazy `AsyncOpenAI` creation; `TestSourceLineVerification` regex-over-source for the eager-construction regression.
- `.resources/hermes/tests/test_tui_gateway_server.py:1-200` — concurrent `write_json` serialisation, JSON-RPC error shapes, env-var precedence for toolset selection, plugin-discovery-after-init.
- `.resources/hermes/tests/test_yuanbao_pipeline.py` — InboundPipeline middleware engine + 12 concrete middlewares; onion-model invariant; OOP+functional middleware mixing; `parametrize` over the middleware-class table.
- `.resources/hermes/tests/test_yuanbao_integration.py` — adapter init, platform enum, GatewayConfig connected-platforms, proto round-trip, markdown chunking, manager imports, P0 reconnect/lock/eviction guards.
- `.resources/hermes/tests/test_yuanbao_markdown.py:1-100` — `unittest.TestCase`-style boundary-split tests (legacy style retained; not a model to copy).
- `.resources/hermes/tests/test_yuanbao_proto.py` — protobuf encode/decode round-trip coverage (referenced from `test_yuanbao_integration.py:182-200`).
- `.resources/hermes/tests/run_interrupt_test.py:1-148` — script-mode (NOT pytest) live test of `_run_single_child` interrupt propagation under `< 2.0s`.
- `.resources/hermes/tests/integration/test_modal_terminal.py:1-120` — `pytestmark = pytest.mark.integration`, in-test `os.getenv("MODAL_TOKEN_ID")` probe, runnable as both pytest and standalone script.
- `.resources/hermes/tests/integration/test_ha_integration.py:1-100` — `pytestmark = pytest.mark.integration`, `FakeHAServer` real TCP/WebSocket fake, full WS handshake (auth_required → auth → auth_ok → subscribe → ACK).
- `.resources/hermes/tests/integration/test_voice_channel_flow.py:17-18` — `pytest.importorskip("nacl.secret")` and `discord` for optional-dep gating.
- `.resources/hermes/tests/integration/test_batch_runner.py:1-50` — script-style integration test with `pytestmark`, also runnable as `__main__`.
- `.resources/hermes/tests/fakes/fake_ha_server.py` — referenced from `test_ha_integration.py`; in-process real-TCP fake server pattern.
- `.resources/hermes/RELEASE_v0.12.0.md:418-419` — single sentence on testing in the release: "Hermetic test parity (scripts/run_tests.sh) held across this window. Microsoft Teams xdist collision guard — prevents worker collisions when Teams platform tests run in parallel (#17828)" — hermes treats hermetic-test-parity as a release-quality signal.
- `.resources/hermes/ui-tui/vitest.config.ts` — minimal vitest config (`exclude: ['dist/**', 'node_modules/**']`), the rest is convention over config.
- `.resources/hermes/ui-tui/src/__tests__/*.test.ts` and `.resources/hermes/ui-tui/packages/hermes-ink/src/ink/*.test.ts` — vitest test locations and naming convention.
- `.resources/hermes/tests/test_cli_skin_integration.py:1-120` — `_make_cli_stub()` builds a `HermesCLI` via `__new__` with hand-set attributes, then exercises skin-engine integration with `set_active_skin("ares")` and asserts on `_app.style` and `_invalidate(min_interval=0.0)`.
- `.resources/hermes/CONTRIBUTING.md:622-635` — commit-prefix table (`feat`, `fix`, `test`, etc.) — matches AGH's commit-style rule.
