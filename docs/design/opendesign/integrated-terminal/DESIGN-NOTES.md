# Integrated terminal — design notes (locked contract)

Visual contracts for `.compozy/tasks/integrated-terminal/_uiux.md` (surfaces S1–S8).
Delivered 2026-08-23. **Iterate these boards — never regenerate.**

Authority chain (highest first): `packages/ui/src/tokens.css` (post-#440; ds-core is
production parity per `design-system/PARITY.md`) → `_uiux.md` + `_user_stories.md` +
`_spec.md` → `design-system/ds-core.css` + `ds-shell.css` → `integrated-terminal.css`
(chapters 31–38; lab scaffold vendored — boards never link `_done/`).

## Board budget

| Board | Surfaces | Sections |
| --- | --- | --- |
| `integrated-terminal-app-window.html` | S1 window + S2 dock | 8 |
| `integrated-terminal-states.html` | S1 states | 10 |
| `integrated-terminal-session-blocks.html` | S3 · S4 · S8 | 7 |
| `integrated-terminal-input-request.html` | S5 | 6 |
| `integrated-terminal-approvals.html` | S6 | 6 |
| `integrated-terminal-journal.html` | S7 | 6 |

S9 (Settings) ships no artboard by spec — it follows the existing settings pattern.

## Signal map (locked — from `_uiux.md`, final call exercised here)

| State | Treatment |
| --- | --- |
| Watching / presence | info `--info` chip + **hollow cursor** — never prose |
| Take control | the **single accent** action in terminal chrome (`.btn--primary`) |
| Input requested · approx journal row | warning `--warning` glyph/chip; card ground stays neutral |
| Kill · irreversible approval | danger `--danger` word + glyph + `tm-cmd--danger` well |
| Exited 0 | success pill; **non-zero = neutral pill + code**, never danger |
| Reported by agent | neutral label chip (`.tm-reported`) — deliberately outside the signal palette |
| Recording | neutral chip with a pulsing `--danger` dot (see authorized deltas) |

Accent budget per screen: empty-state CTA **or** Take control — never both visible at
once (an empty terminal has nothing to take control of). The watcher's one-gesture
"Take control & deliver" in the request card is **neutral**; the header keeps the accent.

## ANSI token proposal (chapter 32 of `integrated-terminal.css`)

`tokens.css` carries no ANSI ramp (verified in `_uiux.md`). This set proposes
`--term-bg #131211` (one step below `--canvas` — the byte well is the only solid
sub-canvas ground in the product), `--term-fg #e3e1de`, `--term-cursor #f7f6f4`,
`--term-selection rgba(255,255,255,.16)`, and `--term-ansi-0..15` where 1/9, 2/10, 3/11
derive from the danger/success/warning lanes and 4/5/6 (+bright) are new desaturated
hues at matched OKLCH lightness ≈0.68–0.72 so no ANSI color outshouts the signal
palette. ANSI 8 is dim-by-design (≈3.5:1 — terminal convention, not UI text).
**These land in `packages/ui/src/tokens.css` via the token pipeline** (`make codegen`
regenerates `DESIGN.md`; `codegen-check` gates drift); the emulator theme object reads
the computed tokens. The chapter-32 block is the only raw-literal region in the set.

## Anatomy decisions

