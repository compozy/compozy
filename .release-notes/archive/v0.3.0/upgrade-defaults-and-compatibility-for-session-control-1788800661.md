---
title: Upgrade defaults and compatibility for session control
type: highlight
---

Persisted database state and layouts upgrade through their owning migrations. Review these session defaults when upgrading:

- `session.busy_input.default_mode` is now `steer`; choose `queue` to defer follow-ups. Live injection depends on runtime capability, and unsupported runtimes report the interrupt fallback.
- New supervision settings are `session.supervision.quiet_after` (30m) and `stop_grace` (10m). Existing `inactivity_warning_after` and `inactivity_timeout` settings retain their legacy timer behavior during the v0.4 compatibility window. For ordinary positive thresholds, migrate the warning threshold to `quiet_after` and the difference between stop and warning thresholds to `stop_grace`; check the documented zero semantics before changing disabled timers.
- `network.live.max_total_wall_time` defaults to `0`, disabling that aggregate wall budget. Set an explicit duration if you need a limit.
- Prefer `--expected-turn` over the deprecated `--expected-turn-id`. The old flag and the historical `interrupt` default remain accepted with warnings until their planned removal in v0.5.0; explicit per-send interruption remains supported.
- `compozy__session_prompt` defaults to `wait: false`. Request `wait: true` explicitly when you need a synchronous native-tool call.

PRs: [#525](https://github.com/compozy/compozy/pull/525), [#555](https://github.com/compozy/compozy/pull/555), [#557](https://github.com/compozy/compozy/pull/557).
