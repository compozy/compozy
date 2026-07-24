# Index Format

The terse index `.compozy/DECISIONS.md` is the loaded surface: it is designed to be `@import`ed whole
into the project's agent-memory file (`CLAUDE.md` / `AGENTS.md`), so every agent session — planning
included — loads it automatically. Keep it terse; the rich detail lives in the `AD-NNN.md` bodies.

## File Location

```
.compozy/DECISIONS.md
```

At the **workspace root**, never under `.compozy/tasks/<slug>/`.

## Line Grammar

One line per **active, proven** decision:

```
AD-NNN | Title | status | [tags] | rationale | source_slug
```

Six pipe-delimited fields, in this fixed order:

- **AD-NNN** — the decision id; matches the body filename `.compozy/decisions/AD-NNN.md`.
- **Title** — the decision's one-line title (same as the body `title`).
- **status** — always `proven` for a line that appears here (the membership rule below guarantees it).
- **[tags]** — the body's `tags`, rendered as a bracketed comma list, e.g. `[orders, async]`.
- **rationale** — a one-line reason the decision matters; the terse "why", not the full body.
- **source_slug** — the originating workflow slug (provenance at a glance).

**`|` is the reserved field delimiter.** The line is split on `|` into exactly six fields, so **Title**
and **rationale** must not contain a literal `|` — render any pipe as `/` (e.g. `JWT / opaque tokens`).
Title is copied verbatim from the body `title` (kept pipe-free for the same reason), so the two always
agree and the validator's index↔body check passes.

## Membership Rule

- Include a record **iff** it is `proven` AND not `superseded` (i.e. `superseded_by: null`).
- Exclude every `candidate` record — it lives in its `AD-NNN.md` file but is never loaded.
- Exclude every `superseded` record — in a chain A→B→C, only the active tail (C) appears.
- This is ADR-003: keep the `candidate` status, but load only `proven`, active decisions.

## Structure

Begin with a short header comment so the imported block is self-describing, then the lines. Example:

```
# Project Decisions (active, proven)

# Imported into agent memory. One line per active, proven decision.
# Rich bodies: .compozy/decisions/AD-NNN.md

AD-001 | Event-sourcing for orders | proven | [orders, async] | audit + replay | feat-orders
AD-004 | Idempotency keys on writes | proven | [orders, api] | safe client retries | feat-payments
```

## Empty State

When no decision is active-proven (fresh project, or every promoted decision is a `candidate`), write a
valid empty-state index — the header plus a note — not an empty file:

```
# Project Decisions (active, proven)

# No active, proven decisions captured yet.
```

## Rules

- Sort lines by `AD-NNN` ascending so diffs stay stable across captures.
- Do not include headers, columns, or fields beyond the six-field grammar on a decision line.
- Regenerate the full set of active-proven lines on each capture; do not append duplicates.
- The file must stay parseable by the `decisionlog` validator (task_02) and safe to `@import` whole.