- **Window**: deck (37px, tabs = the project's terminals, traffic lights inside) →
  per-tab 44px head (identity once: glyph · title · `term-` MonoId; lease chip;
  viewers; ≤2 trailing actions) → full-bleed `tm-surface`. Exit bar and request card
  pin under the grid, inside the surface.
- **Cursor semantics**: solid block = you can type; hollow = watching; pipe mode has
  no cursor at all. The cursor is the read-only explanation — no helper prose.
- **Pipe mode**: log glyph on the tab, numbered gutter, verbs wait/signal/close only.
  Interactive affordances are **absent, not disabled** (also: execute-only platforms).
- **Approvals**: attention-row anatomy (`.att`) + `tm-cmd` well showing the exact
  command unmodified + cwd. Irreversible set: danger marking, no "always allow".
  Grants list/revoke in the existing tool-approvals surface — no new admin surface.
- **Journal**: the journal is a **table** by the ds-core definition (fixed columns,
  values compared across rows) and rides the canonical ladder from
  `design-system/components.html` §09 — the command cell is the title / sub /
  mono-id stack, outcome and confidence are `.pill`, every other cell is demoted
  text, and `table-layout:fixed` gives the command column all remaining width so
  the panel never scrolls sideways. It lives as a **pinned deck tab** in the
  Terminal app (S7 places it inside the app, not in a window of its own).
  Estimated detection is the only warning at rest. Cursor paging with **no
  total** (server never reports one) — the footer states rows *loaded*. Picking a
  row opens the DetailInspector rail (`.jr-detail`), which is where the full
  command, the exact times and the last output live.
- **Quote**: canonical `terminal_context` shape — id + line range in the head,
  numbered lines, trim caveat stated on the surface.
- **Secrets**: redacted input renders as dots in the entry field and as a
  length-only marker (`tm-redact-marker`) everywhere else — stream, journal, replay.

## Data story (exactly these fixtures — do not mint more)

Project **atlas-api**. Terminals: `term-4f21c9a03b7e` *dev server* (pty) ·
`term-9cd7e14b2a66` *psql* (pty) · `term-a03b558d21f0` *make gate* (pipe, exec-born) ·
`term-1e8f7a55c402` *ssh staging* (exited) · `term-77c1d0e94ab3` *e2e suite* (S3 only).
Actors: agent **Claude Code** (chip `CC`), humans **Pedro** (`PN`) and **Marina**
(the displacement/size-vote counterpart). Recording `rec-9f21ac`. Times cluster
12:20–12:59.

## Authorized deltas (every divergence; an un-annotated one is a defect)

1. `.a1..a15` spans stand in for emulator cell attributes — production paints via the
   `TerminalView` theme object, not CSS classes.
2. The recording dot reuses `--danger` as the universal record convention; the chip
   stays neutral and the state is also carried by the word `rec` + elapsed time.
3. Watcher one-gesture answer renders neutral (accent budget — see Signal map).
4. Filtered-empty counts only loaded rows ("in the 50 loaded rows") because the
   server never reports a total.
5. Actor chips render as plain `.agent-chip` initials beside the actor's name;
   production binds `colorsFor(ownerKind, ownerId)` from the owner palette.
6. The journal's working directory renders as the **demoted sub-line of the
   command cell** instead of its own column. The seven facts and their reading
   order survive, but the web row is *scanned*, not parsed, so it gets one title
   and six demoted attributes; the CLI keeps its column order (`_dx.md`).
7. Opening the journal detail rail **drops the permission and confidence
   columns**; they reappear as rows inside the rail rather than fighting the
   command for width (production `DetailInspector` inline breakpoint).

## Design → production mapping (from `_uiux.md` component plan)

`TerminalView` (new `@compozy/ui` primitive) ← `tm-surface`/`tm-grid`/cursor/selection ·
`TerminalWindowApp`/`TerminalTabs`/`TerminalPane`/`TerminalHeader` ← board app-window ·
`TerminalLeaseBadge` ← `tm-lease` · `TerminalInputRequest` ← `tm-request` ·
`TerminalJournalPanel` ← `tm-journal`/`jr-*` · `TerminalRecordingPlayer` ← `tm-replay` ·
`SessionTerminalBlock`/`SessionAgentReportedBlock` ← `tm-block`/`tm-block--reported` ·
quote composer slot ← `tm-quote`/`tm-composer-stack`.

## Accessibility

Focus: tokenized `--focus-ring` on every interactive element (inherited from ds-core).
Keyboard: open/switch/take-control/release/detach all reachable; control-state changes
are `role="status"` announcements in production; the grid uses the emulator's
screen-reader mode. Cursor state is never the only carrier — the lease chip names the
controller in text. Reduced motion: cursor blink, record pulse and spinners stop.

## Refinement pass — 2026-08-23 (plain-language + parity)

Applied after review feedback (content confusing / dev-first / off-ladder
components). **The plain-language register below is now part of the contract**;
machine truth always survives as demoted micro mono (`mono-id`) beside the
human words, never as the primary label.

| Machine / old surface word | Surface word now |
| --- | --- |
| `You control` / `<agent> controls` / `Available` | You're in control · <agent> is in control · No one in control |
| `Release` · `Kill` | Release control · Stop (danger, `circle-stop`) |
| `Deliver` · `Reject` (input) | Send · Decline |
| `Reject` (approvals) · `Always allow this shape` | Don't allow · Always allow commands like this |
| `exited 0` / `exited 1` / `signaled · TERM` / `ended · cause unknown` | Succeeded / Finished with errors / Stopped / Ended — + `exit 0`·`exit 1`·`signal TERM`·`cause unknown` as mono-id |
| `pipe` | read-only log |
| journal headers `Actor Dir Exit Approval Detected` | Who · Where · Result · Permission · Confidence (CLI column *order* unchanged) |
| journal cells `allowlisted / human / approved_once / —` | always allowed / approved by you / approved once / not needed |
| detection `marker / approx` | verified / estimated (`exact` stays) |
| `attach pass` mixed wording · `socket` | "connection pass", no transport nouns |
| grants `Allow shape: X` / `Typing in X` | Always allowed: X / Can type in X |

Component/geometry alignments: `.jr-det` joined the pill ladder (18px, pill
radius); request-card input 30→32 (`.ctl` ladder); request glyph 24→26 with
14px icon; resolved request glyphs left the warning lane (neutral by default,
success when answered, info when superseded — `data-outcome`); journal/exit
type floors raised (≥11px mono, command 12px); window head enforces ≤2
trailing actions (record chrome lives only in the recording section).

## Retired before first commit (canon already owns them)

`.tm-btn` (=`.btn`) · `.tm-dialog` (=`.dialog`) · `.tm-empty` (=`.empty`) ·
`.tm-tag` (=`.pill`/`.tag`) · `.tm-table` (=`.table`).

## Retired in the journal rebuild — 2026-08-23

The first journal board grew its own cell vocabulary instead of using the table
ladder, and paid for it: seven equal-weight columns inside an 820px window with
`.scroll-x` (whose right-edge mask permanently faded the last column), a
`.prow` legend whose `text-overflow:ellipsis` truncated all three explanations,
an unpadded toolbar flush against the window border, and outcome words
(`ok` / `error 1`) the refinement pass had already renamed. Retired:

`.jr-exit` (=`.pill[data-tone]` + `.jr-code` mono truth) · `.jr-det`
(=`.pill--xs`) · `.jr-rec` (=`.btn--icon` in its own column) · `.tm-loadmore`
(=`.jr-foot`, which also states rows loaded) · `.scroll-x` inside the journal
(=`table-layout:fixed`) · `.prow` for prose (=a two-column `.jr-legend` table
that wraps; `.prow` stays for short values only).
