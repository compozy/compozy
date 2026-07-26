import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { createElement } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";

const workerStart = vi.fn(async () => {});
const setupWorker = vi.fn(() => ({ start: workerStart, use: vi.fn(), resetHandlers: vi.fn() }));
const mswLoader = vi.fn(() => async () => ({}));

vi.mock("msw/browser", () => ({ setupWorker }));
vi.mock("msw-storybook-addon/csf3", () => ({ mswLoader }));

const webPreviewModule = await import("../../../.storybook/preview");
const {
  createStorybookMswWorker,
  createStorybookQueryClient,
  createStorybookRouter,
  isBypassableStorybookRequest,
  isStorybookLocalApiRequest,
  queryClientDecorator,
  resetStorybookAppState,
  routerDecorator,
  storybookDecorators,
  storybookUnhandledRequest,
  storybookSystemHandlers,
} = webPreviewModule;
const { StorybookWorkspaceSetup } = await import("../route-story");
const { useActiveWorkspaceStore } =
  await import("@/systems/workspace/hooks/use-active-workspace-store");

afterEach(() => {
  cleanup();
  document.documentElement.className = "";
  useActiveWorkspaceStore.getState().clearSelectedWorkspaceId();
});

function QueryClientProbe() {
  const queryClient = useQueryClient();
  const queryOptions = queryClient.getDefaultOptions().queries;

  return createElement(
    "output",
    { "data-testid": "query-client-defaults" },
    `${String(queryOptions?.retry)}|${String(queryOptions?.staleTime)}`
  );
}

describe("web Storybook config", () => {
  it("registers MSW and exposes router decorators with system handlers", async () => {
    expect(mswLoader).toHaveBeenCalledWith(createStorybookMswWorker);

    await createStorybookMswWorker();

    expect(workerStart).toHaveBeenCalledWith({ onUnhandledRequest: storybookUnhandledRequest });
    expect(storybookSystemHandlers.length).toBeGreaterThan(0);
    expect(storybookDecorators).toContain(routerDecorator);
    expect(storybookDecorators).not.toContain(queryClientDecorator);
  });

  it("fails local unhandled API requests while bypassing assets and third-party calls", () => {
    const print = {
      error: vi.fn(),
      warning: vi.fn(),
    };
    const localApiUrl = new URL(`${location.origin}/api/storybook-unhandled-request`);
    const storybookConsolePipeUrl = new URL(`${location.origin}/__tsd/console-pipe`);
    const storyAssetUrl = new URL(`${location.origin}/src/main.tsx`);
    const thirdPartyUrl = new URL("https://cdn.example.com/library.js");
    const localUnknownUrl = new URL(`${location.origin}/unexpected-route`);

    expect(isStorybookLocalApiRequest(localApiUrl)).toBe(true);
    expect(isBypassableStorybookRequest(storybookConsolePipeUrl)).toBe(true);
    expect(isBypassableStorybookRequest(storyAssetUrl)).toBe(true);
    expect(isBypassableStorybookRequest(thirdPartyUrl)).toBe(true);

    storybookUnhandledRequest(new Request(localApiUrl.href), print);
    storybookUnhandledRequest(new Request(storybookConsolePipeUrl.href), print);
    storybookUnhandledRequest(new Request(storyAssetUrl.href), print);
    storybookUnhandledRequest(new Request(thirdPartyUrl.href), print);
    storybookUnhandledRequest(new Request(localUnknownUrl.href), print);

    expect(print.error).toHaveBeenCalledTimes(1);
    expect(print.warning).toHaveBeenCalledTimes(1);
  });

  it("creates story-scoped query clients with retry disabled and infinite stale time", () => {
    const queryClient = createStorybookQueryClient();
    const queryOptions = queryClient.getDefaultOptions().queries;

    expect(queryOptions?.retry).toBe(false);
    expect(queryOptions?.staleTime).toBe(Number.POSITIVE_INFINITY);
  });

  it("wraps stories in a QueryClientProvider with the expected defaults", () => {
    render(queryClientDecorator(() => createElement(QueryClientProbe)));

    expect(screen.getByTestId("query-client-defaults")).toHaveTextContent("false|Infinity");
  });

  it("creates a memory router stub rooted at slash for story decorators", async () => {
    const router = createStorybookRouter();

    await router.load();

    expect(router.state.location.pathname).toBe("/");
    expect(storybookDecorators).toContain(routerDecorator);
  });

  it("includes placeholder routes for linked app surfaces used by stories", async () => {
    const router = createStorybookRouter();

    await router.navigate({ to: "/session/$id", params: { id: "sess-storybook" } });
    expect(router.state.location.pathname).toBe("/session/sess-storybook");

    await router.navigate({ to: "/jobs" });
    expect(router.state.location.pathname).toBe("/jobs");

    await router.navigate({ to: "/triggers" });
    expect(router.state.location.pathname).toBe("/triggers");

    await router.navigate({ to: "/tasks" });
    expect(router.state.location.pathname).toBe("/tasks");

    await router.navigate({ to: "/tasks/new", search: () => ({ template: undefined }) });
    expect(router.state.location.pathname).toBe("/tasks/new");

    await router.navigate({ to: "/tasks/$id", params: { id: "task_001" } });
    expect(router.state.location.pathname).toBe("/tasks/task_001");

    await router.navigate({ to: "/tasks/$id/edit", params: { id: "task_001" } });
    expect(router.state.location.pathname).toBe("/tasks/task_001/edit");

    await router.navigate({
      to: "/tasks/$id/runs/$runId",
      params: { id: "task_001", runId: "run_001" },
    });
    expect(router.state.location.pathname).toBe("/tasks/task_001/runs/run_001");
  });

  it("creates an app router rooted in the real route tree for nested settings stories", async () => {
    const router = createStorybookRouter(
      undefined,
      {
        kind: "app",
        initialEntries: ["/settings/providers"],
      },
      createStorybookQueryClient()
    );

    await router.load();

    expect(router.state.location.pathname).toBe("/settings/providers");
  });

  it("Should reset app state and select the story workspace without browser-owned topology", async () => {
    useActiveWorkspaceStore.getState().setSelectedWorkspaceId("workspace:stale");
    resetStorybookAppState();

    expect(useActiveWorkspaceStore.getState().selectedWorkspaceId).toBeNull();

    render(createElement(StorybookWorkspaceSetup));
    await waitFor(() => {
      expect(useActiveWorkspaceStore.getState().selectedWorkspaceId).toBeTruthy();
    });
  });

  it("renders stories through the router decorator stub with a query client available", async () => {
    render(
      routerDecorator(() =>
        createElement(
          "div",
          null,
          createElement("div", { "data-testid": "router-story" }, "Story"),
          createElement(QueryClientProbe)
        )
      )
    );

    expect(await screen.findByTestId("router-story")).toHaveTextContent("Story");
    expect(await screen.findByTestId("query-client-defaults")).toHaveTextContent("false|Infinity");
  });
});
