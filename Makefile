MAGE_VERSION ?= v1.17.2
MAGE ?=
AIR_VERSION ?= v1.65.3

ifeq ($(strip $(MAGE)),)
MAGE_RUN = bash scripts/run-mage.sh $(MAGE_VERSION)
else
MAGE_RUN = $(MAGE)
endif

.PHONY: deps fmt fmt-check lint go-lint test test-integration test-e2e-runtime test-e2e-web test-e2e test-e2e-nightly codegen codegen-check build build-go boundaries verify help bun-lint bun-typecheck bun-test installer-check

deps:
	@$(MAGE_RUN) deps

fmt:
	@$(MAGE_RUN) fmt

fmt-check:
	@$(MAGE_RUN) fmtCheck

lint:
	@$(MAGE_RUN) lint

go-lint:
	@$(MAGE_RUN) goLint

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

help:
	@$(MAGE_RUN) -l

# Documentation Site
.PHONY: site-dev site-build cli-docs

site-dev:
	@cd packages/site && bun run dev

site-build:
	@bunx turbo run build --filter=./packages/site

cli-docs:
	@go run ./cmd/agh doc --output-dir packages/site/content/runtime/cli-reference
	@bunx oxfmt packages/site/content/runtime/cli-reference

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
# `worktree-new` creates a sibling worktree at ../<repo>-worktrees/<slug> and
# bootstraps it (mise pins, bun install + skill symlinks; BUILD=1 adds `make
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
# $TMPDIR/aghqa-* runtime roots, agh-iso-* isolation envelopes). Run after any
# QA pass; mandatory before claiming QA completion. Add PURGE=1 to also remove
# lab runtime dirs after a clean sweep.
.PHONY: qa-reap
qa-reap:
	@python3 .agents/skills/agh/agh-qa-bootstrap/scripts/teardown-qa-env.py --all $(if $(PURGE),--purge,)

# Local daemon run
#
# `start` rebuilds both the daemon and web bundle so their public contracts
# cannot drift, then launches the daemon with the AGH_WEB_DIST_DIR override so
# it serves the freshly-built web/dist from disk instead of the embedded bundle.
# `dev` runs the real daemon under Air and the web UI under Vite. The first
# successful Air build gracefully stops any daemon using the active AGH_HOME;
# daemon web routes redirect to Vite, and Air owns every later rebuild/restart.
.PHONY: start stop restart dev dev-daemon

dev: codegen
	@AIR_VERSION="$(AIR_VERSION)" bash scripts/dev.sh

dev-daemon:
	@bash scripts/dev-daemon-runner.sh "$(AIR_VERSION)" -c .air.toml

start: build web-build
	@test -x ./bin/agh || { echo "bin/agh not found — run 'make build' first"; exit 1; }
	@echo "Starting daemon serving local web bundle: $(CURDIR)/web/dist"
	@AGH_WEB_DIST_DIR="$(CURDIR)/web/dist" ./bin/agh daemon start

stop:
	@./bin/agh daemon stop

restart:
	@$(MAKE) stop || true
	@$(MAKE) start
