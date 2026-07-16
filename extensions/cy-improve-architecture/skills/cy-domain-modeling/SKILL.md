---
name: cy-domain-modeling
description: Maintains a project's ubiquitous language in .compozy/GLOSSARY.md. Use when the user wants to pin down domain terminology, sharpen an overloaded term, or when another skill crystallizes a new domain concept. Do not use for architecture decisions, implementation specifications, or general programming terminology.
---

# Domain Modeling

Actively build and sharpen the project's ubiquitous language as concepts crystallize. Challenge terms, probe edge cases, cross-check the code, and keep the accepted vocabulary in `.compozy/GLOSSARY.md`.

Create the glossary lazily: read it when present, but do not create it until the user accepts the first new or sharpened term. Keep it free-form and unvalidated. Use the structure and rules in [glossary-format.md](glossary-format.md).

## Workflow

1. Read `.compozy/GLOSSARY.md` when it exists.
   - Treat a missing file as an empty glossary.
   - Compare proposed terms case-insensitively with existing canonical terms and their `_Avoid_:` aliases.
   - Complete this step when the relevant existing language and possible duplicates are known.

2. Sharpen the language.
   - Call out a term that conflicts with an existing definition.
   - Replace vague or overloaded language with one precise canonical term.
   - Invent concrete edge-case scenarios when relationships or concept boundaries remain fuzzy.
   - Check whether the code agrees with a claimed domain rule and surface contradictions for resolution.
   - Complete this step when the concept has one agreed name and a tight definition.

3. Offer the glossary update.
   - Show the exact new or revised entry before writing.
   - If the concept already exists, revise that entry rather than appending a duplicate.
   - If the user declines, write nothing.
   - Complete this step when the user has accepted or declined the proposed wording.

4. Apply an accepted update immediately.
   - Create `.compozy/GLOSSARY.md` with `# Glossary` only when writing its first accepted term.
   - Preserve unrelated existing entries and headings.
   - Keep the definition free of implementation details.
   - Complete this step when exactly one canonical entry represents the concept.

## Error Handling

- If existing prose does not follow the suggested format, preserve it and make the smallest compatible edit around the accepted term.
- If the workspace root cannot be identified or the glossary cannot be written, report the path problem and leave the repository unchanged.

> _Adapted from Matt Pocock's MIT-licensed [`domain-modeling`](https://github.com/mattpocock/skills/tree/main/skills/engineering/domain-modeling) skill, slimmed to glossary upkeep. See the extension `NOTICE` for the upstream copyright and license._
