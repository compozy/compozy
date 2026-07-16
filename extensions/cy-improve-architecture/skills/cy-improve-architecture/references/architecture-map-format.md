# Architecture Depth Map format

`.compozy/ARCHITECTURE.md` is the terse, `@import`-safe index over the richer per-area reports in `.compozy/arch-reviews/`. Emit this grammar exactly; the test-only `archmap` package validates the same contract.

## Canonical grammar

```text
# Architecture Depth Map (active)
# @import'd into agent memory. Route behavior INTO deep modules; do NOT widen seams;
# do NOT re-propose avoided deepenings. Detail: .compozy/arch-reviews/<area>.md

## <area> | audited <YYYY-MM-DD> | report <relative-path|->
deep | <target> | <note>
seam | <target> | <note>
avoid | <YYYY-MM-DD> | <what> | <reason>
```

The field separator is the reserved pipe with an ASCII space on both sides: `|`. Alignment may add spaces around the separator, but lines and fields are trimmed. A field must not contain another literal `|`; render such content with `/` instead. Blank lines are ignored.

## Sections

A section header has exactly three fields:

1. `## <area>` — a non-empty area name.
2. `audited <YYYY-MM-DD>` — a real calendar date in that exact zero-padded form.
3. `report <relative-path|->` — a non-empty relative report path, or `-` when no report exists.

Area sections are strictly ascending by area name using lexical order. Duplicate or descending area names are invalid. A section may contain zero entries.

## Entries

Only these entry forms are active grammar:

- `deep | <target> | <note>` — exactly three non-empty fields. Route new behavior into the target.
- `seam | <target> | <note>` — exactly three non-empty fields. Do not widen the target seam.
- `avoid | <YYYY-MM-DD> | <what> | <reason>` — exactly four non-empty fields. The date is a real zero-padded calendar date; the reason is load-bearing, not ephemeral.

Within each section, entries are grouped in `deep`, then `seam`, then `avoid` order. A group may contain any number of entries, including none; entries do not need sorting within their group. There is no per-section line cap. The approximately 150–200-line ceiling is authoring guidance for keeping agent memory terse, not parser grammar.

Lines beginning with `#` are comments and are never active entries. Header commentary and provenance use comments. A malformed `##` line is not a comment: section headers must begin with `## ` and follow the three-field form above.

## Empty state

Before the first audit, emit the canonical header followed by the empty-state comment and no area sections:

```text
# Architecture Depth Map (active)
# @import'd into agent memory. Route behavior INTO deep modules; do NOT widen seams;
# do NOT re-propose avoided deepenings. Detail: .compozy/arch-reviews/<area>.md
# No areas audited yet.
```

This parses as a map whose `Areas` field is nil.

## Superseding an avoidance

An active rejection uses the four-field `avoid` entry. When later work supersedes it, remove it from the active entry group and retain it as a `#` provenance comment. Never silently delete or rewrite the old reason.

```text
## internal/core | audited 2026-07-13 | report .compozy/arch-reviews/internal-core.md
deep | internal/core/router | Route dispatch through this module.
# superseded 2026-07-13: avoid | 2026-06-01 | keep dispatch distributed | Superseded after the routing boundary changed.
```

Because the provenance line starts with `#`, `archmap.Parse` skips it and returns only the active `deep` entry.

## Valid two-area example

```text
# Architecture Depth Map (active)
# @import'd into agent memory. Route behavior INTO deep modules; do NOT widen seams;
# do NOT re-propose avoided deepenings. Detail: .compozy/arch-reviews/<area>.md

## apps/web | audited 2026-07-13 | report .compozy/arch-reviews/apps-web.md
deep | apps/web/navigation | Route new navigation behavior through this module.
seam | apps/web/router | Do not widen the router integration seam.
avoid | 2026-07-12 | merge route handlers | Framework ownership keeps these boundaries load-bearing.

## internal/core | audited 2026-07-13 | report -
```
