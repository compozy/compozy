---
name: architectural-analysis
description: "Audit architecture, dead code, duplication, type confusion, and code smells across a codebase. Excludes formatting, performance profiling, security audits, and feature-level reviews."
disable-model-invocation: true
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---

# Architectural Analysis

Audit the requested scope and report concrete design problems. An audit alone is read-only; existing explicit authorization to apply findings can include remediation. Use the project's boundaries and public compatibility policy when judging a design.

1. Resolve scope from the request/context. Map entry points and relevant dependencies with `rg --files`, scoped `rg` searches, and available static-analysis output. Do not create one todo per source file or assume a TypeScript stack.
2. Prioritize signals: dead exports, duplicated responsibilities, cycles, confused ownership/types, and expensive change patterns. Read the matching section of `references/detection-catalog.md` when classifying a candidate; thresholds are clues, not automatic defects.
3. Trace usage before declaring code dead, including dynamic loading, reflection, framework hooks, tests, and published APIs. An unreferenced public export is not safe to delete solely because this checkout has no consumer.
4. Confirm each material finding in source. Explain the behavior or maintenance cost, cite paths/lines, state uncertainty, and propose the smallest useful change. Avoid a separate generic smell sweep when it adds no evidence.
5. Match report size to the request. For a broad audit, use `assets/report-template.md` as an adaptable outline and save `.audits/architectural-analysis-<timestamp>.md`; distinguish inspected and uninspected areas. For a narrow answer, a concise finding list is enough. Missing coverage must never be reported as `None found`.
6. Summarize the important findings and evidence. Existing source and summary templates are optional presentation aids, not two required reports.
