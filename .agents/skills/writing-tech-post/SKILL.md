---
name: writing-tech-post
description: Writes and edits engineering posts, including launches, incidents, migrations, performance, tutorials, AI systems, and security. Use when drafting, restructuring, choosing evidence, or reviewing an engineering post for publication. Do not use for API references, marketing pages, release notes, or internal specifications.
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---
# Writing Engineering Posts

Produce the requested artifact: a new post, an outline, a focused rewrite, or a publication review. Reuse the accepted brief and evidence. Choose a structure that fits the reader and material; a small edit does not restart an entire authoring workflow.

1. Establish the reader, main point, and supported facts from available context. Ask only for missing information that materially affects the requested output.
2. For a new post, choose a useful primary archetype and outline. For an existing draft, retain working structure and edit the requested scope. Frontmatter, rung labels, depth tuples, and per-section evidence plans are optional tools unless the publisher requires them.
3. Explain the mechanism and why it matters. Support quantitative, comparative, security, and capability claims with appropriate evidence and limitations. Distinguish actual measurements, illustrative examples, hypotheses, and work still in progress.
4. Refine voice and flow where needed. Publisher style matrices, H2 question chains, and first/last-200-word comparisons are editorial heuristics, not universal gates. Do not invent failed mitigations, partial victories, benchmark results, or lessons to satisfy an arc.
5. Before external publication, apply `references/pre-publish-checklist.md` to the relevant claims and disclosure obligations. Draft delivery is not publication authorization. The optional linter reports heuristics; human factual review owns unsupported claims and disclosure decisions.

## Reference router

Read the reference for the current archetype or editing concern, not every phase:

- Structure: `references/archetypes-and-structure.md`; optional depth diagnosis: `references/depth-and-abstraction.md`.
- Incident: `references/postmortems.md`; migration: `references/migrations.md`; performance: `references/performance-deep-dive.md`.
- AI/agents: `references/ai-and-agents.md`; security/reliability: `references/security-and-reliability.md`.
- Evidence/assets: `references/evidence-diagrams-code.md`; narrative: `references/narrative-and-pacing.md`.
- Voice/disclosure: `references/voice-and-disclosure.md`; a requested publisher register: `references/publisher-voice-matrix.md`; specific prose problems: `references/anti-patterns.md`.

Use an `assets/outline.<archetype>.md` template only when useful; omit irrelevant sections. Run helpers through this skill's absolute directory: `python3 <writing-tech-post-dir>/scripts/lint-post.py <draft.md>`. `--strict` makes heuristic warnings fail for an explicitly chosen editorial gate. A clean lint result does not prove factuality or disclosure readiness.
