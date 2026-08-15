# Terminal Loop Effect Stream Lifecycle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development
> (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use
> checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver terminal `effect_results` frames to the mounted Loop run page after a terminal
`status_changed` frame, including during retained replay.

**Architecture:** Keep the existing Loop SSE subscription page-owned instead of treating terminal
run status as an end-of-stream marker. The React effect cleanup remains the sole source-closing
owner; all parsing, named-event routing, invalidation, stale-subscription fencing, and native
`EventSource` reconnect behavior remain unchanged.

**Tech Stack:** React 19, TypeScript, native `EventSource`, TanStack Query, XState Store, Vitest,
Testing Library, Turborepo, Playwright.

## Global Constraints

- Fix [compozy/compozy#405](https://github.com/compozy/compozy/issues/405) independently from
  daemon authorization issue #403.
- Do not add timers, grace periods, effect counters, Loop-definition inspection, or a new SSE
  completion event.
- Do not change API, SSE payload, ToolID, configuration, or extension schemas.
- Preserve the workspace/run identity fence and stale-subscription rejection.
- Run frontend validation through Turborepo from the repository root.
- Follow strict RED/GREEN: the regression must fail for the confirmed terminal-close cause before
  production code changes.
- Persistent artifacts, source, tests, commits, issues, and PR text are English.
- Run isolated combined visual QA with the #403 daemon fix before opening either PR.

## Compozy Impact Audit

- **Native tools:** No impact. The change does not alter `compozy__*` IDs, toolsets, descriptors,
  input/output schemas, risk metadata, capability gates, or CLI/API fallbacks; it only changes how
  the Web consumer owns an existing Loop SSE subscription.
- **Extensibility and hooks:** No impact. Loop effect execution, `effect_results` production,
  extension tools, hook dispatch, skills/capabilities, registries, bridge SDKs, MCP sidecars, and
  config lifecycle are unchanged. The Web already understands the existing event kind and payload.
- **Workspace data isolation:** No impact. The existing workspace-scoped stream URL, query keys,
  `workspace_id`/`loop_run_id` frame checks, active-subscription identity fence, and sequence
  deduplication remain unchanged. The regression suite must keep covering replacement and stale
  subscription rejection.
- **Official Compozy skill:** No impact. The bundled skill already documents the long-lived Loop
  event stream and `effect_results`; no public tool, CLI, hook, extension, or Loop semantic changes.

---

### Task 1: Preserve post-terminal event delivery

**Files:**

- Modify: `web/src/systems/loops/hooks/__tests__/use-loop-stream.test.tsx`
- Modify: `web/src/systems/loops/hooks/use-loop-stream.ts`
- Modify: `web/src/systems/os/apps/loops/use-loop-run-page.ts`

**Interfaces:**

- Consumes: `LoopStreamEventSource`, `LoopRunEventFrame`, `LoopStreamSubscription`, and the
  existing `attachLoopStreamSource` cleanup contract.
- Produces: `useLoopStream(workspaceId, runId, options)` with page-owned source lifetime and no
  public type changes.

- [ ] **Step 1: Make the fake source honor closure**

  Change `FakeLoopStreamEventSource.close` to set a private `closed` flag and make
  `emitMessage`, `emitNamed`, and `emitError` ignore frames after closure. Keep `close` as a Vitest
  spy so cleanup cardinality remains observable.

  ```ts
  private closed = false;
  public close = vi.fn(() => {
    this.closed = true;
  });
  ```

- [ ] **Step 2: Write the failing regression**

  Replace `Should deliver a terminal status frame before closing its replay source` with
  `Should keep the source open for effect results after terminal status until cleanup`.
  Emit these hand-authored frames in order:

  ```ts
  const terminal = buildFrame({
    id: "loopevt_terminal",
    seq: 42,
    kind: "status_changed",
    payload: { status: "done" },
  });
  const effectResult = buildFrame({
    id: "loopevt_effect",
    seq: 43,
    kind: "effect_results",
    payload: {
      delivery_id: "delivery_1",
      trigger: "on_done",
      outcome: "ok",
      code: "",
      cause: "",
    },
  });
  ```

  Assert `onEvent` receives `[terminal, effectResult]` in that order, the source has not closed
  before unmount, and unmount closes it exactly once. Keep the existing event-kind fixture unchanged;
  the regression directly exercises the separately retained `effect_results` listener.

- [ ] **Step 3: Run the focused test and verify RED**

  Run from the repository root:

  ```bash
  bunx turbo run test --filter=./web -- src/systems/loops/hooks/__tests__/use-loop-stream.test.tsx
  ```

  Expected: FAIL because the current terminal branch closes the fake source after the first frame,
  so `effectResult` is not delivered and closure occurs before unmount.

- [ ] **Step 4: Implement the minimal root fix**

  In `use-loop-stream.ts`, remove the terminal-status close branch, delete the now-unused
  `isTerminalLoopStatus` import, and delete `isTerminalStatusFrame`. Update the hook docstring to
  state that source ownership lasts until the React effect cleanup. Do not change event parsing,
  invalidation, lifecycle-store opening, error handling, or detach behavior. Update the stale
  `use-loop-run-page.ts` comment that says the hook closes on the replayed terminal frame.

- [ ] **Step 5: Run the focused test and verify GREEN**

  ```bash
  bunx turbo run test --filter=./web -- src/systems/loops/hooks/__tests__/use-loop-stream.test.tsx
  ```

  Expected: PASS with the terminal and effect-result frames delivered in order and the source
  closed only by unmount.

- [ ] **Step 6: Run focused formatting, type, and lint validation**

  ```bash
  make bun-lint
  bunx turbo run typecheck --filter=./web
  git diff --check
  ```

  Expected: all commands exit 0 with no warnings.

- [ ] **Step 7: Commit the implementation batch**

  ```bash
  git add web/src/systems/loops/hooks/__tests__/use-loop-stream.test.tsx \
    web/src/systems/loops/hooks/use-loop-stream.ts \
    web/src/systems/os/apps/loops/use-loop-run-page.ts
  git commit -m "fix: keep loop streams open for terminal effects"
  ```

---

### Task 2: Verify the Web branch broadly

**Files:**

- Verify only; no source files should change.

**Interfaces:**

- Consumes: the complete Web workspace test, typecheck, lint, and production build graphs.
- Produces: fresh branch-level evidence that the transport lifecycle change does not regress other
  Web systems.

- [ ] **Step 1: Run required frontend gates**

  ```bash
  make bun-lint
  make bun-typecheck
  make bun-test
  make web-build
  ```

  Expected: every command exits 0; lint reports zero warnings.

- [ ] **Step 2: Run the canonical Web E2E lane**

  Retry the repository-pinned browser installation once if the earlier CDN timeout is no longer
  present, then run:

  ```bash
  bash scripts/worktree.sh bootstrap --e2e --skip-install
  COMPOZY_TEST_DAEMON_BIN="$PWD/bin/compozy" make test-e2e-web
  ```

  Expected: the browser installation and E2E lane exit 0. If the CDN remains unavailable, record
  the exact external failure and use `/usr/local/bin/chromium` only for the manual isolated QA;
  do not claim the canonical E2E lane passed.

---

### Task 3: Verify both independent fixes in one isolated visual walk

**Files:**

- Verify in a temporary integration worktree; do not commit its merge result.
- Evidence: fresh `eng-qa-bootstrap` lab under `~/dev/qa-labs/`.

**Interfaces:**

- Consumes: committed heads of `fix/loop-effect-tool-policy` (#403) and
  `fix/loop-terminal-effect-stream` (#405).
- Produces: structured and visual proof that daemon delivery and Web presentation work together
  without coupling the two PRs.

- [ ] **Step 1: Create a temporary combined worktree**

  After both feature branches are clean and committed, create a disposable integration branch from
  the #403 head and merge the #405 head without pushing it:

  ```bash
  git worktree add /home/franciscpd/Projects/_worktrees/loop-effects-integration \
    -b test/loop-effects-integration fix/loop-effect-tool-policy
  git -C /home/franciscpd/Projects/_worktrees/loop-effects-integration \
    merge --no-edit fix/loop-terminal-effect-stream
  ```

- [ ] **Step 2: Build the combined binary and Web bundle**

  ```bash
  mise exec -- make build
  make web-build
  ```

  Record the version, full commit identities, binary SHA-256, and size.

- [ ] **Step 3: Bootstrap a fresh isolated QA lab**

  Use `eng-qa-bootstrap` with the combined binary, distinct HOME, port, UDS, and Web proxy target.
  Register every daemon, Vite, browser, and helper PID in the lab before proceeding.

- [ ] **Step 4: Flag the affected QA tracker scenario**

  Reset `docs/qa/scenarios/LP-terminal-outcome-notification.md` to `qa_status: untested` and append
  a dated impact note before the acceptance walk. This is a changed user-visible Web behavior, not
  a pure refactor. Preserve the scenario's full seven-outcome acceptance contract.

- [ ] **Step 5: Prove structured success and denial**

  Publish a generic transform-only Loop with a same-workspace `on_done` tool effect. Confirm the
  retained event stream contains terminal `status_changed` followed by successful
  `effect_results`. Run a second fixture targeting another workspace and confirm its terminal effect
  is retained with `outcome=failed` and `code=tool_denied`. Confirm `loop-effect` is absent from the
  public agent catalog.

- [ ] **Step 6: Prove visual delivery live and after reload**

  Open the successful run in the isolated Web app with the exact workspace selected. Confirm the
  timeline shows the successful terminal effect after the run reaches `Done`. Reload the page and
  confirm the retained effect row remains. Open the denied run and confirm the failed terminal
  effect is visible with its denial outcome. Capture screenshots and browser-console/network
  evidence.

- [ ] **Step 7: Record the tracker verdict, audit, and tear down**

  Update `LP-terminal-outcome-notification` with the structured reads, screenshots, console/network
  evidence, and teardown reference. Mark it `pass` only if the complete acceptance walk passes; if
  the public fixture still cannot seed all seven terminal outcomes, restore `blocked-verify` and
  record that exact remaining blocker rather than broadening the focused QA claim. Run the strict QA
  audit, then execute the exact bootstrap teardown. Require `clean: true`, `survivors: []`, and no
  listeners on the lab HTTP/Vite ports. Remove the temporary integration worktree only after
  evidence paths are recorded.

---

### Task 4: Finalize and ship the two focused PRs

**Files:**

- Review each branch against `upstream/main` independently.
- Update PR bodies only; do not combine implementation commits.

**Interfaces:**

- Consumes: clean #403 and #405 feature branches plus successful combined QA evidence.
- Produces: two independently reviewable PRs, each linked to its own issue.

- [ ] **Step 1: Apply deslop and review each independent diff**

  Check for unnecessary comments, duplicated lifecycle logic, weak casts, test-only production
  helpers, unrelated docs, and accidental generated changes. Ensure the Web branch changes only its
  design/plan and the canonical hook/test files.

- [ ] **Step 2: Run the final full gate exactly once per frozen branch**

  ```bash
  mise exec -- make gate-full
  make gate-status
  ```

  Run these commands in each feature worktree only after all writes for that branch are complete.
  Require a current full-pass fingerprint before creating its PR.

- [ ] **Step 3: Push and open the daemon PR**

  Push `fix/loop-effect-tool-policy` to the fork and open a PR to `compozy/compozy:main` titled
  `fix: authorize daemon-owned loop tool effects`. Use the repository PR template, include
  `Fixes #403`, the structured and combined visual QA evidence, and an explicit impact audit.

- [ ] **Step 4: Push and open the Web PR**

  Push `fix/loop-terminal-effect-stream` to the fork and open a PR to `compozy/compozy:main` titled
  `fix: retain terminal loop effect stream events`. Use the repository PR template, include
  `Fixes #405`, the RED/GREEN evidence, screenshot evidence, browser cleanup proof, and an explicit
  statement that API/SSE payload schemas and daemon behavior are unchanged.

- [ ] **Step 5: Inspect initial CI and review comments**

  Confirm both PRs target `main`, linked issues resolve correctly, required checks start, and no
  generated or unrelated files appear in either diff. Report all actionable review feedback before
  applying changes.
