# Goose Migration Authoring

Use the owner directory shape:

```text
schema/
├── schema.sql                 # small stream
├── definitions/              # or ordered domain fragments for a large stream
│   ├── 10_<domain>.sql
│   └── 20_<domain>.sql
├── embed.go
└── migrations/
    ├── 00001_baseline.sql
    ├── 00002_<change>.sql
    └── atlas.sum
```

1. Make the declarative source describe the complete desired stream schema. Use one `schema.sql` for a cohesive small stream or flat, lexically ordered domain fragments for a large stream; never generate a monolithic compatibility file.
2. Run `make codegen`; the Atlas planner appends the next gap-free Goose SQL file, runs SQLite sqlcheck, refreshes `atlas.sum`, and regenerates sqlc.
3. Inspect both `-- +goose Up` and `-- +goose Down`. Ensure the Up path preserves required rows and the Down path is truthful. For a destructive greenfield cut, document the data loss and delete targets rather than adding compatibility branches.
4. Add a bounded backfill to the newly generated, unpublished tail only when declarative DDL cannot preserve required data. Rerun `make codegen` so the checksum reflects the final bytes.
5. Never insert a migration version or edit any preexisting `.sql`/`atlas.sum` identity. A correction after publication is a new migration.

Goose owns transactions and its per-stream version tables (`goose_db_version_global`, `goose_db_version_memory`, `goose_db_version_session`, `goose_db_version_workspace`). Migration SQL must not write those tables directly or issue its own transaction control.
