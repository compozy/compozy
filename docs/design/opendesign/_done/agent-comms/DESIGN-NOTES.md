# Agent comms — visual contract

## Revision 2026-08-23 — legibility pass (recorded fixes)

Feedback: boards read dev/operator-first, rows too dense, too few views of
how calls actually appear inside a session. Locked responses:

- **New board `agent-comms-in-session.html`** — the primary meeting point.
  Inline call card `.ac-turn-call` rides the ds-core `.toolcall` geometry
  inside the `.transcript` grammar; variants for queued / running /
  completed (+extracted) / invalid-result / canceled, the child-side view
  (`.ac-turn-bound` / `.ac-turn-return`), an inline message
  (`.ac-turn-msg` + `.ac-untrusted`), and the settled-calls stack
  (`.ac-turn-stack`). CSS chapter 9.
- **Ids and digests live in the drill-in, not in flows.** Tree rows,
  bell rows, and catalog rows no longer render `call_`/`ses_`
  ULIDs or `sha256:` digests; the root header keeps its session id and the
  call detail keeps every record field verbatim. (Refines "machine truth
  demoted": demoted means one level deeper, not on every row.)
- **Plain words first, everywhere.** Lab notes and in-UI prose describe
  states in plain language ("the answer didn't match what was asked");
  the exact CLI state words render only inside `.ac-state`/`.ac-delivery`
  chips, and deterministic error codes stay as mono suffixes. Bell-row
  descriptions no longer repeat state words as prose.
- **Session panel stages a real conversation** — elided-transcript
  placeholder replaced with actual turns around the markers and the
  inline card.

### Revision 3, same day — the inbox board is cut

The standalone inbox surface (S3, `/agents/inbox`, `agent-comms-inbox.html`,
VC-14–16) was cut from the spec: a mailbox of receipts is not a place an
operator lives. The mailbox stays a runtime channel (receipts, brakes,
provenance, CLI/API views); messages render in context — in-session
transcript turns (chapter 9, now VC-14) and the call-detail message-child
compose. VC-15/16 are withdrawn, never reused; surface ids stay stable.

### Revision 2, same day — collapsed by default + fan-out

Feedback: the inline card showed the ask and the answer open by default
(visual noise), and nothing staged a large parallel fan-out. Locked:

- **`.ac-turn-call` is a `details/summary` disclosure, closed by
  default.** The resting card is its 34px head row alone — caret ·
  agent · state chip · liveness dots · age. The ask and the result
  strip render only on expand. A closed card still informs: the chip
  grammar carries state, the bell carries urgency. Boards stage one
  exemplar `open` per section to show the inside.
- **`.ac-turn-fan` — many calls at once are one card.** A parallel
  fan-out renders one closed card: overlapping identity chips (3 +
  `+N`), "Asked N helpers", live tallies ("9 done · 2 running · 1
  needs a look"), and the worst state escalated to the head as a
  normal state chip (childSessionSignalTone rule, no new dictionary).
  Expanded: compact `.ac-call-li` rows, capped, footed by the Calls
  panel link — the panel owns the full list. The transcript never
  renders N sibling cards for one fan-out.

Companions: `.compozy/tasks/agent-comms/_uiux.md` (surface map S1–S7, S3
retired), `_user_stories.md` (states come from ACs/ECs), `_dx.md`
(vocabulary, CLI transcripts, error codes), `task_06.md` (VC-01–22 with
VC-15/16 withdrawn). This file is the locked semantic contract for the set;
the boards illustrate it, the implementation tasks cite it.

## Locked decisions

### Nine call states, one chip grammar

A call is always in exactly one of nine states (`_dx.md`): `queued`, `running`,
`completed`, `invalid-result`, `completed-without-result`, `failed`,
`canceled`, `timeout`, `expired`. The `.ac-state` chip renders tone + glyph +
the literal state word in mono — the word is the exact CLI vocabulary and never
leaves the chip. No tenth state, no synonyms ("pending", "done", "error" are
banned as state names).

### queued and running are neutral — liveness is motion

The session dictionary pins `running` to accent with a pulse
(`session-badge.ts`); a delegation tree can hold dozens of running calls, so
accent there would blow the accent budget into a control-room wash. The
component plan already resolves this: call `running` = `StatusDot` neutral +
`TypingDots` on the active row. Locked: queued = hollow neutral (`clock`),
running = neutral (`circle`) + typing dots. Color never announces liveness;
motion does, and reduced-motion holds the dots at steady 70% opacity.

### Terminal tones

- `completed` → success (`check`). Verdict provenance rides beside it as a
  neutral mono kind word (`.ac-verdict`): `returned`, `extracted`, `repaired`.
  `extracted` NEVER renders as `returned` — provenance is admission truth.
- `invalid-result` (`file-x`), `failed` (`x`), `expired` (`hourglass`),
  `completed-without-result` (`circle-slash`) → danger. All four mean
  "expectations unmet"; the needs-you class shares one tone and differs by
  glyph (the `session-badge.ts` rule — color is never the only channel).
- `canceled` (`ban`), `timeout` (`timer-off`) → warning. Both are deliberate
  outcomes, not faults.

### Child-session states — exactly three

`running` (neutral, typing dots), `parked` (neutral + `moon` — still
addressable, TTL counting), `gone` (hollow, `circle-off` — identity kept,
affordances absent). There is **no Revive control anywhere**: revival IS
calling or messaging a parked child, so the affordances are call-again and
message. A gone target's affordances are absent, never grayed.

### Delivery receipts — the public four

`delivered-into-turn` / `woke` → success (a receipt is a confirmation),
`queued` → neutral, `failed` → danger with the typed reason on the row.
Mono words, `.ac-delivery` chip. Internal states never surface. **No
read/seen state exists in the runtime, so no unread mark and no mark-read
control render — anywhere.**

### Attention — three causes, two sections, no dismissal

Needs-you call causes are exactly: `invalid-result`,
`completed-without-result`, child blocked on a decision. They join the bell's
needs-you section in danger roundels (18 px, glyph identity); completed calls
awaiting a look sit in Finished with info tone, never counted as needs-you.
Signal storms coalesce per tree into one row with the real count. Rows clear
only when their cause resolves — no dismiss, snooze, or clear-all. No
budget-exhausted attention state exists (completions are never
admission-denied, ADR-011). Counts come from the daemon summary projection;
stale sources contribute zero but rows stay clickable (`attention-model.ts`).

### Truthful numbers

Cost renders only through `describeCost()` — `≈` prefix when estimated,
"Included", "Unavailable"; absent provider data never renders `0`. List totals
come from daemon projections, never from loaded pages ("25 of 247 loaded").
Zero instance counts render nothing (no "0 running" pill). `result_bytes`,
`depth`, `repair_attempts` are record fields, shown verbatim.

### Deadlines and TTL

There is no default deadline: most calls carry `deadline_at: null` and **no
timer chrome at all**. A timeout call shows its opt-in deadline in the header.
The idle-TTL fact states its own physics: "suspended while running", "expires
in 41m" while parked.

### Untrusted text is framed, stamped, inert

Message bodies and agent descriptions render inside `.ac-untrusted` — dashed
hairline, provenance stamp ("from agent reviewer — not the operator"),
embedded commands inert. Errors and validator output are quoted verbatim
(already sanitized upstream: secret-shaped values hash-redacted before
validation).

### Error copy = plain words first, code in mono

Every failure surfaces the deterministic `_dx.md` code
(`call_expect_invalid`, `call_target_expired`, `message_target_blocked`,
`message_rate_limited`, `message_duplicate`, …) as a mono suffix after a
plain-language first line that names the recovery. The wake/toast copy is the
`_dx.md` wake shape verbatim — the toast never rephrases the wake.

### Accent budget

One primary action per composed screen: the compose's Send / Call button.
Rows, chips, tabs, and selection stay neutral (fg underline for tabs,
elevated plate for selection). The dock `dk-badge` keeps its production
accent — it is the shell's one attention marker, unchanged by this set.

### Lab scaffold ownership (recorded fix)

Boards used to link `../graph-eng/graph-eng.css` for the lab scaffold
(`.shell .main .w2head .scroll .page .lab-sec .spec .stage`). graph-eng was
retired to `_done/` on 2026-08-23, so this set carries its own copy of the
scaffold chapter inside `agent-comms.css` — boards link only
`ds-core.css → ds-shell.css → agent-comms.css`. Never link into `_done/`.

### Lab layout

Full-viewport lab pages: 44 px `w2head`, fluid `.page` (24/36/80 padding,
760 px single-column breakpoint). `.stage` is the wallpapered frame (400 px
base, `--short` 260, `--tall` 520); staged windows run at production content
width (fluid ≤1240), the inspector rail is pixel-true 320 px.

### CSS

`agent-comms.css` is the set's single domain stylesheet, prefix `.ac-*`,
linked after ds-core/ds-shell. It owns: the lab scaffold copy, the call-state
/ child-state / verdict / delivery dictionaries, tree rows, call-detail
blocks, compose shells, roster scope chips + contract editor, panel call
rows + markers + wake card, bell rows. Boards contain no `<style>` block and
no inline `style` except small layout shims on staged fragments. Artboard CSS
is a contract — never imported into production.

### Primitives — reuse before create

From `@compozy/ui` (production mapping in `_uiux.md` component plan):
`Tree/TreeItem/TreeItemLabel` (first real consumer), `Pill`, `PillDot`,
`MonoId`, `KindChip`, `StatusDot`, `TypingDots`, `OwnerAvatar`, `Time`,
`ListingRow`, `MetadataList`, `PropertyRow`, `Timeline/TimelineEvent`,
`JsonViewer`, `CodeBlock`, `CopyIconButton`, `Metric`, `ActionResultBanner`,
`DataSurface*`, `Empty`, `Marker`/`MarkerMeta`, Sonner toasts. No new
`@compozy/ui` primitive is planned by this set.

| Board element | Class used | Owner |
| --- | --- | --- |
| state/verdict/delivery chips | `.ac-state` `.ac-verdict` `.ac-delivery` | this set → `Pill`/`KindChip` tones |
| tree rows + guides | `.ac-tree` `.ac-row` | this set → `Tree*` + projection module |
| roster rows | `.listing-row` family | ds-core |
| property rows / rail | `.prow` `.railbox` `.rail-sec` | ds-core |
| tabs | `.tabbar` `.tab` | ds-core |
| banners | `.notice` (+ tone) | ds-core |
| empty states | `.empty` family | ds-core |
| bell shell / toasts / dock | `.popover` `.toast` `.dock-*` | ds-shell |
| buttons / inputs | `.btn` `.input` `.textarea` | ds-core |

### No seventh board

S7 (dock badge) is one section on the attention board: the change is a badge
union widening on an existing tile — there is no new chrome to design. The
operator Call path (S4) and the child-side context (S5) live as sections on
their owning boards for the same reason.

## Canonical data story

Workspace `ws_main`. Definitions: `reviewer` (workspace, "Reviews a diff and
returns structured findings with severity.", `sha256:2b8e…`), `scout` (global,
"Maps a codebase area and returns entry points.", `sha256:91aa…`),
`docs-writer` (workspace shadowing a global twin), `triage` (no description).
Root session `loop-retry-fix` = `ses_01JBD7ZZAAAA`. Canonical call
`call_01JBD8G2K7Q9` (reviewer, child `ses_01JBD8G2MZTX`, completed/returned,
312 B, contract `sha256:9f2c…`, result `{"verdict":"needs-changes"}`).
Follow-up `call_01JBD8H9PW2M` (running → canceled in later sections), scout
call `call_01JBD8J1XKCV` / child `ses_01JBD8J1ZQ8F` (parked), messages
`msg_01JBD8M2R4V7` (operator → child, delivered-into-turn) and
`msg_01JBD8KX9QQ1` (child → parent, woke). Config numbers quoted from
`_dx.md` defaults: depth 3, TTL 1h, budget 256 KiB/4 MiB, messages 30/min,
dedup 30s, pending cap 50, 64 KiB max.

## Staging that is not a VC

Before/after pairs (queued→woke, needs-you→finished auto-resolve) stage two
moments of one row side by side — production animates in place. Compose flow
states (editing / invalid / submitting / accepted) are staged as parallel
fragments of one stateful form; the accent budget applies per composed screen,
not per lab sheet. The 150-call VC-05 fixture is represented by its collapsed
summary rows; virtualization thresholds are an authorized difference
(task_06).

## Chapter map (agent-comms.css)

0 header + domain tokens · lab scaffold (copied from retired graph-eng) ·
1 call-state dictionary (+ child, verdict, delivery, typing, legend) ·
2 activity tree · 3 call detail (head, blocks, untrusted, timeline, attempts,
result, superseded) · 4 compose (shared shell — inbox rows retired with the
S3 cut, 2026-08-23) · 5 roster (scope, contract editor) · 6 session calls
panel (list rows, markers, wake) · 7 attention (bell rows, sections) ·
8 shared small parts (cost, TTL) ·
9 in-session (turn call disclosure, fan-out, turn message, bound/return, stack).
Append new chapters at the end; never renumber.

## Board budget

| Board | Sections | Note |
| --- | --- | --- |
| agent-comms-in-session.html | 7 | revision 2026-08-23; transcript variants + fan-out; VC-14 (inline message turn) |
| agent-comms-activity-tree.html | 7 | VC-01–07 |
| agent-comms-call-detail.html | 6 | VC-08–13 |
| agent-comms-roster.html | 4 | VC-17–19 |
| agent-comms-session-calls-panel.html | 4 | VC-20–21 |
| agent-comms-attention.html | 5 | VC-22 |
