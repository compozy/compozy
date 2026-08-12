# Worktree design set — shared contract

Spec: `.compozy/tasks/worktree-support/_uiux.md` (surface map S1–S16, states per surface).
Copy authority: `analysis/07_analysis_assisted-exit-ui.md` §Transferable Patterns 8 + `COPY.md` register.
Every file links `../design-system/ds-core.css` then `worktree.css` (this folder). Page-local styles stay inline and small. Lucide via CDN + `createIcons()`.

## Shared data story (same entities across every artboard)

Workspace **compozy** (`~/dev/compozy`, home workspace, git-backed) — worktrees:

**Canonical worktree path (ADR-005, central home):** `~/.compozy/worktrees/<workspace>/<name>` — in this story `~/.compozy/worktrees/compozy/<name>`. The in-repo (`<repo>/.worktrees/`) and sibling (`~/dev/<repo>-<name>`) forms are ADR-005's rejected alternatives — never render them. Exception: a *discovered external* worktree keeps its foreign path (`../compozy-spike`), before and after adoption.

| name (record label) | branch | state | facts |
| --- | --- | --- | --- |
| payments-retry | `pedro/payments-retry` | ready | dirty +142 −18 (6 files) · ↑3 · agent **running** (session "Fix payment retries", claude) · last activity 4m |
| auth-refresh | `pedro/auth-refresh` | ready | clean · ↑0 ↓2 (behind) · PR #412 open · idle · 1h |
| fix-flaky-e2e | `run/fix-flaky-e2e` | ready | origin `per-run` · clean · PR #398 **merged** → safe-to-clean · 2d |
| data-migration | `pedro/data-migration` | ready | setup-flagged (bootstrap failed: `bun install` exit 1) · clean · ↑1 · 3h |
| spike-sqlite-vacuum | `spike/sqlite-vacuum` | discovered | external, not adopted · found at `../compozy-spike` |
| docs-refresh | `pedro/docs-refresh` | pending | materializing (create → branch → bootstrap) |
| hotfix-cors | `pedro/hotfix-cors` | missing | directory removed out of band · history preserved |
| bench-harness | `pedro/bench-harness` | error | `git status` failed: not a git repository (read failure blocks actions) |

Workspace **branas-site** (`~/dev/branas/site`, git-backed, **zero worktrees**).
Workspace **notes** (`~/notes`, **not git-backed** → no worktree affordance at all).

Detached example (S5 sheet only): pinned at `9ec3d15`.
Base ref: `main`. Remote: `github.com/compozy/compozy`. Task: **Nightly triage** (worktree policy demos). Loop: **software-delivery** (loop-level default demos). Sessions: "Fix payment retries" (bound → payments-retry, running), draft session (composer demos).

## Locked decisions (apply everywhere)

