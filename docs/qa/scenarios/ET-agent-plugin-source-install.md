---
id: ET-agent-plugin-source-install
area: ET
title: Install an Agent Plugins package from a directory or git source
persona: Bruno
journey: J-extension-distribution
expected: The same schema-1.0.0 package installs from a local directory and a git ref without a format flag, reports `format: agent-plugin` plus its ingested and skipped components, and returns the installed identity through the canonical status read after normal unverified-source consent.
entry_points: compozy extension install <local-path>|git:<url>@<ref> --allow-unverified --yes; POST /api/extensions over HTTP and UDS; compozy__extensions_install; https://compozy.com/docs/extensions/install
qa_status: pass
bug_ids: BUG-20261107-derived-secret-env-enable-fails
fix_status: fixed
retest_status: pass
fix_commits: 35100d40b55c
evidence: /tmp/mpc-shots/final/cli-client-plugin-enable.txt
last_report: docs/qa/reports/2026-08-16-agent-plugins.md
overlaps: ET-extension-published-source-installs; ET-web-extension-union-install
---

QA impact 2026-08-16: Agent Plugins became a third detected package layout on the existing extension
source union. Task 08 must acquire one fixture from a directory and a real git ref, compare the
structured payloads, and prove no format-selection flag or separate lifecycle appears.

Task 08 evidence must pair each mutation with a fresh `status` read, but the diagnostic/read-plane
contract belongs to `ET-agent-plugin-degraded-inventory` to avoid duplicating that invariant.

QA 2026-08-16: local-directory and pinned-git acquisition of the same upstream example bytes both
resolved to `format: agent-plugin` without a selector flag and converged on the same lifecycle/read
contract after normal unverified-source consent.

## QA impact 2026-11-07 — marketplace plugin catalog (spec `marketplace-plugin-catalog`, S8)

Client-plugin compatibility (US-008–US-011; F6). Use a local fixture in the Cursor or Claude
layout whose manifest omits `mcpServers` and ships a root `.mcp.json` with a server that uses
`${CLAUDE_PLUGIN_ROOT}`, an extra field such as `placement`, and `${NAME}` references (one secret
like `DEMO_API_KEY`, one plain `DEMO_HOST`, one `${OPTIONAL:-x}`).

1. `compozy extension validate <dir> -o json` reports `client_mcp_discovered`,
   `client_field_ignored` for `placement`, and the derived inputs with secret/required flags.
2. `compozy extension install <dir> --allow-unverified --yes --enable` without values fails with
   `extension_inputs_required` naming `DEMO_API_KEY` and `DEMO_HOST` (not `OPTIONAL`).
3. Re-run with `--input` values: the server is loaded, the secret is stored as a vault ref and
   never appears in `compozy extension status` output or daemon logs.
4. Serve the same fixture from a local marketplace source whose `marketplace.json` uses a
   `git-subdir` or `url` entry (or `.agents/plugins/marketplace.json`) and verify it lists and
   installs through Marketplace Add with the review dialog collecting the inputs.

### Walk result 2026-11-07 (S8 integrated lab, `feat/mpc-s8` with S1–S7 + S9)

Isolated lab (own home, ports 49760/49761, worktree web dev server), default sources plus a local `lab-plugins` folder source; cold refresh of the default sources took ~65.7 s (compozy-catalog 18, claude-plugins-official 316, compozy-plugins 333). Screenshots and CLI captures are lab-local in `/tmp/mpc-shots/s8/` (not committed).

- PASS — `compozy extension validate` loads the root `.mcp.json` and drops `placement` (both shown as
  issues; the codes `client_mcp_discovered`/`client_field_ignored` and the derived inputs are not
  printed, and the root-file notice is a warn, unlike `_dx.md`).
- PASS — install without values fails with `extension_inputs_required` naming DEMO_API_KEY and
  DEMO_HOST, not the defaulted DEMO_MODE.
- PASS — the secret value never appears in extension status output or the daemon log.
- **FAIL** — `--enable` with values installs but cannot turn on
  (BUG-20261107-derived-secret-env-enable-fails); the same package with a non-secret `env` name is
  active and healthy.
- PASS — a third-party source's `x-compozy` (featured, category security) is ignored: `promo` lists
  under Design, not featured.

### Re-walk 2026-11-07 (integration 30fcdfee8, fresh isolated lab)

Cold refresh of the default sources took ~87.2 s (compozy-catalog 18 from a generator-regenerated temp feed, claude-plugins-official 316, compozy-plugins 333). Evidence is lab-local in `/tmp/mpc-shots/final/` (not committed).

PASS — the `.mcp.json` fixture with a derived secret in `env` installs with `--enable` and values:
Enabled true, State active, Health healthy; the secret value is not in status output or the daemon log.
