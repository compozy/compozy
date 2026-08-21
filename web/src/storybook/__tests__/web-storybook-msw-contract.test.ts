import type { HttpHandler } from "msw";
import { describe, expect, it } from "vitest";

const { storybookSystemHandlerGroups, storybookSystemHandlers } =
  await import("../../../.storybook/preview");
const { flattenStorybookHandlerGroups } = await import("../msw");

function handlerSignature(handler: HttpHandler) {
  const method = String(handler.info.method);
  const path = String(handler.info.path).replace(/^\*/, "");
  return `${method} ${path}`;
}

function normalizedHandlerSignature(handler: HttpHandler) {
  return handlerSignature(handler)
    .replace(/:[^/]+/g, "{param}")
    .replace(/\{[^/]+\}/g, "{param}");
}

describe("web Storybook MSW contract", () => {
  it("exposes default Storybook system handlers", () => {
    expect(storybookSystemHandlers.length).toBeGreaterThan(0);
    expect(flattenStorybookHandlerGroups(storybookSystemHandlerGroups).length).toBe(
      storybookSystemHandlers.length
    );
  });

  it("does not register duplicate method/path handler pairs across the combined system set", () => {
    const signatures = storybookSystemHandlers.map(normalizedHandlerSignature);
    const uniqueSignatures = new Set(signatures);

    expect(uniqueSignatures.size).toBe(signatures.length);
  });

  it("registers the onboarding status handler required by app route stories", () => {
    const signatures = storybookSystemHandlers.map(handlerSignature);

    expect(signatures).toContain("GET /api/onboarding");
  });

  it("registers the runtime log stream handler required by nav count stories", () => {
    const signatures = storybookSystemHandlers.map(handlerSignature);

    expect(signatures).toContain("GET /api/logs/stream");
  });

  it("registers the provider catalog and directory browser handlers required by onboarding route stories", () => {
    const signatures = storybookSystemHandlers.map(handlerSignature);

    expect(signatures).toContain("GET /api/providers");
    expect(signatures).toContain("GET /api/fs/browse");
  });

  it("registers the bridge health stream handler required by bridge route stories", () => {
    const signatures = storybookSystemHandlers.map(handlerSignature);

    expect(signatures).toContain("GET /api/bridges/health/stream");
  });

  it("registers the marketplace search handler required by marketplace route stories", () => {
    const signatures = storybookSystemHandlers.map(handlerSignature);

    expect(signatures).toContain("GET /api/marketplace/search");
  });

  it("registers the loop run event stream handler required by loop run route stories", () => {
    const signatures = storybookSystemHandlers.map(normalizedHandlerSignature);

    expect(signatures).toContain("GET /api/workspaces/{param}/loop-runs/{param}/events");
  });

  it("registers the loop run read handlers required by loop run route stories", () => {
    const signatures = storybookSystemHandlers.map(normalizedHandlerSignature);

    expect(signatures).toContain("GET /api/workspaces/{param}/loop-runs/{param}/briefing");
    expect(signatures).toContain("GET /api/workspaces/{param}/loop-runs/{param}/nodes");
    expect(signatures).toContain("GET /api/workspaces/{param}/loop-runs/{param}/timeline");
  });
});

// A Visual Contract row with no target is not a missing screenshot — it is a
// state nobody can look at, and without this it surfaces halfway through a
// capture run instead of here.
describe("loop run Visual Contract targets", () => {
  it("stages every VC row against a story export that exists", async () => {
    const { LOOP_RUN_VISUAL_CONTRACT } =
      await import("../../systems/loops/components/stories/loop-run-visual-contract");
    const modules = [
      await import("../../systems/loops/components/stories/loop-run-page.stories"),
      await import("../../systems/loops/components/stories/loop-run-registers.stories"),
      await import("../../systems/loops/components/stories/loop-runs.stories"),
    ];
    const staged = new Set<string>();
    for (const module of modules) {
      const title = (module.default as { title?: string }).title;
      for (const exportName of Object.keys(module)) {
        if (exportName === "default") continue;
        staged.add(`${title}::${exportName}`);
      }
    }

    const missing = LOOP_RUN_VISUAL_CONTRACT.filter(
      row => !staged.has(`${row.title}::${row.exportName}`)
    ).map(row => `${row.id} → ${row.title}::${row.exportName}`);

    expect(missing).toEqual([]);
    // The table in `task_05.md` is VC-01..VC-36 exactly, in order. Counting rows
    // and de-duplicating ids would still accept VC-37 standing in for VC-36.
    const expectedIds = Array.from(
      { length: 36 },
      (_, index) => `VC-${String(index + 1).padStart(2, "0")}`
    );
    expect(LOOP_RUN_VISUAL_CONTRACT.map(row => row.id)).toEqual(expectedIds);
  });
});
