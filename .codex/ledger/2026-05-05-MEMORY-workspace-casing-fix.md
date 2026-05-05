Goal (incl. success criteria):

- Fix the looper root cause where workspace extension enablement misses when the same workspace is referenced with different path casing.
- Success: regression test proves old lowercase enablement keys resolve for the canonical workspace root; implementation canonicalizes/merges workspace enablement keys instead of editing state by hand; focused tests and `make verify` pass.

Constraints/Assumptions:

- Use `systematic-debugging`, `no-workarounds`, `golang-pro`, `testing-anti-patterns`, and `cy-final-verify`.
- No destructive git commands.
- Use `apply_patch` for manual file edits.
- Conversation in BR-PT; code/artifacts in English.
- Treat `../looper` as the correct Compozy source repo.

Key decisions:

- Fix extension enablement path identity at the source, not by manually modifying `~/.compozy/state/workspace-extensions.json` or task files.
- Preserve existing user choices by normalizing legacy workspace enablement keys during reads/writes.

State:

- Completed; full verification passed.

Done:

- Read looper `AGENTS.md`.
- Read relevant skills.
- Confirmed prior evidence: run-scope extension loading receives no extensions because enablement lookup keys differ by workspace root casing.
- Found existing globaldb path-case canonicalization, but extension enablement used a weaker local normalizer and exact map lookup.
- Added regression coverage for `EnablementStore` loading legacy workspace enablement keys through a canonical root alias.
- Added discovery coverage proving a workspace extension is included when the manifest is under the canonical root and enablement state was stored under an alias root.
- Implemented workspace-root canonical casing in extension enablement and deterministic merge of equivalent workspace enablement keys on load/save.
- Focused tests passed: `go test ./internal/core/extension -run 'TestDiscoveryIncludesWorkspaceExtensionEnabledThroughCanonicalRootAlias|TestEnablementStoreLoadsWorkspaceStateAcrossCanonicalRootAliases|TestCanonicalizeExistingPathCaseWithUsesOnDiskNames' -count=1`.
- Package tests passed: `go test ./internal/core/extension -count=1`.
- Full verification passed: `make verify`.
- Verification evidence: frontend lint reported 0 warnings/errors, web tests reported 239 passed, Go lint reported 0 issues, Go tests reported 3042 tests with 3 skipped, Playwright e2e reported 5 passed, final line `All verification checks passed`.
- Rebuilt local binary: `bin/compozy --version` reports `v0.2.1-5-g393c97c`.
- Diff hygiene passed: `git diff --check`.

Now:

- Final handoff.

Next:

- Restart/use the rebuilt daemon binary before expecting existing daemon-backed `tasks run` commands to pick up this code change.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/ledger/2026-05-05-MEMORY-workspace-casing-fix.md`
- `internal/core/extension/enablement.go`
- `internal/core/extension/enablement_test.go`
- `internal/core/extension/discovery_test.go`
- `make verify`
