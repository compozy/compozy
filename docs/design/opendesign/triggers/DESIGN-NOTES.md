# Triggers — design notes

Event-driven automation. A trigger watches a runtime event and, when it matches, runs an **agent** or starts a **Loop**. Time-based runs belong to Jobs.

## Shared story

Workspace `checkout-api` (`ws_checkout_api`). Three entities reused across every artboard:

| Id | Event | Then | Source | Scope |
| --- | --- | --- | --- | --- |
| `summarize-failures` | `session.stopped` · `data.stop_reason=error` | agent `summarizer` + prompt | dynamic | workspace |
| `rerun-delivery` | `session.stopped` · `data.stop_reason=error` | loop `software-delivery` + mapping | dynamic | workspace |
| `deploy-webhook` | `webhook` · `data.action=deploy` AND `data.branch=main` | agent `deployer` + prompt | dynamic (config lock is a lab of the same id) | workspace |

Loop runs cross-link to `loops/` (`software-delivery`, `r-8f3a2b`).

## Job of the detail page

See what this trigger watches, whether it is on, and whether the last fire worked — then enable, edit, or inspect. Not a scheduler. Not a job inspector with the cron stripped out.

## Anatomy (finals)

```
44px drill-in head    Triggers / {id} · Edit + ···   (Edit only if source=dynamic)
(no 38px strip)
page head             Sentence · Enable switch opposite it
subhead               Event pill · workspace · last ran · updated
main                  When / If / Then  ·  Recent runs
rail                  Properties · (Public delivery if webhook) · Reliability · Identity
                      Inspect under the cards
sheet                 Diagnostics · Envelope
```

Then splits on `target_kind`:

- **agent** — chip + prompt (`{{ .Data.* }}` Go template)
- **loop** — workflow well + loop name (link) + mapping rows (`←` event / `=` static). No prompt. No LOOP accent pill.

When splits on `event`:

- observer / hook / ext / memory — sentence + mono event pill
- **webhook** — plus local `POST /api/webhooks/{global|workspaces/…}/{slug}--{wbh_}` path. Public reachability is a separate rail card.

## Gating (daemon truth)

| Control | Gate |
| --- | --- |
| Enable switch | `PATCH /api/automation/triggers/{id}` `{ enabled }` — all sources |
| Edit / Delete | Dynamic source only. Config/package: enable-only |
| Recent runs | `GET .../runs?limit=10` |
| Open session | Run row `session_id` |
| Open loop run | Run row `loop_run_id` (status `delegated`) |
| Run now | **Does not exist** |
| Schedule / next run | Jobs only |
| Signing secret | `webhook_secret_present` — never the value |
| Target kind | Immutable after create |

## Run statuses

`scheduled` · `running` · `delegated` · `completed` · `failed` · `canceled`

Tones: running/delegated info · scheduled warning · completed success · failed danger · canceled neutral.

## Authorized deltas vs production

- Enable is a labeled switch in the page head, opposite the sentence.
- Edit is a ghost in the head (dynamic only), not overflow-only.
- Lucide only. Rail is separate `.rsec` cards, not one `.railbox`.
- Source reads “You created this”, not a `DYNAMIC` pill. Config/package use the dashed lockbar + production sentences.
- Filter shows prose (`stop reason is error`); the `data.stop_reason` path stays a quiet sub line.
- Loop Then is labeled rows, not a JSON dump. No `LOOP` accent pill.
- Webhook: local path in When; public ingress in the rail. Workspace webhooks are in-scope (daemon-real).
- Inspect holds `DYNAMIC` / `WORKSPACE` / envelope JSON / fire-limit internals.

## Files

**Finals** — `trigger-detail.html` (agent) · `trigger-detail-loop.html` · `trigger-detail-webhook.html` · listing empty · create dialog.

**Labs** — `trigger-vocabulary.html` (When/If/Then/runs) · `trigger-detail-states.html` (disabled, pending, lock, empty, errors, 503, mismatch, ingress).

**Shared** — `triggers.css` · `triggers.js` · this file.

## Invented if shown

Run now · task target · trigger suggestions · secret value · job scheduler/next run · changing target after create · `{{event.*}}` templates · `/hooks/…` paths · Portuguese UI labels.
