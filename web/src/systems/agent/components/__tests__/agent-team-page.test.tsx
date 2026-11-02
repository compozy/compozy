// Suite: the Agents page
// Invariant: Stays on comes first and only when someone stays on; both views show the same groups.
// Boundary IN: `AgentTeamPage` from a member list. Boundary OUT: the query, owned by the OS hook.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps } from "react";
import { describe, expect, it, vi } from "vitest";

import { TEAM_WORKSPACES, teamInstances, teamItems } from "../../mocks/team-fixtures";
import { agentStaysOn, buildAgentTeam } from "../../lib/agent-team";
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
  it("Should show Stays on first, with one card per agent", () => {
    renderPage();
    const sections = screen.getAllByTestId(/^agents-section-/);
    expect(sections.map(section => section.dataset.testid)).toEqual([
      "agents-section-stays-on",
      "agents-section-on-demand",
    ]);
    expect(within(sections[0]!).getByText("Triage Lead")).toBeVisible();
    expect(screen.getAllByText("Triage Lead")).toHaveLength(1);
  });

  it("Should replace the Stays on section with a welcome when nobody stays on", async () => {
    const { onCreate } = renderPage({
      members: members.filter(member => !agentStaysOn(member)),
    });
    expect(screen.queryByTestId("agents-section-stays-on")).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "New stays-on agent" }));
    expect(onCreate).toHaveBeenCalledWith({ staysOn: true });
  });

  it("Should drop the welcome while a filter is on", () => {
    renderPage({ members: members.filter(member => !agentStaysOn(member)), unfiltered: false });
    expect(screen.queryByTestId("agents-stays-on-welcome")).not.toBeInTheDocument();
  });

  it("Should show the same groups in rows", () => {
    renderPage({ view: "rows" });
    expect(screen.getByTestId("agents-section-stays-on")).toBeVisible();
    expect(screen.getByTestId("agent-row-general")).toBeVisible();
  });

  it("Should open the profile from a card", async () => {
    const { onOpen } = renderPage();
    await userEvent.click(screen.getByTestId("agent-fleet-row-link-reviewer"));
    expect(onOpen).toHaveBeenCalledWith(expect.objectContaining({ name: "reviewer" }));
  });
});
