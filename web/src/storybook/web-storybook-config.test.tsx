import { cleanup } from "@testing-library/react";
import { createElement } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

const initialize = vi.fn();
const mswLoader = vi.fn(async () => ({}));

vi.mock("msw-storybook-addon", () => ({
  initialize,
  mswLoader,
}));

const webMain = (await import("../../.storybook/main")).default;
const webPreviewModule = await import("../../.storybook/preview");
const {
  createStorybookQueryClient,
  createStorybookRouter,
  routerDecorator,
  storybookDecorators,
  storybookLoaders,
  storybookSystemHandlerGroups,
  storybookSystemHandlers,
  themeDecorator,
} = webPreviewModule;
const webPreview = webPreviewModule.default;

afterEach(() => {
  cleanup();
  document.documentElement.className = "";
});

describe("web Storybook config", () => {
  it("keeps the expected story glob, addons, framework, and public worker dir", () => {
    expect(webMain.stories).toEqual(["../src/**/*.stories.@(ts|tsx)"]);
    expect(webMain.addons).toEqual([
      "@storybook/addon-docs",
      "@storybook/addon-a11y",
      "@storybook/addon-themes",
    ]);
    expect(webMain.staticDirs).toEqual(["../public"]);
    expect(webMain.framework).toEqual({
      name: "@storybook/react-vite",
      options: {},
    });
  });

  it("registers MSW loaders and the app router/theme decorators in preview", () => {
    expect(initialize).toHaveBeenCalledWith({ onUnhandledRequest: "bypass" });
    expect(webPreview.loaders).toEqual(storybookLoaders);
    expect(storybookLoaders).toEqual([mswLoader]);
    expect(webPreview.decorators).toEqual(storybookDecorators);
    expect(webPreview.parameters?.msw?.handlers).toEqual(storybookSystemHandlerGroups);
    expect(storybookSystemHandlers.length).toBeGreaterThan(0);
    expect(storybookDecorators).toContain(themeDecorator);
    expect(storybookDecorators).toContain(routerDecorator);
  });

  it("creates story-scoped query clients with retry disabled and infinite stale time", () => {
    const queryClient = createStorybookQueryClient();
    const queryOptions = queryClient.getDefaultOptions().queries;

    expect(queryOptions?.retry).toBe(false);
    expect(queryOptions?.staleTime).toBe(Number.POSITIVE_INFINITY);
  });

  it("creates a stub router rooted at slash for non-route stories", async () => {
    const router = createStorybookRouter(() => createElement("div", null, "Story"));

    await router.load();

    expect(router.state.location.pathname).toBe("/");
  });

  it("creates an app router rooted in the real route tree for route stories", async () => {
    const router = createStorybookRouter(undefined, {
      kind: "app",
      initialEntries: ["/workflows"],
    });

    await router.load();

    expect(router.state.location.pathname).toBe("/workflows");
  });
});
