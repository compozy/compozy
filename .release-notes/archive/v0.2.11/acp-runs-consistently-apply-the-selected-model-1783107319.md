---
title: ACP runs consistently apply the selected model
type: fix
---

ACP-backed runs now apply the selected model reliably for both newly created and resumed sessions. Previously the chosen model could fail to take effect on some session paths, so an agent could run against the wrong model.

### What changed

- The model is trimmed and validated, then **set before the first prompt turn** for both new and resumed runs, so the very first turn already uses the intended model.
- When switching the model on a session fails, Compozy now performs a best-effort session cleanup instead of leaving a half-configured session around, reducing spurious retry, cancellation, and timeout errors on subsequent turns.

This makes `--model` (and the resolved workspace default) behave consistently across every ACP runtime, whether a run is started fresh or resumed.
