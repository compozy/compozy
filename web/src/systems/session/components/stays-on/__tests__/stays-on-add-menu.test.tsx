import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

import { AgentCreateHostProvider } from "@/systems/agent";
import { agentFixtures, agentInstanceFixture } from "@/systems/agent/mocks";
import { workspaceFixtures } from "@/systems/workspace/mocks/fixtures";

import { StaysOnCreateMenu } from "../stays-on-create-menu";

const { mockApi } = vi.hoisted(() => ({
  mockApi: {
    instances: vi.fn(),
    create: vi.fn(),
    catalog: vi.fn(),
  },
}));

vi.mock("@/systems/workspace/adapters/workspace-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/workspace/adapters/workspace-api")>()),
  fetchWorkspaces: async () => [
    { ...workspaceFixtures[0]!, id: "ws-compozy", name: "compozy", archived_at: null },
    { ...workspaceFixtures[0]!, id: "ws-site", name: "site", archived_at: null },
  ],
}));

vi.mock("@/systems/agent/adapters/agent-instance-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/agent/adapters/agent-instance-api")>()),
  fetchAgentInstances: async () => ({ instances: await mockApi.instances(), capacity: {} }),
  createAgentInstance: mockApi.create,
}));

vi.mock("@/systems/agent/adapters/agent-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/agent/adapters/agent-api")>()),
  fetchAgentCatalog: mockApi.catalog,
}));

vi.mock("@/systems/settings/hooks/use-settings-collections", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/settings/hooks/use-settings-collections")>()),
  useSettingsProviders: () => ({ data: { providers: [] } }),
}));

function grouped(name: string, label: string, kind = "stays_on") {
  return {
    name,
    label,
    kind,
    definition_digest: "d",
    scopes: [],
    active: 0,
    agent: { ...agentFixtures[0]!, name, label },
  };
}

const openDialog = vi.fn();

function renderMenu(
  staysOn: Parameters<typeof StaysOnCreateMenu>[0]["staysOn"] = {
    place: "ws-compozy",
    name: "compozy",
  }
) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>
      <UIProvider reducedMotion="never" skipAnimations>
        <AgentCreateHostProvider openDialog={openDialog} openForDuplicate={vi.fn()}>
          {children}
        </AgentCreateHostProvider>
      </UIProvider>
    </QueryClientProvider>
  );
  render(<StaysOnCreateMenu onNewSession={vi.fn()} staysOn={staysOn} testId="group" />, {
    wrapper,
  });
}

async function openPicker(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByTestId("group-new"));
  await user.click(await screen.findByTestId("group-add-stays-on"));
}

describe("Project + Add stays-on agent", () => {
  beforeEach(() => {
    openDialog.mockReset();
    for (const call of Object.values(mockApi)) call.mockReset();
    mockApi.instances.mockResolvedValue([agentInstanceFixture()]);
    mockApi.create.mockResolvedValue(agentInstanceFixture({ agent_name: "scout" }));
    mockApi.catalog.mockResolvedValue({
      items: [
        grouped("triage", "Triage Lead"),
        grouped("scout", "Scout"),
        grouped("general", "General", "on_demand"),
      ],
      page: { has_more: false },
    });
  });

  it("Should list only stays-on agents not yet in this project, and add the one picked", async () => {
    const user = userEvent.setup();
    renderMenu();

    await openPicker(user);

    expect(await screen.findByText("Scout")).toBeInTheDocument();
    expect(screen.queryByText("Triage Lead")).toBeNull();
    expect(screen.queryByText("General")).toBeNull();

    await user.click(screen.getByText("Scout"));

    await waitFor(() =>
      expect(mockApi.create.mock.calls[0]?.[0]).toEqual({
        agent: "scout",
        workspace: "ws-compozy",
      })
    );
  });

  it("Should open the create screen on that project from Create new", async () => {
    const user = userEvent.setup();
    renderMenu();

    await openPicker(user);
    await user.click(await screen.findByText("Create new"));

    expect(openDialog).toHaveBeenCalledWith({ place: "ws-compozy" });
  });

  // Invariant: a list spanning several places asks where the agent lives before adding it,
  // preselecting a place and accepting another.
  it("Should ask where the agent lives when the list spans several places", async () => {
    const user = userEvent.setup();
    renderMenu(null);

    await openPicker(user);

    expect(await screen.findByText("Global")).toBeInTheDocument();
    expect(screen.getByRole("option", { name: /compozy/ })).toHaveAttribute("data-checked", "true");
    await user.click(screen.getByRole("option", { name: /site/ }));
    expect(await screen.findByText("Add to site")).toBeInTheDocument();
    await user.click(screen.getByText("Scout"));

    await waitFor(() =>
      expect(mockApi.create.mock.calls[0]?.[0]).toEqual({ agent: "scout", workspace: "ws-site" })
    );
    expect(mockApi.catalog.mock.calls.at(-1)?.[0]).toMatchObject({ workspace: "ws-site" });
  });
});
