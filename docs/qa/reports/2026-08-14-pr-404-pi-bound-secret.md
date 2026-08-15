# QA Run Report — 2026-08-14 — PR #404 Pi bound-secret interpolation

- **Scope:** Pi ACP bound-secret materialization and one ordinary streamed session prompt.
- **Cadence tier:** targeted
- **Build:** `fix/pi-models-json-apikey-env-interpolation` working tree
- **Environment:** isolated Compozy home, isolated provider home, real `pi-acp`, and a local OpenAI-compatible provider boundary.
- **Status:** closed

## Persona and Flow

Théo used the public CLI to inspect provider authentication, create a session, and stream a prompt through `J-13` / `RT-018`.

## Result

| Charter | Journey / Scenario | Surface | Status |
|---|---|---|---|
| Targeted Pi credential walk | J-13 / RT-018 | CLI + runtime + provider | Pass |

The setup stored a sentinel in the Compozy vault, restarted for the provider config lifecycle, and created a fresh session.

Session `sess-a4c892b888bb53e5` streamed `PI_ENV_INTERPOLATION_OK`. Its generated Pi `models.json` contained `"apiKey": "$QA_PI_API_KEY"`. The provider boundary accepted the request only when the Authorization header carried the resolved sentinel value and retained only the boolean match result.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-pi-bound-secret-response-20260814-235531-199048-lab/qa-artifacts/qa/evidence/pi-bound-secret/materialized-models.json`
- `/Users/pedronauck/dev/qa-labs/compozy-pi-bound-secret-response-20260814-235531-199048-lab/qa-artifacts/qa/evidence/pi-bound-secret/provider-proof.json`
- `/Users/pedronauck/dev/qa-labs/compozy-pi-bound-secret-response-20260814-235531-199048-lab/qa-artifacts/qa/evidence/pi-bound-secret/prompt-stream.jsonl`
- `/Users/pedronauck/dev/qa-labs/compozy-pi-bound-secret-response-20260814-235531-199048-lab/qa-artifacts/qa/evidence/final-make-verify.log`

## Final Status

- **Verdict:** PASS
- **User-visible failures:** none
- **Teardown:** `/Users/pedronauck/dev/qa-labs/compozy-pi-bound-secret-response-20260814-235531-199048-lab/qa-artifacts/qa/teardown.json` (`clean: true`)
