// Suite: Loop request route attention
// Invariant: mixed session/request notifications compose into the bell and their
// request deep links focus the real schema field. Owning layer: app route composition.
// No existing loops route suite owns the bell-to-request navigation boundary.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryHistory, createRouter, RouterProvider } from "@tanstack/react-router";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HttpResponse, ws } from "msw";
import { setupServer } from "msw/node";
import { z } from "zod";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";

import { TooltipProvider, UIProvider } from "@compozy/ui";
import { routeTree } from "@/routeTree.gen";
import {
  flattenStorybookHandlerGroups,
  storybookSystemHandlerGroups,
  type StorybookHandlerOverrides,
} from "@/storybook/msw";
import { compozyApiMock } from "@/storybook/openapi-msw";
import { StorybookWorkspaceSetup } from "@/storybook/route-story";
import { resetAgentMockState } from "@/systems/agent/mocks";
import { resetWindowManagerMockState, windowManagerStreamHandler } from "@/systems/os/mocks";
import { sessionCatalogStreamHandler } from "@/systems/session/mocks";
import { sessionStore } from "@/systems/session/stores/session-store";
import { resetWindowManagerSettingsMockState } from "@/systems/settings/mocks";
import { resetSettingsRestartStore } from "@/systems/settings/stores/use-settings-restart-store";
import {
  windowManagerClientFixture,
  windowManagerStorySnapshot,
} from "@/systems/os/mocks/fixtures";
import { clearActiveWorkspaceSelection } from "@/systems/workspace";
import { AttentionRequests, AttentionRequestTarget } from "../loop-runs.stories";

const server = setupServer(
  windowManagerStreamHandler,
  sessionCatalogStreamHandler,
  // These fixture catalogs do not change during the route journey.
  ws.link("*/api/logs/stream").addEventListener("connection", () => {}),
  ws.link("*/api/worktrees/catalog-stream").addEventListener("connection", () => {})
);
const clients: QueryClient[] = [];

beforeAll(() => server.listen({ onUnhandledFrame: "error" }));
afterEach(() => {
  cleanup();
  clients.splice(0).forEach(client => client.clear());
  server.resetHandlers();
  clearActiveWorkspaceSelection();
});
afterAll(() => server.close());

function renderAttentionRoute(story: typeof AttentionRequests | typeof AttentionRequestTarget) {
  const initialEntries = story.parameters?.router?.initialEntries;
  if (!initialEntries?.length) throw new Error("The route story must specify its initial location");
  const msw = story.parameters?.msw;
  if (Array.isArray(msw) || (msw && Array.isArray(msw.handlers))) {
    throw new Error("The route story must use named MSW handler groups");
  }
  const overrides = msw?.handlers as StorybookHandlerOverrides | undefined;
  resetAgentMockState();
  resetWindowManagerSettingsMockState();
  resetWindowManagerMockState(initialEntries[0]);
  clearActiveWorkspaceSelection();
  sessionStore.trigger.allDraftsDiscarded();
  resetSettingsRestartStore();
  // The shared Storybook runtime implements open/close only. The live app now
  // reuses its loops window with window.navigate; supply that daemon response.
  const snapshot = windowManagerStorySnapshot(initialEntries[0]);
  server.use(
    compozyApiMock.post(
      "/api/workspaces/{workspace_id}/window-manager/commands",
      async ({ request }) => {
        const command = await request.json();
        if (command.command_id !== "window.navigate") return;
        const payload = z
          .object({
            window_id: z.string(),
            route: z.object({ pathname: z.string(), search: z.record(z.string(), z.unknown()) }),
          })
          .parse(command.payload);
        const windowId = payload.window_id;
        const window = snapshot.windows[windowId];
        if (!window || !command.client_id) throw new Error("Invalid request target navigation");
        window.route = payload.route;
        snapshot.revision += 1;
        return HttpResponse.json({
          snapshot,
          applied: true,
          changes: { window_ids: [windowId] },
          diagnostics: [],
          client: windowManagerClientFixture(command.client_id, snapshot.workspace_id, windowId),
        });
      }
    ),
    // App-shell reads introduced after these route stories; no extra node rows.
    compozyApiMock.get("/api/settings/shell", () =>
      HttpResponse.json({
        available_scopes: ["user"],
        config: { sessions: { scope: "workspace", sort: "last_activity" } },
        scope: "user",
        section: "shell",
      })
    ),
    compozyApiMock.get("/api/workspaces/{workspace_id}/loop-nodes", () =>
      HttpResponse.json({ items: [], next_cursor: "" })
    ),
    ...flattenStorybookHandlerGroups({
      ...storybookSystemHandlerGroups,
      ...overrides,
    })
  );
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity } },
  });
  clients.push(client);
  const router = createRouter({
    routeTree,
    context: { queryClient: client },
    history: createMemoryHistory({ initialEntries }),
    defaultPreload: "intent",
    defaultStructuralSharing: true,
    defaultPreloadStaleTime: 0,
  });
  render(
    <QueryClientProvider client={client}>
      <UIProvider>
        <TooltipProvider>
          <StorybookWorkspaceSetup />
          <RouterProvider router={router} />
        </TooltipProvider>
      </UIProvider>
    </QueryClientProvider>
  );
  return router;
}

describe("Loop request attention routes", () => {
  it("E2E-030: Should compose four waiting notifications and close the bell when jumping to a request", async () => {
    const user = userEvent.setup();
    const router = renderAttentionRoute(AttentionRequests);
    await user.click(await screen.findByRole("button", { name: "Attention, 4 waiting" }));
    const row = await screen.findByTestId(
      "os-attention-loop-request-ws_launch_hq:looprun_release_train:confirm-rollout:0"
    );
    expect(row).toHaveTextContent("launch-hq");
    expect(row).toHaveTextContent("release-train — ask");
    await user.click(row);
    await waitFor(() => expect(screen.queryByTestId("os-bell-popover")).not.toBeInTheDocument());
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/loop-runs/looprun_release_train")
    );
    expect(router.state.location.search).toMatchObject({
      request_node: "confirm-rollout",
      request_item: 0,
    });
  });

  it("E2E-030: Should focus the requested schema field when opening its deep link", async () => {
    renderAttentionRoute(AttentionRequestTarget);
    expect(await screen.findByTestId("loop-run-detail-content")).toBeVisible();
    await waitFor(() => expect(screen.getByTestId("loop-request-field-regions")).toHaveFocus());
  });
});
