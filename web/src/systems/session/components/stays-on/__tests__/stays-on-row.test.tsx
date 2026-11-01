import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

import type { AgentInstancePayload } from "@/systems/agent";
import { agentInstanceFixture } from "@/systems/agent/mocks";

import { StaysOnRow } from "../stays-on-row";

const { mockNavigate, mockApi } = vi.hoisted(() => ({
  mockNavigate: vi.fn(),
  mockApi: { ensureMain: vi.fn(), pause: vi.fn(), resume: vi.fn() },
}));

vi.mock("@/systems/agent/adapters/agent-instance-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/agent/adapters/agent-instance-api")>()),
  ensureAgentInstanceMain: mockApi.ensureMain,
  pauseAgentInstance: mockApi.pause,
  resumeAgentInstance: mockApi.resume,
}));

vi.mock("@tanstack/react-router", async importOriginal => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  useNavigate: () => mockNavigate,
}));

function wrap(children: ReactNode) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return (
    <QueryClientProvider client={client}>
      <UIProvider reducedMotion="never" skipAnimations>
        {children}
      </UIProvider>
    </QueryClientProvider>
  );
}

function renderRow(
  instances: readonly AgentInstancePayload[],
  props: Partial<Parameters<typeof StaysOnRow>[0]> = {}
) {
  const onOpenSession = vi.fn();
  render(wrap(<StaysOnRow instances={instances} onOpenSession={onOpenSession} {...props} />));
  return { onOpenSession };
}

const seatNames = () =>
  within(screen.getByRole("toolbar", { name: "Stays-on agents" }))
    .getAllByRole("button")
    .map(button => button.getAttribute("aria-label"));

describe("StaysOnRow", () => {
  beforeEach(() => {
    mockNavigate.mockReset();
    for (const call of Object.values(mockApi)) call.mockReset();
  });

  it("Should render nothing with no instances", () => {
    renderRow([]);
    expect(screen.queryByTestId("stays-on-row")).toBeNull();
  });

  it("Should seat whoever needs you first", () => {
    renderRow([
      agentInstanceFixture({ id: "a", label: "Idle" }),
      agentInstanceFixture({ id: "b", label: "Paused", state: "paused", pause_reason: "manual" }),
      agentInstanceFixture({ id: "c", label: "Busy", activity: "working" }),
      agentInstanceFixture({ id: "d", label: "Waiting", needs_attention: true }),
    ]);

    expect(seatNames()).toEqual([
      "Waiting · compozy · Needs you",
      "Busy · compozy · Working",
      "Idle · compozy · Idle",
      "Paused · compozy · Paused",
    ]);
  });

  it("Should move the rest into +N and open the whole crew by state", async () => {
    const user = userEvent.setup();
    const crew = Array.from({ length: 8 }, (_, index) =>
      agentInstanceFixture({ id: `i-${index}`, label: `Agent ${index}` })
    );
    renderRow(crew, { limit: 3, scopeName: "compozy" });

    await user.click(screen.getByRole("button", { name: "Show all 8 agents in compozy" }));

    expect(screen.getByText("8 agents in compozy")).toBeInTheDocument();
    expect(screen.getByText("8 resting")).toBeInTheDocument();
    await user.type(screen.getByRole("searchbox", { name: "Find an agent" }), "Agent 5");
    expect(screen.getByRole("button", { name: /Agent 5/ })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Agent 4/ })).toBeNull();
  });

  it("Should open the main session on click and mark the open one", async () => {
    const user = userEvent.setup();
    const instance = agentInstanceFixture();
    const { onOpenSession } = renderRow([instance], { currentSessionId: "sess-main-triage" });

    const seat = screen.getByTestId(`stays-on-seat-${instance.id}`);
    expect(seat).toHaveAttribute("aria-current", "page");
    await user.click(seat);

    expect(onOpenSession).toHaveBeenCalledWith({
      id: "sess-main-triage",
      agent_name: "triage",
      workspace_id: "ws-compozy",
    });
  });

  it("Should create the main session again when it was lost", async () => {
    const user = userEvent.setup();
    const instance = agentInstanceFixture({ main_session_id: "" });
    mockApi.ensureMain.mockResolvedValue({ ...instance, main_session_id: "sess-rebuilt" });
    const { onOpenSession } = renderRow([instance]);

    await user.click(screen.getByTestId(`stays-on-seat-${instance.id}`));

    await waitFor(() =>
      expect(onOpenSession).toHaveBeenCalledWith(expect.objectContaining({ id: "sess-rebuilt" }))
    );
  });

  it("Should disable Resume when the definition is gone, and say why", async () => {
    const user = userEvent.setup();
    const instance = agentInstanceFixture({
      state: "paused",
      pause_reason: "definition_missing",
    });
    renderRow([instance]);

    await user.pointer({
      keys: "[MouseRight]",
      target: screen.getByTestId(`stays-on-seat-${instance.id}`),
    });

    expect(
      await screen.findByText("Paused because this agent's definition is gone. Its memory is kept.")
    ).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: "Resume" })).toHaveAttribute(
      "aria-disabled",
      "true"
    );
  });

  it("Should pause from the context menu", async () => {
    const user = userEvent.setup();
    const instance = agentInstanceFixture();
    mockApi.pause.mockResolvedValue({ ...instance, state: "paused" });
    renderRow([instance]);

    await user.pointer({
      keys: "[MouseRight]",
      target: screen.getByTestId(`stays-on-seat-${instance.id}`),
    });
    await user.click(await screen.findByRole("menuitem", { name: "Pause" }));

    await waitFor(() => expect(mockApi.pause.mock.calls[0]?.[0]).toBe(instance));
  });
});
