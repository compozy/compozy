# CI recovery

The main-branch CI run 34067355627 failed in the session runtime-provider test
and the Home navigation journey. The scheduled checkptr audit 34083529448 also
failed because Bun was absent and the unsplit global database suite exceeded
its package timeout.

## Corrections

- Always schedule the thinking guard's clock update, including when commit
  work has already consumed its 250 ms delay. Previously that branch returned
  without publishing elapsed time, leaving pending activity hidden.
- Home navigation now proves destination-window focus after each link before
  clicking the Home Dock launcher. A URL change expresses navigation intent;
  it does not prove the window manager has acknowledged focus. Existing URL,
  visibility, artifact, and transport assertions remain in place.
- The full-checkptr audit installs the same Bun dependencies as the regular
  race lane and uses its existing eight-way partitioner, including the global
  database test split. Race/checkptr instrumentation and timeouts are retained.

## Change impact

Following `docs/_memory/change-impact.md`: Web changes are confined to the
pending-reply timer and navigation-test synchronization. Native tools,
CLI/HTTP/UDS contracts, extensions, hooks, config, persisted workspace data,
and `skills/compozy/` are unaffected: no identifiers, payloads, permissions,
data ownership, or command semantics change. RT-054 owns the visible indicator
regression; no site documentation changes are needed.

## Validation

- Existing runtime-provider suite: 58 tests passed through root Turborepo.
- Existing SessionThread suite: 130 tests passed through root Turborepo,
  including the delayed-effect regression.
- Delivery gate and current-head remote CI are required before completion.
