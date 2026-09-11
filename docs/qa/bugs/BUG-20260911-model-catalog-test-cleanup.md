# BUG-20260911-model-catalog-test-cleanup: Catalog validation intermittently leaves a database after shutdown

- **Status:** open
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Maintainer validating the accumulated QA repairs.
- **Journey Step:** Required local delivery gate, outside the product persona walk.
- **Scenarios:** No additional product scenario verdict; owning suite is TestDaemonModelCatalogWiring.
- **Report:** docs/qa/reports/2026-09-10-qa-execution-unblock.md

## Observed failure

On a172d49ff, `make gate` failed once at `TestDaemonModelCatalogWiring/Should_compose_catalog_service_when_global_DB_and_config_are_available`: TempDir RemoveAll cleanup returned directory not empty. The retained directory contained only a 4096-byte compozy.db. The broad gate had concurrent model discovery and other daemon tests. No runtime panic or failed behavior assertion was reported for this case.

## Investigation and evidence

`rebase-20260911-third-gate.txt` captures the failure. Five consecutive runs of the exact subtest passed under the race detector (`rebase-third-catalog-repro.txt`, 11.780s); the complete owning suite passed (`rebase-third-catalog-suite.txt`, 11.021s). Catalog shutdown already cancels and joins its owned refresh workers before the database closes. The observation is compatible with late database/file activity, but its owner and cause are unproven. No speculative production change, cleanup sleep, retry in the test, or weakened assertion was added. The subsequent full gate result is recorded separately in pr-delivery-gate.txt.

Keep this finding open for a deterministic reproduction if it recurs. No user-state corruption, runtime shutdown failure, or causal connection to the incoming PRs has been established.
