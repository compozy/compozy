---
title: One Automations window for schedules, events, and links
type: feature
---

The Web UI's separate Jobs and Triggers windows are now one **Automations** window. Every automation reads as one sentence — how it starts, optional conditions, what it does — with its last-run result and an On/Off switch on the row, so a failed run is visible without opening anything. One detail page and one "New automation" dialog serve schedules, events, and links; the start you pick decides whether CompozyOS stores a job or a trigger.

- `last_run` (`id`, `status`, `started_at`, `ended_at`, `skip_reason`) is on every job and trigger from `compozy automation jobs|triggers`, `GET /api/automation/{jobs,triggers}` (HTTP and UDS), and `compozy__automation_{jobs,triggers}_{list,get}`. The CLI tables gain a **Last run** column; `-o toon` adds `last_run_status` and `last_run_started_at`.
- The list commands, routes, and tools accept `target=agent|loop|task` (`--target` on the CLI).
- The dock has one Automations launcher; the command palette gains "New scheduled automation" and "New automation on an event"; Loop pages replace "Add trigger" and "Add schedule" with **Automate ▾**; Settings → Automation uses plainer labels and writes the same `config.toml` keys.
- CLI verbs, HTTP/UDS routes, tool ids, and `[[automation.jobs]]` / `[[automation.triggers]]` are unchanged.

```bash
compozy automation jobs --workspace checkout-api -o json | jq '.jobs[] | {name, last_run}'
compozy automation triggers --workspace checkout-api --target loop
```

Migration notes:

- Saved desktops migrate permanently (window-manager snapshot v5): every Jobs or Triggers window, tab, and recently closed entry reopens as an Automations window on the matching `/automations/…` path, in the same place. No window is closed.
- Command palette pins, recents, and usage for the Jobs and Triggers commands merge into "Open Automations" and the Automations view; usage counts add up and no pin is dropped.
- Old web links `/jobs`, `/jobs/<id>`, `/triggers`, and `/triggers/<id>` redirect to `/automations`, `/automations/jobs/<id>`, and `/automations/triggers/<id>`, carrying their filters. The redirects are removed in v0.5.0.
- The app ids `jobs` and `triggers` (`compozy window open --app`, window commands, `window_layout` resources) and the palette ids `app.open.jobs`, `app.open.triggers`, `palette.view.jobs`, and `palette.view.triggers` are accepted as `automations` with a deprecation warning (CLI stderr; WARN logs `windowmanager.app_id_deprecated` and `cmdpalette.command_id_deprecated`). They are removed in v0.5.0 and then fail like any unknown id. Use `automations`, `app.open.automations`, and `palette.view.automations`.
- Site docs: the Automation section is now **Automations**, with two tutorials. `/docs/automation/jobs` and `/docs/automation/triggers` moved to `/docs/automation/schedules` and `/docs/automation/events` and redirect permanently.
