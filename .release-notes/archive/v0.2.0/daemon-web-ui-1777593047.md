---
title: Daemon Web UI
type: highlight
---

Compozy now ships a built-in web UI served straight from the daemon. Start the daemon and you get a single-binary, localhost-only dashboard for browsing workspaces, workflows, tasks, runs, reviews, and memory — with live SSE-backed run streaming, raw event diagnostics, a run transcript viewer, and skeletons / empty-states throughout. Frontend assets are embedded in the Go binary, so there is nothing extra to install. Contributors can point the daemon at a Vite dev server with `--web-dev-proxy`.

### What's in the UI

- **Dashboard** with workspace KPIs and a "Sync all workflows" action.
- **Workflows** inventory, per-workflow task board, and task detail page.
- **Workflow Spec** viewer (PRD / TechSpec / ADR markdown rendered inline).
- **Memory** index plus per-workflow memory view.
- **Reviews** index, per-round view, and issue detail pages.
- **Runs** list with workflow filter, plus run detail with live event stream.
- **Run Event Feed** — raw daemon events with in-memory event store, SSE snapshots, heartbeat, and overflow framing.
- **Run Transcript Panel** — full transcript view of agent turns and tool calls for any run.
- **Workspace picker** with onboarding shell and live workspace WebSocket sync.
- New shared UI primitives: `Alert`, `EmptyState`, `Markdown`, `Metric`, `Skeleton`, `StatusBadge`, plus button loading state and token refresh.

### Getting started

```bash
# Start the daemon (foreground for visibility); the UI is served at the daemon HTTP port
compozy daemon start --foreground

# Discover the URL
compozy daemon status        # prints "http_port: <N>"
open "http://127.0.0.1:<N>"

# UI contributors: proxy the daemon to a Vite dev server
compozy daemon start --foreground --web-dev-proxy http://127.0.0.1:3000
```

### Defaults & overrides

| Setting            | Default                     | Override                                           |
| ------------------ | --------------------------- | -------------------------------------------------- |
| Bind host          | `127.0.0.1` (loopback only) | hard-coded; non-loopback binds are rejected        |
| HTTP port          | OS-chosen (ephemeral)       | `COMPOZY_DAEMON_HTTP_PORT=<n>`                     |
| Frontend dev proxy | off (embedded `web/dist`)   | `--web-dev-proxy <url>` or `COMPOZY_WEB_DEV_PROXY` |

```bash
# Pin the daemon UI to a known port
export COMPOZY_DAEMON_HTTP_PORT=4444
export COMPOZY_WEB_DEV_PROXY=http://127.0.0.1:3000   # only for UI dev
compozy daemon start
```

### Security model

There is no login — the UI is loopback-only and the API enforces:

- Host header must match localhost; non-`127.0.0.1` binds are rejected.
- Origin validation against the bound host.
- Per-session CSRF cookie + header.
- `X-Compozy-Active-Workspace` header propagated by the SPA.
- Standard hardening headers via `securityHeadersMiddleware`, plus ETag/304 caching for the embedded static assets.
