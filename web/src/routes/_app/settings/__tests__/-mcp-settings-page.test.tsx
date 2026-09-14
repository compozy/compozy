// Invariant: Settings actions select the daemon-returned MCP owner and scope, even when names repeat.
// Owner: the new MCP Settings page; no existing route suite exercises this composition.
// Canonical suite: this file. HTTP is mocked; routing, query state, controllers and editors run normally.
import { TopbarSlotProvider } from "@compozy/ui";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  createMemoryHistory,
  createRootRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it } from "vitest";

import { handlers as profileHandlers } from "@/systems/profiles/mocks";
import { handlers as workspaceHandlers } from "@/systems/workspace/mocks";
import { handlers as statusHandlers } from "@/systems/status/mocks";
import { handlers as settingsHandlers } from "@/systems/settings/mocks";
import {
  mcpExtensionServerFixtures,
  settingsAppliedMutationFixture,
} from "@/systems/settings/mocks/fixtures";
import { resetSettingsRestartStore } from "@/systems/settings/stores/use-settings-restart-store";
import type { SettingsMCPServerEntry, SettingsMCPServerRequest } from "@/systems/settings";
import { MCPSettingsPage } from "../-mcp-settings-page";

let definitions: SettingsMCPServerEntry[] = [];
const server = setupServer(
  http.get("*/api/settings/mcp-servers", () =>
    HttpResponse.json({
      collection: "mcp-servers",
      available_scopes: ["user", "workspace"],
      scope: "user",
      mcp_servers: definitions,
    })
  ),
  ...settingsHandlers,
  ...profileHandlers,
  ...workspaceHandlers,
  ...statusHandlers
);
const clients: QueryClient[] = [];
beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterAll(() => server.close());
beforeEach(() => {
  definitions = structuredClone(mcpExtensionServerFixtures.slice(0, 2));
  resetSettingsRestartStore();
});
afterEach(() => {
  cleanup();
  clients.splice(0).forEach(client => client.clear());
  server.resetHandlers();
  resetSettingsRestartStore();
});
async function openSettings() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  clients.push(client);
  const root = createRootRoute({
    component: () => (
      <TopbarSlotProvider>
        <MCPSettingsPage />
      </TopbarSlotProvider>
    ),
  });
  const router = createRouter({
    routeTree: root,
    history: createMemoryHistory({ initialEntries: ["/settings/mcp"] }),
  });
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  );
  await screen.findByRole("button", { name: "Select github from github" });
}

describe("MCP Settings page", () => {
  it("Should keep same-name owners separate and save or reset only the extension override", async () => {
    const user = userEvent.setup();
    const writes: Array<{ url: URL; body: SettingsMCPServerRequest }> = [];
    const resets: URL[] = [];
    const manual = structuredClone(definitions[0]);
    server.use(
      http.put("*/api/settings/mcp-servers/:name", async ({ request }) => {
        const body = (await request.json()) as SettingsMCPServerRequest;
        writes.push({ url: new URL(request.url), body });
        definitions[1] = {
          ...definitions[1]!,
          override: { headers: body.server.headers, url: body.server.url },
        };
        return HttpResponse.json({ ...settingsAppliedMutationFixture, section: "mcp-servers" });
      }),
      http.delete("*/api/settings/mcp-servers/:name", ({ request }) => {
        resets.push(new URL(request.url));
        definitions[1] = { ...definitions[1]!, override: undefined };
        return HttpResponse.json({ ...settingsAppliedMutationFixture, section: "mcp-servers" });
      })
    );
    await openSettings();
    expect(screen.getByRole("button", { name: "Select github" })).toBeVisible();
    expect(screen.getByText("runs as github.github")).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Select github from github" }));
    expect(screen.queryByTestId("settings-page-mcp-selection-delete")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Edit github from github" }));
    const dialog = await screen.findByRole("dialog", { name: "Edit configuration · github" });
    expect(within(dialog).getByText("extension:github")).toBeVisible();
    expect(
      within(dialog).queryByRole("textbox", { name: /Command|Auth|Server name/ })
    ).not.toBeInTheDocument();
    await user.type(
      within(dialog).getByRole("textbox", { name: /Endpoint/ }),
      "https://override.example/mcp"
    );
    await user.click(within(dialog).getByRole("button", { name: "Save override" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(writes).toHaveLength(1);
    expect(writes[0]!.url.pathname).toBe("/api/settings/mcp-servers/github");
    expect(writes[0]!.url.searchParams.get("owner")).toBe("extension:github");
    expect(writes[0]!.url.searchParams.get("scope")).toBe("user");
    expect(writes[0]!.body).toEqual({
      server: {
        name: "github",
        env: {},
        headers: { "X-GitHub-Api-Version": "2022-11-28" },
        url: "https://override.example/mcp",
      },
    });
    expect(definitions[0]).toEqual(manual);
    await user.click(screen.getByRole("button", { name: "Edit github from github" }));
    await user.click(await screen.findByRole("button", { name: "Reset override" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(resets).toHaveLength(1);
    expect(resets[0]!.searchParams.get("owner")).toBe("extension:github");
    expect(resets[0]!.searchParams.get("scope")).toBe("user");
    expect(definitions[0]).toEqual(manual);
    expect(screen.getByRole("button", { name: "Select github from github" })).toBeVisible();
  });

  it("Should delete the selected manual source without removing its same-name extension", async () => {
    const user = userEvent.setup();
    const removed: URL[] = [];
    server.use(
      http.delete("*/api/settings/mcp-servers/:name", ({ request }) => {
        removed.push(new URL(request.url));
        definitions = definitions.filter(entry => entry.owner !== "manual");
        return HttpResponse.json({ ...settingsAppliedMutationFixture, section: "mcp-servers" });
      })
    );
    await openSettings();
    // A workspace collection may inherit this user-owned definition. Its effective source owns deletion.
    await user.click(screen.getByTestId("settings-page-mcp-scope-workspace"));
    await user.click(screen.getByRole("button", { name: "Select github" }));
    await user.click(screen.getByTestId("settings-page-mcp-selection-delete"));
    await user.click(await screen.findByRole("button", { name: "Delete definition" }));
    await waitFor(() =>
      expect(screen.queryByRole("button", { name: "Select github" })).not.toBeInTheDocument()
    );
    expect(removed).toHaveLength(1);
    expect(removed[0]!.searchParams.get("owner")).toBeNull();
    expect(removed[0]!.searchParams.get("scope")).toBe("user");
    expect(removed[0]!.searchParams.get("target")).toBe("sidecar");
    expect(removed[0]!.searchParams.get("workspace_id")).toBeNull();
    expect(screen.getByRole("button", { name: "Select github from github" })).toBeVisible();
  });
});
