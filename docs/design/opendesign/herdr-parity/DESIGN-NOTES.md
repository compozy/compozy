# Herdr parity — session attention · orchestration DX · shortcuts v2

Design contract for the 14 surfaces in `.compozy/tasks/herdr-parity/_uiux.md`,
delivered as seven boards in this folder. Companions: `_spec.md` Part I (behavior
authority), `_user_stories.md` (ACs/ECs), `_dx.md` (keymap + config truth).

## Locked decisions

### The one dictionary (S1 · S3 · S13 · S14)

One exported badge→tone/glyph map. The `session-status-line.tsx` duplicate dies.

- **needs-you class = danger** — `waiting-for-input` (question), `waiting-for-auth`
  (shield), `failed` (cross). One tone owns "you are the blocker"; the glyph
  carries which member. This was the spec's proposed direction; the design pass
  confirms it: warning would read "caution", danger reads "stopped on you".
  `waiting-for-auth` re-inks from warning → danger as part of the unification.
- **done (finished-unseen) = info + check.** Clears on focus. Never inflates the
  needs-you badge.
- **warning stays runtime health** — `hung` (filled) / `unhealthy` (ring). Unchanged.
- **unknown = dashed hollow ring, neutral.** Honesty state — distinct from
  `stopped`'s solid ring; never fake liveness.
- Two scales, one meaning: 7–9px shapes on rows (circle · diamond · square ·
  check · rings), 18px tinted glyph roundels (`.sig`) on bell rows, palette rows,
  toasts and the window status line. The state word (exact CLI vocabulary:
  `waiting-for-input`, never "asking") is always present — color is never the
  only channel (WCAG 1.4.1 floor).
- **Precedence (US-001.EC-1):** question + permission coexisting renders ONE
  badge — `waiting-for-auth` outranks `waiting-for-input` (the permission gate is
  the harder block). The row sub shows `+1 question`; the bell row lists both
  reasons. `running` beats `done` instantly on a new turn (US-002.EC-4).

### Sidebar (S1 · S2)

- Sessions is a normal OS window (WindowFrame) — the list lives in its body,
  scope + sort in the 38px context strip. No rail, no legacy pagehead.
- Scope is tri-state: `Recent | All | All workspaces`. The third widens to every
  workspace, grouped by workspace with sticky headers, per-group inline failure
  (`Couldn't load sessions · Retry`) and quiet empties — one workspace failing
  never blanks the list (US-031.EC-1).
- Attention-first sort is an option beside the scope, persisted per operator;
  Last activity stays default. Needs-you band first (recency, stable ties), then
  running, then last activity. Keyboard selection follows the session, not the
  index (US-007.EC-2).
- Scale posture: 28px rows, hairline dividers, collapse per group
  (`data-collapsed` contract), virtualize past ~60 rows.

### Bell (S3 · S4)

- Two sections, populated-only: **Needs you** (input/auth/failed sessions + task
  approvals) and **Finished** (`done`). Rows: 18px roundel · title · reason ·
  workspace label · relative time. Activation switches workspace and focuses the
  session window; landing on a Finished row marks it seen.
- Badge = needs-you count, cross-workspace, staleness-gated (stale source
  contributes zero, rows stay clickable dimmed as fallback jump). Finished never
  counts. Menubar pill keeps the 9+ cap.
- Muted workspaces keep rows + counts; the row wears a small `bell-off` mark
  (US-015.AC-1). Quiet state is designed ("All quiet"), disconnected preserves
  the existing `os-bell-disconnected` contract as a glass notice.
- **S4 tab title (copy proposal, COPY.md pass owns final):** `(4) compozy — Compozy`
  while count > 0; clean title at zero; exact number in the title even when the
  pill caps at 9+.

### Toasts (S5 · S7)

- Three kinds: needs-you (immediate, per session, dedup 5s, body = the reason in
  plain words, no buttons — the toast IS the jump), coalesced completion
  ("N sessions finished", the only toast with an action → bell Finished),
  agent-sent (sender identified: `agent — session`).
- Stack: newest first, 4 visible, rest fold into a `+N more need you` ledge that
  opens the bell (US-008.EC-1). Focused session's own transitions never toast.
- Resolved-before-click: the jump still lands on the session with a quiet info
  notice — never an error, never a dead end (US-008.EC-4).
- Sound (S7): one built-in chime per delivery batch, on by default, global
  toggle only, silent failure (US-010.EC-2). No visual surface beyond the S8 row.

