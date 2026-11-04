// Suite: the Agents page
// Invariant: Always-on comes first and only when an agent is always-on; both views show the same groups.
// Boundary IN: `AgentTeamPage` from a member list. Boundary OUT: the query, owned by the OS hook.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps } from "react";
import { describe, expect, it, vi } from "vitest";

import { TEAM_WORKSPACES, teamInstances, teamItems } from "../../mocks/team-fixtures";
import { agentAlwaysOn, buildAgentTeam } from "../../lib/agent-team";
import { AgentTeamPage } from "../agent-team/agent-team-page";

const members = buildAgentTeam({
  items: teamItems(),
  instances: teamInstances(),
  workspaces: TEAM_WORKSPACES,
});

function renderPage(overrides: Partial<ComponentProps<typeof AgentTeamPage>> = {}) {
  const onCreate = vi.fn();
  const onOpen = vi.fn();
  render(
    <QueryClientProvider client={new QueryClient()}>
      <AgentTeamPage
        actions={{
          add: {
            capacity: { limit: 10, active: 2, reached: false },
            onAdd: vi.fn(),
            onSignIn: vi.fn(),
            places: [],
            signedOut: new Map(),
          },
          newSessionDisabled: false,
          onNewSession: vi.fn(),
          onOpen,
        }}
        members={members}
        onClearFilters={vi.fn()}
        onCreate={onCreate}
        status="ready"
        unfiltered
        view="cards"
        {...overrides}
      />
    </QueryClientProvider>
  );
  return { onCreate, onOpen };
}

describe("AgentTeamPage", () => {
  it("Should show Always-on first, with one card per agent", () => {
    renderPage();
    const sections = screen.getAllByTestId(/^agents-section-/);
    expect(sections.map(section => section.dataset.testid)).toEqual([
      "agents-section-always-on",
      "agents-section-on-demand",
    ]);
    expect(within(sections[0]!).getByText("Triage Lead")).toBeVisible();
    expect(within(sections[0]!).getByText("Always-on")).toBeVisible();
    expect(within(sections[1]!).getByText("On-demand")).toBeVisible();
    expect(screen.queryByText(/Stays on/)).toBeNull();
    expect(screen.getAllByText("Triage Lead")).toHaveLength(1);
  });

  it("Should replace the Always-on section with a welcome when there are no always-on agents", async () => {
    const { onCreate } = renderPage({
      members: members.filter(member => !agentAlwaysOn(member)),
    });
    expect(screen.queryByTestId("agents-section-always-on")).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "New always-on agent" }));
    expect(onCreate).toHaveBeenCalledWith({ alwaysOn: true });
  });

  it("Should drop the welcome while a filter is on", () => {
    renderPage({ members: members.filter(member => !agentAlwaysOn(member)), unfiltered: false });
    expect(screen.queryByTestId("agents-always-on-welcome")).not.toBeInTheDocument();
  });

  it("Should show the same groups in rows", () => {
    renderPage({ view: "rows" });
    expect(screen.getByTestId("agents-section-always-on")).toBeVisible();
    expect(screen.getByTestId("agent-row-general")).toBeVisible();
  });

  it("Should open the profile from a card", async () => {
    const { onOpen } = renderPage();
    await userEvent.click(screen.getByTestId("agent-fleet-row-link-reviewer"));
    expect(onOpen).toHaveBeenCalledWith(expect.objectContaining({ name: "reviewer" }));
  });
});
