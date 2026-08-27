## 15bf6af1f103d644 — Keep the call hook catalog aligned with observability

- Scope: internal/hooks/**
- Rule: Keep the extension hook family and canonical call observability catalog on the same eleven event names.
- Why: One shared catalog prevents lifecycle observations from drifting between extensions, runtime diagnostics, docs, and QA.
- Origin: session, 2026-08-26

## cf13849291102e09 — Never fold an applied migration into another migration

- Scope: internal/store/globaldb/schema/migrations/**
- Rule: Preserve every existing migration byte and version; add a later migration when schema evolution is required.
- Why: The repository's append-only migration identity rule forbids editing, folding, renumbering, or reordering existing migrations.
- Origin: session, 2026-08-26
