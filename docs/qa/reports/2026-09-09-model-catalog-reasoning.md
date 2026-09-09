# Model catalog and reasoning validation

## Scope

As Sol, open a session, choose the current Codex model and its advertised effort, switch to Cursor Grok, and send a bounded request. Use the isolated native-provider lab and preserve operator authentication. Tour: configuration changes. Time box: 60 minutes.

| Scenario | Status | Evidence |
| --- | --- | --- |
| RT-model-catalog-cold-open: native model discovery and refreshed capabilities | Pass | Fresh onboarding; native Codex and Cursor payloads; three complete OpenCode refreshes |
| ET-web-runtime-selector-minimal-slider: Astra and Grok effort selection | Pass | Astra Ultra, Luna Max, Grok 4.5 High and Grok 4.6 Extra high; persisted selection after restart/reload |
| RT-cursor-agent-mode: selected runtime executes a prompt | Pass | Real native Cursor prompt created catalog-proof.txt with exact expected bytes; unsupported Ultra rejected with 400 |

## Environment

Manifest: `/Users/pedronauck/dev/qa-labs/compozy-model-catalog-reasoning-20260909-154001-298064-lab/qa-artifacts/qa/bootstrap-manifest.json`. Production daemon on 2123 was read only; this walk used isolated Compozy state. The manifest teardown completed with `clean: true`; its `qa/teardown.json` retains the process cleanup record.

## Findings and fixes

- Codex ACP discovery retained model names but lost per-model reasoning metadata. Native `model/list` now supplies the advertised levels, defaults, visibility, and pages.
- Cursor launch bindings carried reasoning but merge discarded the levels because the provider does not apply them through an ACP option. Preserve binding-backed levels and defaults.
- The closed effort vocabulary rejected `ultra` and future provider values. Keep provider identifiers as strings through generated contracts, storage and selectors, retaining the SDK ReasoningEffort export. Native runtime membership remains enforced.
- OpenCode needed verbose metadata; an intermittent truncated CLI pipe payload required complete stdout capture at the subprocess boundary. A bounded regular-file read now captures the full result and cleanup is joined. Three consecutive live refreshes returned 533 rows and Grok 4.6 low/medium/high/xhigh.
- OpenCode Zen models.dev enrichment now carries the native opencode/model namespace.
- Session hydration preserves ACP options; rapid selection writes are serialized with the acknowledged revision.

## Session debriefs

Session `sess-e0d9da4d062c93d0`, workspace `ws_7d3d7909f2aa08a9`:

- Cursor Grok 4.6 xhigh completed the bounded native file-writing prompt. Independent byte comparison found `MODEL_CATALOG_OK\n`.
- An invalid Cursor Ultra request returned HTTP 400 and left the saved xhigh selection unchanged.
- Switching the same session to Codex Astra Ultra completed the no-tools prompt with `ASTRA_ULTRA_OK`. The ACP acknowledgement explicitly reported model gpt-6-astra and reasoning_effort ultra, alongside its six advertised values.
- Reloading after the isolated daemon restart preserved Astra Ultra.
- OpenCode Grok 4.6 Extra high completed the no-tools prompt with `OPENCODE_REASONING_OK`. The ACP acknowledgement reported `model=openrouter/x-ai/grok-4.6` and `effort=xhigh`, confirming application of the advertised variant.

Screenshots: `docs/qa/evidence/2026-09-09-model-catalog-reasoning/`. Native API evidence and provider attempts: the manifest's `qa-artifacts/qa/evidence/` and `provider-attempt.json`.

## Validation boundaries

The original pasted queue 400, session/presence 404 and window-manager 409 lacked response bodies. Production logs were inspected read-only and showed session removal/cancellation; the catalog fix does not establish the cause of every historical lifecycle error. The current native journey completed with persisted runtime selection and a typed invalid-combination error.

Initial full Web iteration: 7104 passed, two failed. The effort-position regression was fixed in production; the unchanged history-anchor case and selector both passed the focused root Turbo rerun (156 tests). The final root Turbo Web lane (`TURBO_CONCURRENCY=2 bunx turbo run lint typecheck test --filter=./web`) passed 7,106 tests across 771 files, typecheck, and lint with zero warnings/errors. The latest affected Go gate passed codegen-check and Go lint (zero issues), and the previously failing OpenAPI and SDK tests passed. The user explicitly requested publishing the PR and moving the remaining full gates to CI; the running local gate was stopped accordingly. A preceding run had already passed globaldb (1,670.9 seconds), session, and the other Go suites, with only the subsequently corrected contract expectations failing.


Production Web build: `bunx turbo run build --filter=./web` passed all four tasks. Vite reported CSS optimizer warnings for the unchanged session-search `::highlight` rules, chunk sizes above 500 kB, and plugin timings. These are recorded separately from the lint/typecheck/test gate. `GOOS=windows GOARCH=amd64 go build ./internal/modelcatalog` also passed.

## Final status

Functional QA PASS. Full CI gates pending on the PR by explicit user direction. No claim of a complete local gate pass or deployment to the production daemon is made.
