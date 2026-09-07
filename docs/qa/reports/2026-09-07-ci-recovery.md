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
- Reuse structurally equal branches when publishing the desktop projection.
  Moving one window no longer rerenders subscribers of unchanged windows. Keep
  the root publication signal, command behavior, and all projected values.

## Change impact

Following `docs/_memory/change-impact.md`: Web changes are confined to the
pending-reply timer, terminal catalog cache fencing, desktop projection sharing,
and navigation-test synchronization. Native tools,
CLI/HTTP/UDS contracts, extensions, hooks, config, persisted workspace data,
and `skills/compozy/` are unaffected: no identifiers, payloads, permissions,
data ownership, or command semantics change. RT-054 owns the visible indicator
regression; ET-terminal-stream-resilience owns the terminal exit race. No site
documentation changes are needed. ET-web-window-routing-lifecycle owns the
12-window performance canary.

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

## Desktop projection performance follow-up

The notes-only commit's first CI attempt passed 91 of 92 Web shard 2 cases, but
E2E-023 recorded a 77 ms peer-convergence task against its unchanged 50 ms ceiling.
Restore took 197.1 ms (budget 500 ms), and drag had no task over 50 ms. Ten isolated
normal repetitions on the original code passed, so a normal-speed local pass
alone did not explain the Linux failure.

Two diagnostic captures with Chromium CPU throttled to 4x reproduced a 51 ms
peer task. CPU samples attributed 40.85–46.92 ms of inclusive work to React.
The runtime rebuilt every derived window/frame object for one authoritative
move, invalidating otherwise unchanged selectors. Opportunity score:
impact 4 × confidence 4 / effort 1 = 16. The selected change uses the existing
TanStack structural-sharing helper at the projection publication boundary.

The canonical runtime suite proves that the moved window updates while an
unchanged window subscriber does not render. Its original 40 cases remain intact;
all 41 pass. A complete runtime-view capture from the same fixture has identical
canonical JSON before and after, SHA-256
`c776f644c826fea83c50c1d4f1a8d1986836f6841c3ac57da91eda9a45276620`.
Array order, tie-breaking inputs, numeric geometry, commands, and error values
are unchanged; only equal branch identities are reused. RNG is not involved.

Three corrected diagnostic repetitions at the same 4x CPU setting passed with no
long task over 50 ms. Inclusive React samples across the six peer captures ranged
from 24.18 to 30.39 ms. These are local diagnostic samples, not a claim about total
CI duration. Instrumentation stayed in an isolated checkout; the canonical E2E
scenario and all timing ceilings remain unchanged.

The final normal-speed E2E scenario passed ten repetitions: restore 51.4–61.1 ms,
no drag or peer-convergence task above 50 ms, and authority/geometry assertions
passed throughout. Assets were built through `bunx turbo run build --filter=./web`;
the unchanged Go daemon was built once with `go build` and supplied through the
fixture's existing `COMPOZY_TEST_DAEMON_BIN` seam. The focused command was
`CI=true COMPOZY_TEST_DAEMON_BIN=/tmp/compozy-ci-perf-daemon-d829 bun run --cwd web test:e2e:daemon-served:raw __tests__/os-shell.spec.ts --grep 'E2E-023: the 12-window envelope' --repeat-each 10`.
