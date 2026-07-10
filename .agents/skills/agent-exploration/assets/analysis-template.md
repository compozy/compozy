# Analysis: {slug}

Read-only exploration of the slice `{slug}` (ordinal `{NN}`) for the research prompt:

> {operator_prompt}

## Scope

- Slice question: {one-line slice question the parent assigned}
- Primary sources: {paths, directories, URLs, or topical bounds the parent named}
- Sources read in full vs. sampled:
- Total candidate sources surveyed:

## Overview

(2-4 paragraphs)

What does this slice cover? What is its scope? Where does it overlap with adjacent slices? What is the operator most likely to act on from this slice?

## Mechanisms / Patterns

(Bulleted; each item names the pattern + the mechanism)

- **Pattern name:** what it does, where in the source, why it matters.
- ...

## Relevant Sources

(Cite files, line ranges, or URLs that implement the patterns above. Real paths and URLs, not paraphrased.)

- `path/to/file.ext:NNN-NNN` — purpose
- `https://example.com/...` — purpose
- ...

## Transferable Patterns

(Patterns that map cleanly into the operator's working context. Each item: what to take, where it would apply, what it replaces or augments.)

- {Pattern name} → applies to {target area} because {reason}.
- ...

## Risks / Mismatches

(Patterns that look attractive but conflict with the operator's constraints — architectural rules, performance budgets, security invariants, etc. Cite the constraint when known.)

- {Pattern name} would violate {constraint} because {reason}.
- ...

## Open Questions

(Things this slice can't resolve. The parent decides.)

- ...

## Evidence

(Final list of source citations referenced above, deduplicated. The next reader will follow these directly.)

- `path/to/file.ext`
- `https://example.com/...`
- ...
