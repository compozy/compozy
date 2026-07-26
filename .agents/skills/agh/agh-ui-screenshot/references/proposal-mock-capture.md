# Proposal-mock capture (URL-driven React mocks)

Design audits may need screenshots from `docs/design/new-proposal/agh-refined-7.html` — a single-file React app that does not use a router. Static screenshots of the file at `/` always show the default view; alternate views (kanban, dashboard, inbox, detail, new-task editor, empty state) need a different boot path.

## Pattern: clone-and-patch wrapper

1. Boot a static HTTP server in the mock's directory:
   ```
   python3 -m http.server 8765
   ```
   (Run from `docs/design/new-proposal/`; the proposal's relative imports require this exact CWD.)

2. Copy the canonical mock to a `_capture.html` sibling — never edit the canonical file in place:
   ```
   cp docs/design/new-proposal/agh-refined-7.html docs/design/new-proposal/_proposal-capture.html
   ```

3. Patch the copy to read view state from URL `?view=…&task=…&new=…&empty=…&tpl=…&accent=…` query params. The proposal app uses `useState` for `view / taskId / showNew / showEmpty / newTpl / accent`. Replace each `useState(<default>)` call with `useState(_initFromQuery)` where `_initFromQuery` is a small `URLSearchParams` reader injected just before the `App` function.

4. Inject `.tweaks { display: none !important }` into the patched copy's `<style>` block so the dev panel does not leak into screenshots.

5. Capture via `scripts/cap.mjs` against `http://localhost:8765/_proposal-capture.html?view=<view>` etc.

## Recommended view matrix

| Capture name | Query string | Notes |
| --- | --- | --- |
| `proposal-list-1440` | `?view=list` | Default state. |
| `proposal-kanban-1440` | `?view=kanban` | Four-column board. |
| `proposal-dashboard-1440` | `?view=dashboard` | KPI grid + queue health. |
| `proposal-inbox-1440` | `?view=inbox` | Lane tabs + group cards. |
| `proposal-empty-1440` | `?view=list&empty=1` | Empty-state template grid. |
| `proposal-new-oneshot-1440` | `?view=list&new=1&tpl=one_shot` | New-task modal, one-shot template. |
| `proposal-new-recurring-1440` | `?view=list&new=1&tpl=recurring` | New-task modal, recurring template. |
| `proposal-detail-1440` | `?task=tsk-7f3a26` | Running task detail. |
| `proposal-detail-blocked-1440` | `?task=tsk-6b1c44` | Blocked task detail (block-reason banner). |

## Cleanup

The patched copy is throwaway; delete it after the audit:

```
rm docs/design/new-proposal/_proposal-capture.html
```

Do not commit the patched copy. The canonical `agh-refined-7.html` is the only file that lives in version control.
