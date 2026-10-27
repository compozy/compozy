// Suite: agent detail location while the agent loads
// Invariant: the page keeps its real chrome while the agent loads: the tab list stays live and the
//   open tab shows the placeholder of its own content, so tabs never wait on the agent read.
// Boundary IN: the location's frame, tab list and per-tab skeletons.
// Boundary OUT: the detail view model and each tab's loaded content, owned by their suites.
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  setTab: vi.fn(),
  tab: "overview" as "overview" | "instructions" | "configuration" | "sessions",
  file: "soul" as "agent" | "soul" | "heartbeat",
}));

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children }: { children: React.ReactNode }) => <a href="/">{children}</a>,
}));

vi.mock("../use-agent-detail", () => ({
  useAgentDetail: () => ({
    agent: undefined,
    agentLoading: true,
    agentError: null,
    metricsLoading: true,
    metricsUnavailable: true,
    sessionsTotal: 0,
    activeSessionsTotal: 0,
    search: { tab: mocks.tab, file: mocks.file, filter: "all" },
    setTab: mocks.setTab,
    setFile: vi.fn(),
    setFilter: vi.fn(),
    onBackToAgents: vi.fn(),
  }),
}));

vi.mock("../use-agent-detail-topbar", () => ({ useAgentDetailTopbar: () => undefined }));

vi.mock("@/systems/agent/hooks/use-agent-window-scope", () => ({
  useAgentWindowScope: () => ({ projectId: "ws_alpha" }),
}));

import { AgentDetailLocation } from "../agent-detail-location";

describe("AgentDetailLocation while the agent loads", () => {
  beforeEach(() => {
    mocks.setTab.mockReset();
    mocks.tab = "overview";
    mocks.file = "soul";
  });

  it("Should keep the real tab list live over the overview placeholder", async () => {
    render(<AgentDetailLocation name="fraud-ops" rawSearch={{}} />);

    const page = screen.getByTestId("agent-detail-loading");
    expect(within(page).getByRole("status")).toHaveTextContent("Loading agent");
    const tabs = within(page).getAllByRole("tab");
    expect(tabs.map(tab => tab.textContent)).toEqual([
      "Overview",
      "Instructions",
      "Configuration",
      "Sessions",
    ]);
    // The tile labels are real; only their values wait.
    expect(within(page).getByText("Time working")).toBeVisible();
    expect(screen.queryByTestId("agent-detail-page")).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole("tab", { name: "Sessions" }));
    expect(mocks.setTab).toHaveBeenCalledWith("sessions");
  });

  it.each([
    { tab: "sessions" as const, marker: "agent-sessions-loading" },
    { tab: "configuration" as const, marker: "agent-configuration-skeleton" },
    { tab: "instructions" as const, marker: "agent-instructions-skeleton" },
  ])("Should show the $tab tab's own placeholder", ({ tab, marker }) => {
    mocks.tab = tab;

    render(<AgentDetailLocation name="fraud-ops" rawSearch={{}} />);

    expect(screen.getByTestId(marker)).toBeInTheDocument();
    expect(screen.queryByTestId("agent-overview-skeleton")).not.toBeInTheDocument();
  });
});
