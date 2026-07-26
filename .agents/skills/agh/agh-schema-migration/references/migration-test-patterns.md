# Migration Test Contract

Extend existing suites; do not create a second test for an invariant already owned below.

| Invariant | Owning suite |
| --- | --- |
| Embedded streams are gap-free and have `atlas.sum` | `internal/store/migrate_streams_test.go:TestProductionMigrationStreams` |
| Applied migrations equal the complete declarative schema | `internal/store/migrate_streams_test.go:TestMigrationSchemaEquivalence` |
| Fresh apply, repeated apply, ahead refusal, and migration failure | `internal/store/migrate_test.go:TestApplyMigrationStream` |
| Checksum/integrity refusal | `internal/store/migrate_test.go:TestApplyRejectsAtlasSumDrift` and `internal/store/migrate_status_test.go` |
| Real open/reopen and row preservation | The owner suite: `global_db_test.go`, `session_db_test.go`, `catalog_migration_test.go`, or `workspace_db_test.go` |
| Global+memory shared-file isolation | `TestProductionMigrationStreams` and `internal/memory/catalog_migration_test.go` |

For an appended migration, seed a file with the previous embedded prefix, insert representative rows, close it cleanly, reopen through the production owner, and assert preserved/transformed data plus `store.Status` at the new version. Use a separate `t.TempDir()` database per parallel subtest.

When refusal or recovery changes, assert byte identity on refusal and whole-family handling for `.db`, `-wal`, `-shm`, and sibling databases. Do not assert prose, generated-file existence, or raw schema formatting when the stronger codegen and behavioral gates own the contract.
