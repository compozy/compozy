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
- The follow-up CI exposed a terminal catalog race: an overlapping REST read
  could overwrite a live exit event with an older running row. Cancel the exact
  catalog query before each authoritative stream write. The existing catalog
  suite reproduces the race for concrete and aggregate profiles while proving
  that another workspace's read completes normally.

## Change impact

Following `docs/_memory/change-impact.md`: Web changes are confined to the
pending-reply timer, terminal catalog cache fencing, and navigation-test synchronization. Native tools,
CLI/HTTP/UDS contracts, extensions, hooks, config, persisted workspace data,
and `skills/compozy/` are unaffected: no identifiers, payloads, permissions,
data ownership, or command semantics change. RT-054 owns the visible indicator
regression; ET-terminal-stream-resilience owns the terminal exit race. No site
documentation changes are needed.

## Validation

- Existing runtime-provider suite: 58 tests passed through root Turborepo.
- Existing SessionThread suite: 130 tests passed through root Turborepo,
  including the delayed-effect regression.
- The delayed-effect test fails with the original timer branch and passes with
  the fix. The two catalog race cases likewise fail before the production fix;
  all 25 catalog stream tests pass afterward through root Turborepo.
- CI run 34141218757 confirms the original failures are repaired. Every lane
  except Web E2E shard 4 passed; its terminal-agent E2E-003 failure exposed the
  catalog race above. The E2E assertions remain unchanged.
- Full-checkptr run 34141223686 passed all eight shards, including all 392
  global database top-level tests.
- `make gate` passed on the terminal fix: 771 Web test files and 7,104 tests,
  plus lint, typecheck, and generated-artifact checks. React Doctor scored 100.
- [CI run 34144138424](https://github.com/compozy/compozy/actions/runs/34144138424)
  passed every lane on `52d2c4a63`, including all eight Go race shards, Windows,
  Darwin, Frontend, Desktop, runtime E2E, and all four Web E2E shards. Terminal
  E2E-003 passed unchanged; its shard finished with 66 passing tests. React Doctor
  and Release also passed on that commit.
