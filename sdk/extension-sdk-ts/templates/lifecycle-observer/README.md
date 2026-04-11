# Lifecycle Observer

Observes the run lifecycle with the `run.post_shutdown` hook and writes optional diagnostic records when `COMPOZY_TS_RECORD_PATH` is set.

## Scripts

- `npm test` compiles the project and runs the template test with the SDK test harness.
- `npm run build` emits `dist/` for use as the extension subprocess entrypoint.
