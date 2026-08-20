---
title: A guarded history reference no longer crashes generation 1
type: fix
---

A Loop that referenced `previous.*` defensively still failed on its first generation, because the history namespace did not exist yet when templates were evaluated. The complete shape of `previous.*` and the generation history namespaces is now defined before evaluation, so documented guarded references validate and generation 1 runs. (#438)

- History construction is topology-aware, so a node sees the namespaces its position actually implies.
- Template and materialization failures stay inside the node lifecycle instead of escalating into an opaque coordinator failure.
- Canonical compiler, linter, namespace, coordinator, and control-flow suites cover the behavior.

```gotemplate
{{ if .previous.generation }}
The prior quality gate returned {{ .previous.verdicts.quality.outcome }}.
Blocking issues: {{ .previous.verdicts.quality.blocking_issues }}
{{ end }}
```