- **Chip enum** = `ready · pending · discovered · missing · error` via `.wt-chip[data-state]`. **Ready-chip density rule: the chip renders only when state ≠ ready** — a ready row carries the quiet success dot alone, in full rows and nests alike. Ready is calm, never loud.
- **Nested state dots** use `.wt-dot[data-state]` from `worktree.css` (single source of shape truth, mirrors `.wt-chip::before`): pending = filled warning diamond · discovered/missing = hollow info ring · error = danger square. Ready keeps ds-core `.d--ok`. Never restyle dots inline.
- Semantic tones: success = merged/bootstrap-ok/safe-to-clean · warning = dirty/pending/setup-flagged/**behind`[data-hot]`** (action needed — registered) · danger = destructive/error · info = discovered/stale/missing. Accent only: agent-running dot + the single primary action per screen.
- **Counts count adopted only** — discovered entries never enter a worktree count. Story numbers: overview "3 workspaces · 7 worktrees", switcher overflow "All 7 worktrees", index lede "eight worktree records (7 adopted, 1 discovered)".
- **Adopt-on-select (ADR-002 / US-009):** discovered rows in S1/S2/S3 are selectable and carry a trailing `Adopt` affordance; selecting opens the adopt confirm, which names the validation ("metadata resolves to this repository") and states bootstrap is **not** re-run. Refusal state: metadata resolving into the main checkout refuses with the reason, directory untouched (AC-3). Pending/missing stay non-selectable.
- **Remove-confirm titles quote the record label** — `Remove "fix-flaky-e2e" from disk?` — never the branch or path basename. Branch stays in the target card and the "branch is not deleted" line.
- **Held branch in create (US-002 EC-2):** picking a branch another worktree holds refuses at the field and offers "Select that worktree instead" as a secondary escape — never a duplicate checkout, never a silent jump. Held rows are badged with the holder in the picker.
- **Environment-mode vocabulary (locked):** `Workspace root · Inherit · Named worktree · Per-run · Directory` — the only mode labels across S7/S8/S10/S12/S13. Never `None`, `Unset`, `Root`, or `Inherit workspace`.
- **Agent-activity vocabulary:** `running` (filled accent pulse) · `awaiting-input` (hollow accent `.d--wait` — turn finished, waiting on the user; **exit actions unlock in this state**) · `idle` (renders nothing in rows). Session status dots: accent = running, success = finished, neutral = idle.
- **S6/S16 mount (OQ7, locked):** the binding chip always mounts in the session header; the exit split button mounts only in the worktree detail context — never in the session header (dot + title + agent + chip + menu is already the density ceiling).
- **Nest sort order (S1/S2/S3 — one rule, identical render):** state group `ready → pending → discovered → missing → error`; within a group, last activity desc, unknown last. Truncated nests keep the five most recently active rows; the overflow row opens the overview.
- **Gating comments:** every distinct control class per artboard cites its gate at least once. Canonical prefix `gating:`; `stream:` / `result:` / `events:` are blessed for streamed-phase surfaces (exit-progress).
- **Spec scaffolding never renders.** US/AC/ADR ids, spec slugs, and artboard filenames live in `.spec__note` columns and HTML comments only — never inside a mock's fidelity boundary.
- The create dialog's educational subtitle ("A separate checkout of <repo> on its own branch and directory.") is the set's one authorized helper-text exception.
- **Nest density rule:** 30px nested rows carry name + state signal + at most one trailing signal. Companion badges (`per-run` origin, setup-flag text) render in full 44px rows only; in nests the setup flag is icon-only with an `aria-label`.
- **Generation honesty rule (from the commit sheet, applies set-wide):** copy may promise generation ("Leave empty to generate…") only where daemon/agent-backed generation is actually assumed (synara git.ts:181-187 contract; OQ2). If v1 ships a fixed default instead, the copy MUST become "Leave blank to use a default message" — a promise against a hardcoded constant is a UI lie.
- **Truthful UI**: unknown renders as em-dash/absent (`data-unknown`), never `+0 −0` or "clean"; ahead/behind only when upstream known; PR widgets absent (not disabled) with zero credentials; stale remote values marked with `.wt-stale` timestamp.
- **Name is the record label** (never the path basename). Branch is mono + copyable. Path is micro mono (≤10.5px, faint), demoted not removed.
- Nesting depth 2 (workspace → worktrees) via `.wt-nest` indent + 1px rail. Never box-drawing glyphs.
- Blocked actions render the reason as a functional label (menu row `.gmenu__reason`, tooltip). No helper prose under headings.
- Split button `.splitbtn` auto-advances: Commit → Commit & push → Push → Open PR → View PR. Status read failure blocks the whole control.
- Every control cites its gating payload field or route in an HTML comment (GUIDE directive).
- Provenance comment at the top of every file: production / spec worktree-support / authorized delta.

## Files

Finals: nav-switcher, menubar-menu, overview, create-dialog, session-create, composer-environment, task-setup, fanout, loop-config, detail-exit, commit-sheet, pr-sheet, remove-dialogs, exit-progress, merged-cleanup (the last two own required S14/S16 states — contract, not lab).
Labs: row-states (anchor vocabulary).
`index.html` maps finals × labs; finals cross-link each other through the shared story.
