# QA Run Report — 2026-09-14 — Marketplace review public journeys

- **Scope:** public CLI/HTTP follow-up for the accepted PR636 review corrections; one review round remains the only review round.
- **Cadence tier:** targeted.
- **Build:** `0f77945a70e3f081296ccd772b93189357b6d45e`; isolated daemon compiled for these journeys. Delivery gates run in GitHub CI.
- **Environment:** fresh bootstrap lab; local authored packages and synthetic credentials. No operator workspace or credentials are mutated.
- **Started:** 2026-09-14T08:08:45.342453+00:00; **Status:** in-progress.

## Personas

Bruno is an operator maintaining separate workspaces and profiles through CLI and documented HTTP endpoints. His intent is to keep installed tools available while changing sources, development packages and profile names.

## Flows in Scope

The bounded retests extend existing charters for Marketplace source management, plugin development, skill discovery, MCP authorization and profile lifecycle. Existing screenshots remain historical visual evidence. Current-head CI owns full Web/Desktop E2E and the injected database-failure, encrypted-storage and subprocess integration checks.

## Session Matrix & Results

| # | Charter | Scenario | Persona | Tour | Status | Issue | Fix commit |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | CH-marketplace-public-ownership | ET-api-marketplace-sources | Bruno | Data | Pass | | |
| 2 | CH-marketplace-public-ownership | ET-agent-plugin-dev-reload | Bruno | Data | Pending | BUG-20260914-agent-plugin-dev-rejected | |
| 3 | CH-marketplace-public-ownership | RT-agent-hot-discovery-skill-isolation | Bruno | Data | Pending | | |
| 4 | CH-marketplace-public-ownership | ET-profile-cli-lifecycle | Bruno | Data | Pending | BUG-20260914-profile-rename-mcp-reference | |
| 5 | CH-marketplace-public-ownership | ET-cli-mcp-authorize | Bruno | Data | Pending | | |

## Session Debriefs

### Source maintenance — Bruno — 08:13–08:17 UTC

Created the local `studio` source through CLI and read the same row over HTTP. Duplicate registration, an invalid toggle payload, protected-source removal and an unsupported source scheme returned their structured rejections (CLI duplicate exit 2; HTTP 400/403/422). The documented config.toml retained exact bytes and its comment; the separately saved 11-minute refresh TTL survived. Disabled, re-enabled and removed `studio`; the installed `desk-notes` package retained its version, enabled state and exact provenance. Fresh CLI/HTTP reads after daemon restart confirmed both retained installation and removed source. Evidence: steps 012–044.

The public walk covers operator-reachable rejection paths. Injected database transaction failures remain verified by the real SQLite CI suite, not attributed to this walk. The initial `/api/v1` probe was a runner URL assumption corrected against the documented `/api` route before the source walk. CLI `config set marketplace.ttl` is unsupported; the documented Settings HTTP endpoint performed the edit. No product defect was filed for either setup detour.


### Plugin development — Bruno — 08:25 UTC

Validation accepted the Claude package, but CLI development rejected it before linking (steps 055–056). The leg ended and BUG-20260914-agent-plugin-dev-rejected was filed. Repair and a fresh public re-walk remain pending.

### Development recovery — Bruno — 08:29–08:30 UTC

After 685af5f3c, CLI dev linked the Claude package from inside the design workspace, reload changed only that instance from 1.0.0 to 1.1.0, and HTTP confirmed the same version. Restart retained both the dev generation and the other workspace's published 1.0.0 installation. Removing the dev link restored the published installation in design. Steps 069–085 verify BUG-20260914-agent-plugin-dev-rejected; the additional hook-placement leg remains pending.

### Profile credentials — Bruno — 08:31–08:32 UTC

Settings wrote synthetic manual MCP client secrets independently in user, profile and workspace scopes. A shared Vault ref remained independently owned. The CLI profile rename moved the owned Vault row and left shared/user/workspace rows intact, but MCP config retained the old profile ref and public reads failed. Filed BUG-20260914-profile-rename-mcp-reference; the leg ended before delete. The workspace scope selector writes the base workspace sidecar even with a profile query; it does not author workspace-profile files. Extension Settings overrides reject auth changes, so extension OAuth setup must use its documented auth acquisition path rather than treating overrides as an authoring surface.

## What Was Fixed

Review corrections are already implemented through `0f77945a7`; this run verifies their public effects. No new product fix has been made in this QA session.

## Paper Cuts

Not assessed yet.

## Runtime Errors Observed

Not assessed yet.

## Verification and Evidence

The bootstrap manifest and live session state are tracked in `.deep-review/pr-636-luna-round-1/public-qa-state.json`. Durable public transcripts will be stored under `docs/qa/evidence/2026-09-14-marketplace-review-public/`. Database-failure injection and direct ciphertext checks remain integration evidence, not public-walk evidence.

## Decisions for a Human

None identified during preparation.

## Final Status

Pending the five public journeys, strict lab audit, teardown, and current-head CI.
