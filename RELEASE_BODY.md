## 0.6.2 - 2026-11-13

### Bug Fixes

- Recover orphaned automation runs and expose cancellation (#1613)
- Stop dropping mid-pass message promotions and race-scale flaky Go test waits (#1615)
- Restore v0.6.0 config upgrades and make daemon boot and restart bounded (#1621)
- Bound filtered logs stream polls by the ledger head (#1625)

### Features

- Optional OpenTelemetry trace and metric export (#1552)
- Make the update button prominent with an animated brand border (#1623)

### Performance Improvements

- Read a session's instance binding by key instead of a catalog list (#1632)

### Testing

- Wait for obsolete attention wake settlement

### Release Notes

#### Features

##### Export daemon traces and metrics with OpenTelemetry

CompozyOS can now export daemon traces and metrics to an OpenTelemetry collector over OTLP HTTP or gRPC. Export is off by default. Operators enable it through the global `observability.otel.*` settings, store authentication in environment or vault-backed header refs, and check export health with `compozy status`. Exported values are sanitized, and collector failures never fail a session turn. The repository includes a local Grafana, Tempo, Loki and Prometheus stack under `infra/observability/`.

Migration notes: provider processes no longer inherit `OTEL_*` or `TRACEPARENT` from the daemon environment, including when daemon export is off. If you previously used that inheritance for independent provider telemetry, move those variables into the agent's `session_env`. Alternatively, enable daemon export and `observability.otel.forward_to_providers` to share its collector settings. Existing local observability keeps working; the status API gains an optional `otel` object.

##### A more visible update button

When a new CompozyOS version is available, the Update button in the top bar now has an animated border in the brand gradient, so updates are easier to notice. The animation stops while the update installs, and the border stays static when reduced motion is on.

#### Fixes

##### Automation jobs recover after a restart

Automation jobs blocked as `self_overlap` by an orphaned run after a restart now recover automatically, preserving their run history and recording why the old run failed. Use `compozy automation runs cancel <run-id>` to cancel a stuck run whose session is missing or stopped. Live executions must be stopped through their owning session, Loop or Task first.

##### The desktop boot window names a config file error

When CompozyOS cannot load your `config.toml`, the desktop boot window now says so. It used to report that the daemon was "listening but unhealthy", retry five times, and end on a generic start failure, even when the running daemon was healthy. It now stops at once, names the file and the settings it does not recognize or the line of invalid TOML, and adds **Open config file** next to **Try again** and **Open logs**.

Migration notes: no action needed.

##### Restarts are faster and no longer look like a hang

Restarting CompozyOS, from the desktop app, `compozy daemon relaunch` or an update, used to take longer the more sessions you had ever created, and could stall for a minute or more with no sign of progress.

- **The app opens sooner.** The daemon serves your sessions, transcripts and status as soon as it is listening, then resumes interrupted sessions in the background, several at a time. The boot window names the step in progress, and sessions that are still coming back show as resuming.
- **Startup cost follows what changed.** Sessions that have not changed since the last start are no longer reopened, repaired and re-indexed on every boot.
- **Shutdown is bounded.** A restart no longer waits up to 30 seconds for turns that are saved and resumed anyway, no new provider process starts once shutdown begins, the database is always checkpointed, and the process is guaranteed to exit.
- **A bad update no longer leaves you without a daemon.** Before the running daemon stops, the replacement is checked against your config and pending database migrations. If it cannot start, the running daemon stays up and the restart reports why.
- **Interrupted sessions continue on their own more often.** A leftover infrastructure error from an earlier start, such as a tool connection timeout, no longer pauses a session with "restart continuation requires operator input". Real agent failures still wait for you.
- **An interrupted upgrade no longer blocks startup.** A session database that already had the `reported_cost_amount` column without its migration recorded now finishes that migration instead of failing with `duplicate column name`.

The desktop app also waits for a restart or update that is already running instead of failing with "runtime mutation in progress", and no longer gives up on a daemon that is still starting after 45 seconds.

Migration notes: no action needed. `compozy daemon status` and `daemon.json` gain a readiness phase (`starting`, `serving`, `ready`, `stopping`); `status: running` still means fully ready, so existing scripts keep working.

##### Idle filtered log streams use less daemon CPU

Keeping the desktop or web app open no longer makes a quiet, filtered log stream rescan retained event history every second. Live streams start at the physical ledger head and advance past events that do not match their filters, while full pages and reconnect cursors continue draining without skipping delayed events. No migration or operator action is required.

##### Always-on agents recover after the provider backend changes

An always-on agent no longer gets stuck when its provider account or backend changes, for example after you switch Codex `model_provider`. Before, the provider refused to reopen the agent's saved thread ("This chat uses openai, but your current provider is ..."), CompozyOS retried on every wake, the main session filled with identical load failures, and the agent's queue stopped draining.

CompozyOS now recognizes that refusal as permanent. On the next wake it starts a new provider thread under the same session, replays the stored transcript into it, and shows one notice that the provider backend changed. The session, its transcript, queued inputs and agent memory are kept, and the queue drains again. Ordinary sessions recover the same way. Other load failures still retry the saved thread.

For a main session that cannot resume for any other reason, `compozy agent instance clear-main <agent-or-instance-id>` and the operator-only `compozy__agent_instance_clear_main` tool start it over with a fresh conversation. They keep the instance, the session ID, queued inputs and agent memory, and discard the main's conversation.

Migration notes: no action needed.

##### Messages sent during a long turn are no longer delayed

A message from another session that arrived while a long-running turn was already checking its queue could wait for the next supervisor tick before being delivered. Those requests are now merged into the pass that is running, so the message reaches the agent right away.

##### CompozyOS starts again with settings v0.6.0 still accepted

Updating to v0.6.1 could leave the desktop app on "CompozyOS could not start". v0.6.1 treated several settings as errors that v0.6.0 still accepted, and the only way out was to edit files by hand.

CompozyOS now starts with those settings present and handles each one for you:

- **`permissions.provider_full_access` in `config.toml`.** The key stopped doing anything in v0.6.0. CompozyOS removes that line, changes nothing else, and keeps the original next to the file as `config.toml.retired-keys.bak`.
- **Retired memory and compaction settings in `config.toml`.** `[memory]`, the Dream, checkpoint summary, memory extractor and memory controller roles, `[session.compaction]`, `memory.consolidated` automation triggers and the `compaction_reason` / `compaction_strategy` hook matchers move to a commented block at the end of the same file. The values are kept and no longer apply.
- **The earlier always-on agent settings.** A config table or an `AGENT.md` that still uses the kind name from before v0.5.1 is updated to `always_on` in place.
- **`compaction_reason` and `compaction_strategy` in `AGENT.md` hooks.** The two keys are ignored and the file is left as it is.

This covers the global, profile and workspace files, including a workspace config you pull after updating. If a file cannot be rewritten, for example in a read-only folder, CompozyOS ignores the retired settings, starts anyway, and tries again on the next start.

An installed extension whose packaged resources no longer load is now skipped with a warning in the log. Your other extensions and CompozyOS itself start normally.

Migration notes: no action needed. `permissions.mode` still selects the provider access level, and `compaction_trigger` replaces the two retired hook matchers. CompozyOS stops accepting these settings in v0.7.0, so let it rewrite your files, or update the ones it could not rewrite, before then.

##### Session status uses less CPU and memory

CompozyOS now reads only the instance binding it needs when checking a session's status, reducing repeated database work and memory allocations in busy daemons with large session catalogs. Session status and access rules stay the same.
