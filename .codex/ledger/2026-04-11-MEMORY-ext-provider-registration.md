Goal (incl. success criteria):

- Implement first-class extension provider registration for Compozy.
- Success means: early overlay bootstrap before config validation/forms, typed IDE provider overlays with fallback/bootstrap support, executable extension-backed review providers, minimal model overlay wiring, updated Go/TS SDK surfaces, regression/integration tests, and clean `make verify`.

Constraints/Assumptions:

- Follow root `AGENTS.md` and `CLAUDE.md`; do not touch unrelated dirty files already present in the worktree.
- Required skills active: `brainstorming` (design gate already satisfied by accepted plan), `golang-pro`, `testing-anti-patterns`, `cy-final-verify`.
- Chosen architecture from the accepted plan: lazy subprocess RPC for executable review providers; no `host.providers` namespace in v1; model providers remain alias/catalog only.
- Workspace and user extensions must still honor operator-local enablement and existing precedence rules.

Key decisions:

- Break the workspace bootstrap cycle by discovering the workspace root first, activating overlays, then loading and validating workspace config.
- Keep provider discoverability declarative in manifests; only review-provider execution crosses the extension subprocess protocol.
- Reuse extension manager/session lifecycle primitives for provider RPC instead of inventing a separate transport stack.

State:

- Completed.

Done:

- Read the task prompt, extension-related ledgers, current bootstrap/provider/runtime code, and the required skill files.
- Produced and persisted the accepted implementation plan under `.codex/plans/2026-04-11-ext-provider-registration.md`.
- Confirmed key hidden gaps beyond the prompt: workspace config and reusable-agent runtime defaults currently validate IDE/provider names before overlays are active.
- Reworked CLI/workspace bootstrap so the workspace root is discovered first, extension overlays are activated before `.compozy/config.toml` validation, and interactive form/runtime catalogs use the active overlays.
- Expanded extension manifest/provider parsing and validation to support typed IDE launcher/bootstrap fields, explicit review-provider kinds, and legacy-field normalization.
- Added executable extension-backed review-provider bridges with lazy session startup, active-manager reuse, initialize-time `providers.register` enforcement, and RPC handling for `fetch_reviews` and `resolve_issues`.
- Added minimal command-scoped model alias overlays and wired runtime model resolution through them without regressing passthrough behavior for undeclared model names.
- Extended the Go and TypeScript extension SDKs with typed review-provider registration APIs and initialize-time registered-provider reporting.
- Replaced the TypeScript review-provider starter template with a real executable extension template and updated scaffold coverage accordingly.
- Added regression and integration coverage across CLI bootstrap/forms/help, manifest compatibility, overlay resolution, Go review-provider stdio, and TypeScript review-provider stdio.
- Ran targeted package and template tests during development and finished with a clean `make verify` pass.

Now:

- Final handoff only.

Next:

- Optional cleanup only: remove this ledger in a follow-up if no further continuity is needed.

Open questions (UNCONFIRMED if needed):

- None currently blocking.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-11-MEMORY-ext-provider-registration.md`
- `.codex/plans/2026-04-11-ext-provider-registration.md`
- `.compozy/tasks/ext-imp/_prompt.md`
- `internal/cli/{run.go,form.go,state.go,workspace_config.go,extensions_bootstrap.go}`
- `internal/core/{agent,provider,extension,workspace,agents}`
- `sdk/extension`
- `sdk/extension-sdk-ts`
- `internal/core/modelprovider`
- Commands: `rg`, `sed`, `git status --short`, `go test ...`, `npx vitest ...`, `make verify`
- Final verification: `make verify` -> exit `0`; `0 issues`; `DONE 1710 tests in 41.424s`; build passed; `All verification checks passed`
