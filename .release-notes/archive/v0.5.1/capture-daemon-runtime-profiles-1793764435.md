---
title: Capture runtime profiles from a running daemon
type: feature
---

When the daemon is slow or busy, you can now capture a Go runtime profile from it without restarting. Run `compozy daemon profile <kind> --file <path>`, where `<kind>` is `cpu`, `heap`, `allocs`, `goroutine`, `mutex`, `block` or `threadcreate`.

```bash
compozy daemon profile cpu --seconds 30 --file cpu.pprof
go tool pprof -tagfocus session_id=<session-id> cpu.pprof
```

- CPU, mutex and block captures sample for 1 to 120 seconds (30 by default). Mutex and block sampling is on only during the capture. Only one CPU capture runs at a time; another returns `profile_busy`.
- Profiles are labeled by session, provider, tool and route, so you can narrow one to a single session. They don't include prompts, file contents, tool arguments or query strings.
- The file is written with owner-only permissions, and an existing file is kept if the capture fails.

This command is experimental. It works only over the local daemon socket: the `GET /api/daemon/profile/{kind}` route isn't served over HTTP or the Gateway, and app tokens can't use it.
