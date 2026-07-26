---
name: agh-ui-screenshot
description: Screenshot capture for deterministic PNG evidence and visual-contract bundles for AGH Storybook stories and local UI URLs. Use for visual audits, regression diffs, and design-parity checks. Do not use for interactive E2E flows, remote authenticated sites, or Storybook test execution.
---

# AGH UI Screenshot

Capture deterministic PNG evidence through the bundled CDP helper. The helper
owns load/font settling and viewport emulation; the procedure owns target
resolution, evidence checks, and process cleanup.

When a task or spec names a trusted visual reference, **STOP and read
`.agents/skills/agh/agh-ui-screenshot/references/visual-contract.md` in full
before capture or implementation**. Its evidence bundle, not an
implementation-only screenshot, is the completion contract.

## Procedure

**Step 1: Prepare an Owned Workdir**

1. Start at the repository root and record it: `REPO_ROOT="$(pwd)"`.
2. Run the bootstrap helper with a unique directory:
   `WORKDIR="$(bash "$REPO_ROOT/.agents/skills/agh/agh-ui-screenshot/scripts/setup-workdir.sh" "$(mktemp -d /tmp/agh-ui-screenshot.XXXXXX)")"`
3. The bootstrap copies the current capture helpers into `WORKDIR` so Bun resolves their dependencies from the isolated package. Keep `REPO_ROOT` for references and server packages; execute the materialized helpers from `WORKDIR`.

*Done when:* one unique workdir exists, its dependencies are installed, and every helper path resolves independently of the current directory.

**Step 2: Establish Storybook Ownership**

1. Skip this step when the capture set has no Storybook target. Otherwise read `.agents/skills/agh/agh-ui-screenshot/references/storybook-urls.md` in full.
2. Probe the required server (`6006` for `web`, `6007` for `packages/ui`) and reuse it only when its `index.json` is healthy.
3. If a server is absent, start the matching `bun run storybook` from its package, redirect logs into `WORKDIR`, and immediately record its PID as `WORKDIR/web-storybook.pid` or `WORKDIR/ui-storybook.pid`.
4. Poll the health endpoint until it returns `200` or the owned process exits; on exit, fail with its log.

*Done when:* every required Storybook is healthy and each server started by this run has exactly one PID file; pre-existing servers have none.

**Step 3: Resolve Story IDs**

1. Skip this step for arbitrary non-Storybook URLs.
2. From `WORKDIR`, run the materialized read-only helper:
   `bun run "$WORKDIR/list-stories.mjs" http://localhost:6006 [--filter <substring>]`
3. Confirm every requested story id appears exactly in the output; never infer an id from the app URL.

*Done when:* every Storybook target is backed by a real `index.json` entry.

**Step 4: Capture Through CDP**

1. From `WORKDIR`, run the materialized mutating helper:
   `bun run "$WORKDIR/cap.mjs" --out <output-dir> --width <W> --height <H> --wait <ms> --shot <name> <url> [...]`
2. Use `1440 × 900` for routes, `1680 × 1050` for wide breakpoints, `1100 × 700` for primitives, and `320 × 800` for collapsed navigation. Read `.agents/skills/agh/agh-ui-screenshot/references/cdp-flow.md` in full before changing capture mechanics or defaults.
3. Use `2200 ms` as the current route settle floor; increase to `4000 ms` only when captured evidence shows fallback fonts.
4. Treat a non-zero helper exit as a failed capture set. A sub-20 KB PNG is also suspicious and requires inspection even when capture succeeded.

*Done when:* every requested target prints `saved`, the helper exits zero, and every expected PNG exists with a plausible size.

**Step 5: Capture a Proposal Mock When Requested**

1. Only for proposal-side state not reachable from the canonical HTML, read `.agents/skills/agh/agh-ui-screenshot/references/proposal-mock-capture.md` in full and follow its clone-and-patch branch.
2. Start the static server as an owned process and record its PID in `WORKDIR/proposal-server.pid`.
3. Capture with the same `WORKDIR/cap.mjs`, then remove the throwaway wrapper without editing the canonical proposal.

*Done when:* every requested proposal state is captured, the canonical HTML is unchanged, and the temporary wrapper is gone.

**Step 6: Inspect the Evidence**

1. Verify the exact expected PNG set; file size is a tripwire, not proof.
2. Open at least one PNG from every distinct surface or viewport and compare it with the intended state or trusted baseline.
3. For visual-contract runs, inspect every reference/implementation pair and require the complete evidence bundle before reporting parity.

*Done when:* target identity, viewport, rendered content, fonts, and visible state are verified rather than inferred from filenames; visual-contract runs additionally have no unresolved blocking divergence.

**Step 7: Tear Down Owned Processes**

1. Terminate only PIDs recorded under this run's `WORKDIR`: request graceful termination, wait for exit, and escalate only the same proven-owned process when necessary.
2. Preserve healthy servers that predated this run.
3. Confirm every owned PID is dead before completion; files may remain for evidence.

*Done when:* all processes started by this run are stopped and no unowned browser or dev server was touched.

## Error Handling

- **A capture exits non-zero:** read stderr and `.agents/skills/agh/agh-ui-screenshot/references/troubleshooting.md`; fix the failing target and rerun the whole requested set.
- **A Storybook port is occupied but unhealthy:** identify the owner and report it. Reuse only a healthy `index.json`; never kill an unowned process to claim the port.
- **The bootstrap workdir fails:** discard that unique temporary workdir and rerun the bootstrap into a new one; do not mutate repository dependencies.
- **Chrome survives a crashed helper:** use the printed debug port to identify the exact headless Chrome PID, verify ownership from its command line, then terminate only that PID/process group. Never use a machine-wide `pkill`.
- **Reference and implementation dimensions differ:** recapture both at the exact contract viewport; never resize one image to manufacture a diff.
