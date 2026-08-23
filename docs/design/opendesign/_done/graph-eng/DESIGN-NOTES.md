# Graph engine — requests, strategies, time travel

Design contract for the surfaces in `.compozy/tasks/graph-eng/_uiux.md`,
delivered as ten boards in this folder. Companions: `_spec.md` (behavior
authority), `_user_stories.md` (ACs/ECs). This file is the locked semantic
contract — every chip, wait sentence, and decision button on the boards
traces back here.

## Locked decisions

### Requests (S1 · S4)

One exported request→tone/glyph map. Run-page surfaces and the waits rail
share it; the bell board consumes the same map with one re-ink (below).

- **pending human request = warning** on run-page surfaces. Danger is
  reserved for bell contexts per ADR-006 — warning means "parked on you",
  not "broken". Glyph `triangle-alert` + the word `pending`.
- **near-expiry = warning + countdown.** Same tone as pending; the clock
  glyph and a mono countdown (`expires 4m`) carry the extra urgency. Rail
  and card use the same remaining-time string.
- **expired request = danger.** Glyph `circle-alert` + `expired`. The
  card shows the route that was taken, never an answer form.
- **answered / amended provenance = info.** Glyph `check` / `pencil` +
  the word `answered` or `amended`. Actor + decision stay visible.
- **partial join = warning**, and the chip always contains the word
  `partial` plus coverage numbers (`2 of 3 lanes`). Never a bare yellow
  mark.
- **canceled-by-strategy / route-not-taken / never-materialized = neutral.**
  Absence is calm — glyph `minus` / `git-branch` / `circle-dashed` + the
  literal state word. Never alarming.
- **fork lineage = info.** Glyph `git-fork` + `fork`.
- Color is never the only channel (WCAG 1.4.1): every colored element
  pairs tone + glyph + literal state word.
- **Truthful UI.** A request renders only daemon-persisted fields:
  prompt, redacted context preview + "fetch full", expected shape,
  deadline. Redacted values render `••• redacted`, never blank. A
  request whose run has terminated shows the resolved outcome — never
  an answer form (US-007.EC-2).
- **Decisions vocabulary (exact):** `approve / edit / reject / respond`.
  Buttons render only the persisted `decisions` set. An absent decision
  is absent, not disabled.
- **Wait-kind sentences (exact):** ask node "is waiting for an answer";
  review node "is waiting for a decision on its proposed action".
- **Responders (US-005).** `confirm-rollout` permits operator `pedro` +
  agent `release-bot`. `apply-migration` is operator-only. A denied
  self-response is a deterministic micro-mono reason, not a toast.
- **No optimistic paints.** Submit disables the form and waits for the
  daemon ack. Keyboard-first: forms and decision bars are fully operable
  without a pointer.

### Strategies / progress

Locked for later boards; do not invent a second vocabulary.

- Fan-out over `services[]` → lanes `api`, `web`, `worker`.
- `best_effort` threshold 66%. `fail_fast` triggered by lane `worker`.
  `race` won by `api`.
- A lane canceled by strategy, a route not taken, or a node that never
  materialized stays **neutral** (see Requests). A partial join stays
  **warning** and always says `partial`.
- Route node `triage`: `hotfix` when `severity == "p0"`, `standard`
  when `severity == "p1"`, default `backlog`.

### Time travel

- Amend lands on node `render-notes` and wears the **info** `amended`
  chip (provenance, not a new request).
- Rerun starts from `apply-migration` on the same run identity.
- Fork from generation 2 produces run `run_9c1d` and wears the **info**
  `fork` chip. Lineage is a fact, not an alarm.

### Editor

- `edit` opens the editor pre-filled with the proposed args.
- `respond` opens an output form validated against the node's output
  shape. Neither verb is a hidden overflow item.
- Route/ask editor boards own the field chrome; this file owns the
  verbs and the schema-driven answer form (`{regions: string[], canary:
  boolean}` on `confirm-rollout`).
- **The staged `release-train` graph and any palette rows drawn on the
  editor boards are the set's illustrative story, never an inventory
  statement.** The production palette is additive: every existing node
  kind stays and `ask`/`route` join them (authority: `_uiux.md` S10 +
  `web/src/systems/loops/lib/loop-palette.ts` + the Go DSL). Never
  remove or hide a kind to match a board.

### Bell

- ADR-006: danger is reserved for bell contexts. The same pending
  request that is **warning** on the run page re-inks to **danger** in
  the bell (you are the blocker across workspaces). Do not leak that
  re-ink back onto run-page cards.
- Bell later adds workspace `payments` beside `compozy`. Run-page
  boards stay on `compozy`.

### Lab layout

Lab pages run full-viewport (authorized delta vs the herdr 960px scaffold — operator direction 2026-08-16). Staged page fragments render at production content width (fluid ≤1240px inside the stage); true component widths stay pixel-true.

## Shared data story

Workspace `compozy` (bell later adds `payments`). Loop `release-train`,
published v3. Run `run_7f3a`, generation 3. Fan-out over `services[]` →
lanes `api`, `web`, `worker`. Ask node `confirm-rollout`: prompt "Which
regions ship first?", expected shape `{regions: string[], canary:
boolean}`, expires in 24h. Review node `apply-migration`: proposed args
original vs edited (`--dry-run` flag removed in the edit), decisions
`approve / edit / reject / respond`. Responders: operator `pedro` +
agent `release-bot` (agents permitted on `confirm-rollout`, denied on
`apply-migration`). Route node `triage`: `hotfix` when `severity ==
"p0"`, `standard` when `severity == "p1"`, default `backlog`.
Strategies: `best_effort` threshold 66%; `fail_fast` triggered by lane
`worker`; `race` won by `api`. Amend on node `render-notes`. Rerun from
`apply-migration`. Fork from generation 2 → run `run_9c1d`.

## Files

Each board = final surface (§01) + states lab. `index.html` is the set hub.

| Board | Surfaces | Status |
| --- | --- | --- |
| `graph-eng-needs-you-requests.html` | S1 needs-you region + S4 parked / waits | delivered |
| `graph-eng-timeline-rows.html` | timeline request rows | delivered |
| `graph-eng-progress-strategies.html` | fan-out progress + strategies | delivered |
| `graph-eng-node-verbs.html` | node verb confirmations | delivered |
| `graph-eng-inspect-lineage.html` | inspect + fork lineage | delivered |
| `graph-eng-run-diff.html` | run diff | delivered |
| `graph-eng-fork-dialog.html` | fork dialog | delivered |
| `graph-eng-editor-route-ask.html` | route + ask editor | delivered |
| `graph-eng-editor-chrome.html` | editor chrome | delivered |
| `graph-eng-bell-requests.html` | bell request rows (danger re-ink) | delivered |

`graph-eng.css` holds chapters 1–12 (1–5 requests, 6–10 timeline through
fork, 11 editor chrome, 12 bell request row). Later runs append after
the marked append point — they never restyle earlier chapters.

Iterate on these files; don't regenerate. Implementation tasks cite the boards as visual contracts — artboard CSS is a contract, never a stylesheet to import.
