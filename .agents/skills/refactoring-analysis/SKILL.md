---
name: refactoring-analysis
description: "Audit code quality and architectural health or plan refactoring using Fowler code smells and techniques; write prioritized findings to docs/_refacs/. Excludes formatting, performance, and security audits."
disable-model-invocation: true
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---

# Refactoring Analysis

Find changes that reduce demonstrated maintenance cost while preserving behavior. Name the code smell, source evidence, and concrete improvement; a textbook category without a practical consequence is not a finding.

1. Resolve the scope from the request and active context. For a broad request, map the codebase and prioritize dependencies/hotspots; do not stop for confirmation merely because it exceeds 50 files. Disclose sampled or uninspected areas.
2. Read the relevant sections of `references/code-smells-catalog.md` and `references/refactoring-techniques.md` for the candidates found. Catalog thresholds are signals, not mandatory edits or a requirement to produce findings in every category.
3. Trace ownership, coupling, call sites, and public boundaries. Consult `references/solid-ddd-context.md` only for a domain-model question; ordinary code does not need a SOLID pass.
4. Rank confirmed opportunities by user/maintenance impact, frequency, risk, and effort. For the highest-impact items, describe the before/after behavior and owning verification. Reuse existing coverage; missing tests do not automatically require a new suite at every layer.
5. A requested report defaults to `docs/_refacs/<YYYYMMDD>-<slug>.md`. Adapt `assets/refactoring-report-template.md`; a narrow request can use a concise response. Check material citations and reasoning with `checklists/analysis-checklist.md` as applicable.
6. If the user requested an audit only, report findings. If they already authorized refactoring, continue the selected work without asking again. Preserve published compatibility and unrelated changes.
