---
title: Always-on agents check external sources on each check-in
type: feature
---

An always-on agent can now watch something outside CompozyOS, such as new GitHub issues, without paying for a model turn every time it looks. Declare **check-in sources** in `HEARTBEAT.md` under `context.sources`. On each check-in CompozyOS asks every source what is new, without a model, and starts the read-only check only when a source or a built-in fact returned something new. An empty or unchanged result never starts a model turn.

```yaml
context:
  sources:
    - id: new-issues
      kind: forge-github/issues
      with:
        repo: compozy/compozy
        labels: [bug]
```

- **Two kinds of source.** Extensions provide source kinds through the new `heartbeat.source` provide surface, and the bundled `forge-github` extension ships `forge-github/issues`. You can also put your own scripts in the agent's `sources/` folder.
- **Scripts run only after you approve them.** CompozyOS runs a script only when its exact hash matches your approval, and editing the script needs a new approval. Approve with `compozy agent heartbeat sources approve <agent> <script>`, revoke with `compozy agent heartbeat sources revoke`, or use the Check-ins page. Agents can't approve scripts.
- **Nothing is missed.** A source's position advances only after a check has seen its items, so a failed or interrupted check sees the same items again. A source that has more items wakes the agent again at `agents.heartbeat.min_interval`.
- **Failures are visible.** Each source reports a typed error and backs off on its own. After `failure_threshold` failures in a row, the check-in report says so.
- **Where to find it:** `compozy agent heartbeat sources <agent>`, `sources` on `compozy agent instance show` and `check --dry-run`, the native tool `compozy__agent_heartbeat_sources`, the Check-ins page (add, edit, remove and approve sources), and Go/TS SDK helpers with a `heartbeat-source-go` scaffold. The new guide "Watch new GitHub issues" walks through a setup.
- **Config:** `[agents.heartbeat.sources]` sets `scripts_enabled`, `timeout`, `max_concurrent_polls`, `max_per_agent`, `max_items`, `max_output_bytes`, `max_state_bytes`, `failure_threshold` and `env_allow`. It is daemon-wide and needs a restart.

Agents without sources work as before.

### Migration notes

- The daemon applies two new database migrations at startup. No existing row or file changes.
- Older daemons reject `context.sources`. Upgrade every daemon that reads a `HEARTBEAT.md` before you add sources to it.
