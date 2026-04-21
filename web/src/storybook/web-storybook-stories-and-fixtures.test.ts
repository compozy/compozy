import { describe, expect, it } from "vitest";

describe("web Storybook coverage contract", () => {
  it("loads route story modules for every required daemon route family", async () => {
    const modules = await Promise.all([
      import("@/routes/_app/stories/-dashboard.stories"),
      import("@/routes/_app/stories/-workflows.stories"),
      import("@/routes/_app/stories/-workflows.$slug.tasks.stories"),
      import("@/routes/_app/stories/-workflows.$slug.tasks.$taskId.stories"),
      import("@/routes/_app/stories/-runs.stories"),
      import("@/routes/_app/stories/-runs.$runId.stories"),
      import("@/routes/_app/stories/-reviews.stories"),
      import("@/routes/_app/stories/-reviews.$slug.$round.$issueId.stories"),
      import("@/routes/_app/stories/-workflows.$slug.spec.stories"),
      import("@/routes/_app/stories/-memory.stories"),
      import("@/routes/_app/stories/-memory.$slug.stories"),
    ]);

    expect(modules).toHaveLength(11);

    for (const module of modules) {
      expect(module.default).toBeDefined();
    }
  });

  it("keeps every mocked domain barrel populated for Storybook composition", async () => {
    const mockModules = await Promise.all([
      import("@/systems/app-shell/mocks"),
      import("@/systems/dashboard/mocks"),
      import("@/systems/workflows/mocks"),
      import("@/systems/runs/mocks"),
      import("@/systems/reviews/mocks"),
      import("@/systems/spec/mocks"),
      import("@/systems/memory/mocks"),
    ]);

    for (const module of mockModules) {
      expect(module.handlers.length).toBeGreaterThan(0);
    }
  });
});
