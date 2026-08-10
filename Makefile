MAGE_VERSION ?= v1.17.2
MAGE ?=
AIR_VERSION ?= v1.65.3

ifeq ($(strip $(MAGE)),)
MAGE_RUN = bash scripts/run-mage.sh $(MAGE_VERSION)
else
MAGE_RUN = $(MAGE)
endif

.PHONY: deps deps-check fmt fmt-check lint go-lint source-policy source-size product-language-check test test-integration test-e2e-runtime test-e2e-web test-e2e test-e2e-nightly codegen codegen-check build build-go cross-build-windows boundaries verify help bun-lint bun-typecheck bun-test installer-check demo-seed desktop-dev desktop-build desktop-test desktop-lint

DEMO_HOME ?= $(HOME)/.agh

deps:
	@$(MAGE_RUN) deps

deps-check:
	@$(MAGE_RUN) depsCheck

fmt:
	@$(MAGE_RUN) fmt

fmt-check:
	@$(MAGE_RUN) fmtCheck

lint:
	@$(MAGE_RUN) lint

go-lint:
	@$(MAGE_RUN) goLint

source-policy:
	@$(MAGE_RUN) sourcePolicy

source-size:
	@$(MAGE_RUN) sourceSize

product-language-check:
	@$(MAGE_RUN) productLanguageCheck

test:
	@$(MAGE_RUN) test

test-integration:
	@$(MAGE_RUN) testIntegration

test-e2e-runtime:
	@$(MAGE_RUN) testE2ERuntime

test-e2e-web:
	@$(MAGE_RUN) testE2EWeb

test-e2e:
	@$(MAGE_RUN) testE2E

test-e2e-nightly:
	@$(MAGE_RUN) testE2ENightly

codegen:
	@$(MAGE_RUN) codegen

codegen-check:
	@$(MAGE_RUN) codegenCheck

build:
	@$(MAGE_RUN) build

build-go:
	@$(MAGE_RUN) buildGo

cross-build-windows:
	@$(MAGE_RUN) windowsSubprocessBuild

boundaries:
	@$(MAGE_RUN) boundaries

verify:
	@$(MAGE_RUN) verify

bun-lint:
	@$(MAGE_RUN) bunLint

bun-typecheck:
	@$(MAGE_RUN) bunTypecheck

bun-test:
	@$(MAGE_RUN) bunTest

installer-check:
	@$(MAGE_RUN) installerCheck

demo-seed:
	@go run ./scripts/demo-seed --home "$(DEMO_HOME)" --replace

# Desktop shell
desktop-dev:
	@cargo run --locked --manifest-path desktop/src-tauri/Cargo.toml

desktop-build:
	@cargo build --locked --manifest-path desktop/src-tauri/Cargo.toml

desktop-test:
	@cargo test --locked --manifest-path desktop/src-tauri/Cargo.toml --all-targets

desktop-lint:
	@cargo fmt --manifest-path desktop/src-tauri/Cargo.toml -- --check
	@cargo clippy --locked --manifest-path desktop/src-tauri/Cargo.toml --all-targets -- -D warnings

help:
	@$(MAGE_RUN) -l

# Evidence-cached gates
#
# `gate` classifies the branch diff and runs only affected lanes; sensitive
# paths (schema, contracts, config, deps, build tooling) escalate to the full
# verify. `gate-full` is the completion/PR gate. Both record evidence in
# .cache/gate/ keyed by tree-content fingerprint — a gate whose record matches
# the current fingerprint is a no-op. `gate-status` prints records for citing.
.PHONY: gate gate-full gate-status
gate:
	@bash scripts/gate.sh auto

gate-full:
	@bash scripts/gate.sh full

gate-status:
	@bash scripts/gate.sh status

# Documentation Site
.PHONY: site-dev site-build cli-docs cli-docs-check migration-guide-check

site-dev:
	@cd packages/site && bun run dev

site-build:
	@bunx turbo run build --filter=./packages/site

cli-docs:
	@$(MAGE_RUN) cliDocs

cli-docs-check:
	@$(MAGE_RUN) cliDocsCheck

migration-guide-check:
	@$(MAGE_RUN) migrationGuideCheck

# Web UI
.PHONY: web-dev web-build web-fmt web-typecheck web-test

web-dev:
	@cd web && bun run dev

web-build:
	@bunx turbo run build --filter=./web

web-fmt:
	@cd web && bun run format

web-typecheck:
	@bunx turbo run typecheck --filter=./web

web-test:
	@bunx turbo run test --filter=./web

# Parallel worktrees
#
# `worktree-new` creates a sibling worktree at ../_worktrees/<slug>, copies
# shared dirs from main (.claude .codex .compozy .resources docs), then
# bootstraps (mise pins, bun install + skill symlinks; BUILD=1 adds `make
# build`, E2E=1 installs Playwright chromium). `worktree-bootstrap` preps the
# current checkout. Removal: scripts/worktree.sh rm <slug>.
.PHONY: worktree-new worktree-bootstrap
worktree-new:
	@test -n "$(SLUG)" || { echo "usage: make worktree-new SLUG=<slug> [BRANCH=] [BASE=] [BUILD=1] [E2E=1]"; exit 2; }
	@bash scripts/worktree.sh new $(SLUG) $(if $(BRANCH),--branch $(BRANCH),) $(if $(BASE),--base $(BASE),) $(if $(BUILD),--build,) $(if $(E2E),--e2e,)

worktree-bootstrap:
	@bash scripts/worktree.sh bootstrap $(if $(BUILD),--build,) $(if $(E2E),--e2e,)

# QA lab process hygiene
#
# Stops daemons and kills every process still tied to a QA lab (bootstrap labs,
# $TMPDIR/compozyqa-* runtime roots, compozy-iso-* isolation envelopes). Run after any
# QA pass; mandatory before claiming QA completion. Add PURGE=1 to also remove
# lab runtime dirs after a clean sweep.
.PHONY: qa-reap
qa-reap:
	@python3 .agents/skills/eng/eng-qa-bootstrap/scripts/teardown-qa-env.py --all $(if $(PURGE),--purge,)

# Local daemon run
#
# `start` rebuilds both the daemon and web bundle so their public contracts
# cannot drift, then launches the daemon with the COMPOZY_WEB_DIST_DIR override so
# it serves the freshly-built web/dist from disk instead of the embedded bundle.
# `dev` runs the real daemon under Air and the web UI under Vite. The first
# successful Air build gracefully stops any daemon using the active COMPOZY_HOME;
# daemon web routes redirect to Vite, and Air owns every later rebuild/restart.
.PHONY: start stop restart dev dev-daemon

dev: codegen
	@AIR_VERSION="$(AIR_VERSION)" bash scripts/dev.sh

dev-daemon:
	@bash scripts/dev-daemon-runner.sh "$(AIR_VERSION)" -c .air.toml

start: build web-build
	@test -x ./bin/compozy || { echo "bin/compozy not found — run 'make build' first"; exit 1; }
	@echo "Starting daemon serving local web bundle: $(CURDIR)/web/dist"
	@COMPOZY_WEB_DIST_DIR="$(CURDIR)/web/dist" ./bin/compozy daemon start

stop:
	@./bin/compozy daemon stop

restart:
	@$(MAKE) stop || true
	@$(MAKE) start
