# Actual-run report: opencode-model-discovery live verification (Task 05 / V-001)

Date: 2026-10-18. Build: implementation diff Tasks 01–03 (`compozy` built
16:15Z). Native: installed `opencode v2.0.32`, authorized operator HOME.
Lab: `compozy-opencode-model-discovery-20261018-161510-876010-lab`
(isolated home/ports/sockets; models.dev disabled; `BROWSER_MODE=blocked`).
Evidence: `.compozy/tasks/opencode-model-discovery/evidence/task_05/evidence.md`
+ lab `qa-artifacts/qa/task05-live-legs.log` (redacted).

## What passed live

- **v2 cold discovery + first/restarted prompts (L1):** `provider_live:opencode`
  `succeeded` with 92 live rows. `opencode/beam-free` + effort `low` completed
  `ok`, then after `session stop` completed `done` on a fresh bind with the
  same effective runtime (`prompt_runtime` agreement via CLI read paths).
- **Refresh / failure / recovery (L3):** forced refresh `succeeded` (92 fresh);
  1s-timeout forced refresh failed `timeout` with prior rows stale and redacted
  diagnostics; unset + refresh recovered to 92 fresh.
- **No-fallback (L4a):** bogus explicit discovery command → `refresh_failed`,
  no native fallback process spawned; recovery clean.
- **Partition (L3):** explicit second-install selector (lab symlink, same
  v2.0.32) refreshed 92 rows under the new fingerprint and launched ACP from
  the lab-copy path; stop reaped it.
- **Service lifetime (L5):** owned ACP processes exited on session stop; the
  pre-existing shared `serve --service` stayed intact. Waiter-kill mid-refresh
  left the daemon responsive with consistent state and no orphans.
- **Cross-surface (L6):** CLI, HTTP (`/api/model-catalog/...`), UDS, and
  `compozy__provider_models_*` agree on exact `opencode/*` IDs with source
  `provider_live:opencode`.

## Explicitly unverified / blocked (not passes)

- **L2 v1 first/restarted prompt — unverified:** no pinned v1 binary on this
  box. (npm-fetched v1.19.3 dispatch was observed during L4b, which is not a
  pinned-v1-leg pass.)
- **L4 npm end-to-end — blocked-external:** with native absent, discovery
  correctly selected the npm prefix (`npm exec opencode-ai@latest --version`
  observed), acquired the package (v1.19.3 resolved), and dispatched v1
  discovery — but `models --verbose` fails inside the authorized operator HOME
  because v1.19.3 rejects the v2-created global `opencode.db` ("Database is not
  empty and has no session table"; read-only check: 20 tables, no `session`
  table). With an empty HOME it lists models, but that drops the authorized
  auth and proves nothing about live compatibility. Operator native state is
  untouchable, so no workaround was attempted. Product behavior at the boundary
  met the contract: truthful `refresh_failed`, redacted diagnostics, no silent
  fallback, no orphan process.
- **Browser picker flows — unverified:** neither browser-use nor agent-browser
  was available; prompts/selectors verified via CLI equivalents only.
- **Windows/macOS — unverified:** Linux-only run; no cross-platform inference.

## Defects

None filed. No product defect reproduced; no repair; no re-walk. The npm-leg
blocker is vendor/operator-state behavior with contract-correct product
handling, not an implementation defect.

## Teardown

Manifest teardown will report `clean: true`; no owned process remains and the
operator shared service was never restarted or disturbed. Lab config overrides
(timeout, explicit commands) were all unset and the final lab state re-verified
green (92 fresh live rows) before teardown.

## Selector follow-up — 2026-10-18

The operator's `make dev` reproduction exposed a presentation defect missed by the original
fresh-config fixture: discovery succeeded with 92 live v2.0.32 rows, but an explicit curated set
contained only the deprecated `opencode/muse-spark-1.3-contributor-free` model. Every row in the
complete projection was therefore `curated=false`, and the provider chip incorrectly applied the
curated-only browsing filter. Refreshing could not repair that presentation filter.

The shared selector now keeps curated browsing in All models while a provider chip, like search,
shows its full catalog. No operator configuration, credentials, saved selection, curation, or
discovery state was rewritten to fix the symptom. The change is version-independent.

- Component regression: the new empty/mixed-curation cases failed before the production fix;
  all 91 existing and new selector tests passed afterwards through root Turbo.
- Live browser replay against `make dev`: opened the existing session selector and clicked
  OpenCode with an empty search. Chromium rendered 174 catalog rows, including every one of the
  92 `available=true` model IDs returned by the same HTTP catalog (`missingLive=[]`), and no empty
  state. Screenshot: `/tmp/opencode/opencode-model-selector-fixed.png`. This was a read-only picker
  walk; no new vendor inference or session-runtime mutation was performed. All 92 live rows were
  enabled; Home/End reached both list edges and Escape restored trigger focus.
- Daemon-served `session-provider-override.spec.ts`: **4/4 passed**, including the OpenCode v1→v2
  journey with retired curation, empty-search selection, first/restarted prompts, and CLI/HTTP/UDS
  runtime agreement. The isolated fixture holds an interrupted turn open, confirms the daemon is
  stopped before sending again, and locates the composer across active/stopped accessible labels.
  Focused OpenCode replay: **1/1 passed**. Logs: `/tmp/opencode/opencode-selector-e2e{,-full}.log`.
- Race-enabled `internal/modelcatalog`, `internal/providerexec`, and `internal/session` suites
  passed, as did `TestManagerIntegrationOpenCodeInvocation` with integration tags and the race
  detector. The v1 parser and native discovery behavior remain unchanged. React Doctor reported
  100/100 with no issues in changed frontend files.
- Root Turbo Web build/typecheck and site typecheck plus **258 site tests** passed. Two unrelated
  Web-suite tests timed out during concurrent load; both passed their isolated root-Turbo replay.
- Broader validation found two defects left by the earlier implementation: an E2E assertion used
  the removed `resumeButton` selector, and site prose exposed OpenCode's native model API as an
  ambiguous local API path. The assertion now waits for the existing stop control to disappear;
  the prose explicitly identifies and links OpenCode's API without weakening either check.
- The project-pinned Go 1.27.1 avoids the ambient 1.27.2 linter export-data failure. Six subsequent
  lint findings in the earlier implementation were corrected without changing semantics: shared
  diagnostic text, version-parser early return, unchanged Unicode path spelling, an unused callback
  parameter, and extraction of the existing replay-preparation block from launch orchestration.

The original v1-native/npm/cross-platform limits above remain; a live v2 picker replay does not
prove native v1 compatibility or a fresh vendor prompt/restart through the browser.

## OpenCode provider labels — 2026-10-18

OpenCode model rows now show the native provider prefix after the model name in muted text.
Its catalog groups sort by native provider then display name ascending, with absent provider
text omitted safely. Other CLI labels/ranking, pinned recents/favorites, and exact IDs are unchanged.

- Existing selector suite: **104/104 passed**; nullable/blank identities, sorting, selection, and
  Codex/Claude/Antigravity isolation are covered. Root Turbo typecheck/build passed.
- Daemon-served provider-override suite: **4/4 passed**, including v1/v2 provider-label assertions
  and ascending native-provider order. Log: `/tmp/opencode/opencode-provider-label-e2e.log`.
- Wide/narrow picker inspection: **pass**, 1280×720 and 390×844; no row wrapping or picker overflow.
  Captures: `.impeccable/review/opencode-provider-{desktop,mobile}.png`.
- React Doctor: **100/100**. No discovery, backend contract, or vendor-inference behavior changed.
