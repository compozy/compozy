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

  it("Should resume every agent the brake paused, once two or more did [UT-080]", async () => {
    const user = userEvent.setup();
    const crew = Array.from({ length: 5 }, (_, index) =>
      agentInstanceFixture({
        id: `i-${index}`,
        label: `Agent ${index}`,
        ...(index < 2 ? { state: "paused", pause_reason: "unattended" } : {}),
        ...(index === 2 ? { state: "paused", pause_reason: "manual" } : {}),
      })
    );
    mockApi.resume.mockResolvedValue(undefined);
    renderRow(crew, { limit: 3, scopeName: "compozy" });

    await user.click(screen.getByRole("button", { name: "Show all 5 agents in compozy" }));
    await user.click(screen.getByRole("button", { name: "Resume all" }));

    await waitFor(() => expect(mockApi.resume).toHaveBeenCalledTimes(2));
    expect(mockApi.resume.mock.calls.map(call => (call[0] as AgentInstancePayload).id)).toEqual([
      "i-0",
      "i-1",
    ]);
  });

  it("Should say which agents didn't resume when the cap refuses some [UT-080]", async () => {
    const user = userEvent.setup();
    const crew = Array.from({ length: 5 }, (_, index) =>
      agentInstanceFixture({
        id: `i-${index}`,
        label: `Agent ${index}`,
        ...(index < 3 ? { state: "paused", pause_reason: "unattended" } : {}),
      })
    );
    const cap =
      "You have 10 active stays-on agents. Pause one or raise `agents.stays_on.max_active_instances`.";
    mockApi.resume.mockImplementation(async (instance: AgentInstancePayload) => {
      if (instance.id !== "i-0") throw new Error(cap);
    });
    renderRow(crew, { limit: 3, scopeName: "compozy" });

    await user.click(screen.getByRole("button", { name: "Show all 5 agents in compozy" }));
    await user.click(screen.getByRole("button", { name: "Resume all" }));

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(
      `Agent 1 and Agent 2 didn't resume. ${cap.replaceAll("`", "")}`
    );
    // The config key reads as code, never with literal backticks.
    expect(within(alert).getByText("agents.stays_on.max_active_instances").tagName).toBe("CODE");
    expect(mockApi.resume).toHaveBeenCalledTimes(3);
  });

  it("Should drop an agent from the Resume all error once it resumes elsewhere [DFE2-4]", async () => {
    const user = userEvent.setup();
    const crew = Array.from({ length: 5 }, (_, index) =>
      agentInstanceFixture({
        id: `i-${index}`,
        label: `Agent ${index}`,
        ...(index < 3 ? { state: "paused", pause_reason: "unattended" } : {}),
      })
    );
    mockApi.resume.mockImplementation(async (instance: AgentInstancePayload) => {
      if (instance.id !== "i-0") throw new Error("At the cap.");
    });
    const view = (instances: readonly AgentInstancePayload[]) =>
      wrap(
        <StaysOnRow instances={instances} limit={3} onOpenSession={vi.fn()} scopeName="compozy" />
      );
    const { rerender } = render(view(crew));

    await user.click(screen.getByRole("button", { name: "Show all 5 agents in compozy" }));
    await user.click(screen.getByRole("button", { name: "Resume all" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Agent 1 and Agent 2 didn't resume. At the cap."
    );

    const active = (id: string) => (instance: AgentInstancePayload) =>
      instance.id === id
        ? { ...instance, state: "active" as const, pause_reason: "" as const }
        : instance;
    const afterOne = crew.map(active("i-0")).map(active("i-1"));
    rerender(view(afterOne));
    expect(screen.getByRole("alert")).toHaveTextContent("Agent 2 didn't resume. At the cap.");

    rerender(view(afterOne.map(active("i-2"))));
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("Should offer no Resume all for a single brake pause", async () => {
    const user = userEvent.setup();
    const crew = Array.from({ length: 5 }, (_, index) =>
      agentInstanceFixture({
        id: `i-${index}`,
        label: `Agent ${index}`,
        ...(index === 0 ? { state: "paused", pause_reason: "unattended" } : {}),
      })
    );
    renderRow(crew, { limit: 3, scopeName: "compozy" });

    await user.click(screen.getByRole("button", { name: "Show all 5 agents in compozy" }));

    expect(screen.queryByRole("button", { name: "Resume all" })).toBeNull();
  });

  it("Should say what wakes it next, and what waits while it's paused [UT-060]", async () => {
    const user = userEvent.setup();
    const sleeping = agentInstanceFixture({
      id: "a",
      next_wake: { kind: "pull_request", description: "Check CI on #1166" },
    });
    const paused = agentInstanceFixture({
      id: "b",
      label: "Paused",
      state: "paused",
      pause_reason: "unattended",
      unattended_turn_limit: 300,
      held_wakes: 2,
    });
    renderRow([sleeping, paused]);

    await user.hover(screen.getByTestId("stays-on-seat-a"));
    expect(await screen.findByText("Wakes up next")).toBeInTheDocument();
    expect(screen.getByText("Check CI on #1166")).toBeInTheDocument();
    await user.unhover(screen.getByTestId("stays-on-seat-a"));

    await user.hover(screen.getByTestId("stays-on-seat-b"));
    expect(await screen.findByText("Paused after 300 steps without you.")).toBeInTheDocument();
    expect(screen.getByText("2 things are waiting for you to resume")).toBeInTheDocument();
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

  it("Should open the agent and its check-ins in the agent's own profile [#1183]", async () => {
    const user = userEvent.setup();
    const instance = agentInstanceFixture({ profile_id: "prof-ops", profile_name: "ops" });
    renderRow([instance]);
    const seat = screen.getByTestId(`stays-on-seat-${instance.id}`);

    await user.pointer({ keys: "[MouseRight]", target: seat });
    await user.click(await screen.findByRole("menuitem", { name: "Open agent" }));
    await user.pointer({ keys: "[MouseRight]", target: seat });
    await user.click(await screen.findByRole("menuitem", { name: "Check-ins" }));

    const search = { workspace: "ws-compozy", profile: "ops" };
    expect(mockNavigate).toHaveBeenNthCalledWith(1, {
      to: "/agents/$name",
      params: { name: "triage" },
      search,
    });
    expect(mockNavigate).toHaveBeenNthCalledWith(2, {
      to: "/agents/$name/$page",
      params: { name: "triage", page: "check-ins" },
      search,
    });
  });
});