### Settings → Attention (S8 · S6)

- Policy only: three channel toggles + mute list. Everything applies live
  (`[attention]` round-trip with CLI) — no save bar.
- System channel renders its REAL platform state as a chip beside the switch:
  `Armed` (success) / `Denied` (warning + "Open System Settings") / `Unavailable`
  (neutral, switch disabled). No pretend-armed toggles (US-012.EC-1/EC-2).
- Native notification rendering is platform chrome — content-only spec table
  (title / body / click target per kind). Activation: app → workspace → session.
- Mute list: workspace avatar rows + Unmute; empty state is one quiet row;
  orphan cleanup invisible (US-011.EC-3). Defaults: toasts on · sound on ·
  system off (opt-in via permission flow).

### Shortcuts (S9 · S10 · S11)

- Keymap truth lives in `_dx.md → Keyboard Defaults` — S9 and S11 render it,
  never copy it. Precedence rule surfaces as a "shadowed" diagnostic: focused UI
  wins over global chords.
- **Chord grammar:** solid `.key` = primary, dashed = alternate, struck = displaced.
  Range families render compactly (`⌃⌥1…8`) and expand members only while one
  digit diverges (overridden digit marked accent).
- **Tabs group** joins the table (13 previously unrebindable actions).
- **Conflicts name the member:** blocked expands the exact digit
  ("`⌃⌥3` is member 3 of `window.tab.jump`"); shadowed names the winning surface.
- **Terminal preset:** preview diff lists every change INCLUDING displaced
  defaults (tab jumps → `⌃⌥` digits; bottom tiles → `⌃⌥⇧` layer), flags layout
  hazards (AltGr aliasing) inline, applies atomically as plain overrides,
  reverts in one step, re-apply is a no-op. "Copy as TOML" mirrors the pasteable
  block in `_dx.md`.
- **Cheatsheet:** `?` opens outside editables, `⌘/` everywhere; two-column glass
  overlay derived from the live registry on every open (defaults + overrides +
  preset, override rows marked). Surface-local bindings (palette, permission
  dock 1–4, steer, composer) are a read-only, lock-marked section. One shared
  modifier-glyph helper replaces the 3 duplicated renderers + 7 JSX labels.

### Palette (S12 · S13)

- Generic view stack over the `destinationWindowId` seam: root results push
  registered views; breadcrumb chips live in the input area (ancestors collapse
  to "…" past two); ⌫ on empty query pops one level; Esc closes the stack;
  reopening starts at root. Search + ↑↓/⏎ identical at every level. Built-in
  registry only (v1).
- Sessions view: state chips (All / Needs you / Working / Finished / Idle) with
  truthful counts, single-select, one-keystroke clear on zero matches; scope
  chip widens to all workspaces (labels on rows); attention-first order; ⏎
  focuses the session window (restores closed windows), `done` arrival marks
  seen. ⌘E deep-opens the palette inside this view.

## Glass grammar

Shell-glass surfaces (`--shell-glass-pop`, blur 30–34px, 12–14px radii,
`--shadow-overlay` + top light edge) are reserved for floating chrome: bell
popover, toasts, cheatsheet, palette, sort menu — the workspaces-overview
lineage. Window content stays on the production radius scale (3–10px) per
ds-core. Accent budget: running pulses, focus ring, primary Apply, the override
hairline — accent is state, never wash. **Selection is a neutral plate**
(`--row-selected` + top light edge), never an accent bar — the palette's old
inset-accent active marker is retired in ds-shell too (wsov lineage).

## Files

Each board = final surface (§01, on the OS shell) + states lab (following
sections). `index.html` maps the set.

- `herdr-parity-sidebar.html` — S1 + S2 + S14 (+ the dictionary legend)
- `herdr-parity-bell.html` — S3 + S4 copy spec
- `herdr-parity-toasts.html` — S5 + S7 delivery rules
- `herdr-parity-settings-attention.html` — S8 + S6 content table
- `herdr-parity-settings-shortcuts.html` — S9 + S10
- `herdr-parity-cheatsheet.html` — S11
- `herdr-parity-palette-sessions.html` — S12 + S13
- `herdr.css` — domain vocabulary (companion to ds-core + ds-shell)

Iterate on these files; don't regenerate. Implementation tasks cite the boards
as visual contracts — artboard CSS is a contract, never a stylesheet to import.
