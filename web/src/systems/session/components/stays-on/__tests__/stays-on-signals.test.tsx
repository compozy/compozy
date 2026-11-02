import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, renderHook, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

import { agentInstanceFixture } from "@/systems/agent/mocks";
import type { AgentInstancePayload } from "@/systems/agent";

import { useSessionCreator } from "../../../hooks/use-session-creator";
import { sessionListShelfFilters } from "../../../lib/session-list-shelf";
import { normalizeSessionListFilters } from "../../../lib/session-list-query";
import { withInstanceProjectGroups } from "../../../lib/session-list-groups";
import { StaysOnCollapsedSignal } from "../stays-on-signals";

const mockInstances = vi.hoisted(() => vi.fn(async () => [] as AgentInstancePayload[]));

vi.mock("@/systems/agent/adapters/agent-instance-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/agent/adapters/agent-instance-api")>()),
  fetchAgentInstances: async () => ({ instances: await mockInstances(), capacity: {} }),
}));

function wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return (
    <QueryClientProvider client={client}>
      <UIProvider reducedMotion="never" skipAnimations>
        {children}
      </UIProvider>
    </QueryClientProvider>
  );
}

describe("StaysOnCollapsedSignal", () => {
  it("Should show who needs you in a collapsed project, by face", async () => {
    mockInstances.mockResolvedValue([
      agentInstanceFixture({ id: "a", needs_attention: true }),
      agentInstanceFixture({ id: "b", needs_attention: true }),
      agentInstanceFixture({ id: "c" }),
    ]);
    render(<StaysOnCollapsedSignal workspaceId="ws-compozy" />, { wrapper });

    expect(await screen.findByTestId("stays-on-collapsed-signal")).toHaveTextContent("+1");
    expect(screen.getByRole("img", { name: /Triage Lead needs you/ })).toBeInTheDocument();
  });

  it("Should stay out of a project where nobody needs you", async () => {
    mockInstances.mockResolvedValue([agentInstanceFixture()]);
    const { container } = render(<StaysOnCollapsedSignal workspaceId="ws-compozy" />, { wrapper });

    await waitFor(() => expect(mockInstances).toHaveBeenCalled());
    expect(container.querySelector('[data-testid="stays-on-collapsed-signal"]')).toBeNull();
  });
});

describe("useSessionCreator", () => {
  it("Should name the instance whose main session spawned a session", async () => {
    mockInstances.mockResolvedValue([agentInstanceFixture()]);
    const lineage = { parent_session_id: "sess-main-triage" } as never;
    const { result } = renderHook(() => useSessionCreator({ id: "sess-child", lineage }), {
      wrapper,
    });

    await waitFor(() => expect(result.current?.agent_name).toBe("triage"));
  });

  it("Should find no creator for the main session itself", async () => {
    mockInstances.mockResolvedValue([agentInstanceFixture()]);
    const { result } = renderHook(
      () => useSessionCreator({ id: "sess-main-triage", lineage: undefined }),
      { wrapper }
    );

    await waitFor(() => expect(mockInstances).toHaveBeenCalled());
    expect(result.current).toBeUndefined();
  });
});

describe("stays-on session lists", () => {
  it("Should hide main sessions from every shelf and keep the filter on the request", () => {
    for (const shelf of ["active", "archive", "later"] as const) {
      expect(sessionListShelfFilters(shelf).exclude_instance_main).toBe(true);
    }
    expect(normalizeSessionListFilters({ exclude_instance_main: true })).toMatchObject({
      exclude_instance_main: true,
    });
  });

  it("Should give a project with only an agent its own empty group", () => {
    const input = {
      workspaces: [{ id: "ws-a", name: "alpha" }],
      lens: { profile: "default" },
      ready: true,
    };

    const groups = withInstanceProjectGroups([], ["ws-a", "ws-a", ""], input);

    expect(groups.map(group => [group.key, group.total])).toEqual([["project:ws-a", 0]]);
    expect(withInstanceProjectGroups([], ["ws-a"], { ...input, ready: false })).toEqual([]);
    expect(withInstanceProjectGroups(groups, ["ws-a"], input)).toEqual(groups);
  });
});
