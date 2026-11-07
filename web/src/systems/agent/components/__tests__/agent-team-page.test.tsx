// Suite: the Agents page
// Invariant: Always-on comes first and only when an agent is always-on; both views show the same groups.
// Boundary IN: `AgentTeamPage` from a member list. Boundary OUT: the query, owned by the OS hook.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { handlers as profileHandlers } from "@/systems/profiles/mocks";
import { setupServer } from "msw/node";
import type { ComponentProps } from "react";
import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { agentInstanceFixture } from "../../mocks/instance-fixtures";
import {
  TEAM_WORKSPACES,
  teamInstances,
  teamItemFixture,
  teamItems,
} from "../../mocks/team-fixtures";
import { agentAlwaysOn, buildAgentTeam } from "../../lib/agent-team";
import { AgentTeamPage } from "../agent-team/agent-team-page";

const server = setupServer(
  ...profileHandlers,
  http.post("*/api/agents/activity", async ({ request }) => {
    const input = (await request.json()) as { items: unknown[] };
    return HttpResponse.json({
      activities: input.items.map(() => ({
        reports_by_day: [],
        busy_by_hour: [],
        cost_by_project: [],
      })),
    });
  })
);
beforeAll(() => server.listen());
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

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
  it.each(["cards", "rows"] as const)(
    "Should batch pulse reads for all visible members in %s",
    async view => {
      const requests: unknown[] = [];
      server.use(
        http.post("*/api/agents/activity", async ({ request }) => {
          const input = (await request.json()) as {
            items: { name: string; workspaces: string[] }[];
          };
          requests.push(input);
          return HttpResponse.json({
            activities: input.items.map(item => ({
              reports_by_day: item.workspaces.includes("other-project")
                ? []
                : [{ day: "2026-11-01", quiet: 1, told: 2, needed_you: 0 }],
              busy_by_hour: [],
              cost_by_project: [],
            })),
          });
        })
      );
      const alwaysOn = members.find(agentAlwaysOn)!;
      const selected = [
        alwaysOn,
        {
          ...alwaysOn,
          key: "other-definition",
          label: "Other definition",
          scopes: ["other-project"],
          places: [],
        },
      ];
      renderPage({ view, members: selected });
      await waitFor(() => expect(requests).toHaveLength(1));
      expect(requests[0]).toEqual({
        days: 14,
        items: [...selected]
          .sort((a, b) => a.key.localeCompare(b.key))
          .map(member => ({
            name: member.name,
            workspaces: [
              ...new Set([...member.scopes, ...member.places.map(place => place.scope)]),
            ].sort(),
          })),
      });
      const pulse = await screen.findByRole("img", {
        name:
          view === "cards"
            ? `${alwaysOn.label}'s activity over the last 2 weeks: 2 reports in 2 weeks`
            : `${alwaysOn.label}'s activity over the last 2 weeks`,
      });
      expect(pulse).toBeVisible();
      expect(screen.queryByRole("img", { name: /^Other definition's activity/ })).toBeNull();
      if (view === "cards") {
        // The compact card keeps the recap on hover too, not in a line of its own.
        expect(pulse).toHaveAttribute("title", "2 reports in 2 weeks");
      }
    }
  );

  it("Should read a user-level agent's pulse from the projects it works in [#1340]", async () => {
    const requests: { items: { name: string; workspaces: string[] }[] }[] = [];
    server.use(
      http.post("*/api/agents/activity", async ({ request }) => {
        const input = (await request.json()) as (typeof requests)[number];
        requests.push(input);
        return HttpResponse.json({
          activities: input.items.map(item => ({
            reports_by_day: item.workspaces.includes("ws-compozy")
              ? [{ day: "2026-11-01", quiet: 0, told: 3, needed_you: 0 }]
              : [],
            busy_by_hour: [],
            cost_by_project: [],
          })),
        });
      })
    );
    const [member] = buildAgentTeam({
      items: [teamItemFixture({}, { scopes: ["@user"] })],
      instances: [agentInstanceFixture({ workspace_id: "ws-compozy" })],
      workspaces: TEAM_WORKSPACES,
    });
    renderPage({ members: [member!] });
    expect(
      await screen.findByRole("img", {
        name: `${member!.label}'s activity over the last 2 weeks: 3 reports in 2 weeks`,
      })
    ).toBeVisible();
    expect(requests[0]!.items[0]!.workspaces).toEqual(["@user", "ws-compozy"]);
  });

  it("Should bound batches and skip members with no resolved workspaces", async () => {
    const requests: { items: { name: string; workspaces: string[] }[] }[] = [];
    server.use(
      http.post("*/api/agents/activity", async ({ request }) => {
        const input = (await request.json()) as (typeof requests)[number];
        requests.push(input);
        return HttpResponse.json({
          activities: input.items.map(() => ({
            reports_by_day: [],
            busy_by_hour: [],
            cost_by_project: [],
          })),
        });
      })
    );
    const base = members.find(agentAlwaysOn)!;
    renderPage({
      members: [
        ...Array.from({ length: 101 }, (_, i) => ({
          ...base,
          key: `many-${i}`,
          name: `agent-${i}`,
        })),
        {
          ...base,
          key: "wide",
          name: "wide",
          scopes: Array.from({ length: 101 }, (_, i) => `ws-${i}`),
        },
        { ...base, key: "empty", scopes: [] },
      ],
    });
    // 103 always-on agents page in by 30: 30, 60, 90, then all of them.
    const shown = () => screen.getAllByTestId(/^agent-card-/).length;
    expect(shown()).toBe(30);
    for (const expected of [60, 90, 103]) {
      await userEvent.click(screen.getByRole("button", { name: "Load more agents" }));
      expect(shown()).toBe(expected);
    }
    expect(screen.queryByRole("button", { name: "Load more agents" })).toBeNull();
    await waitFor(() =>
      expect(requests.flatMap(request => request.items).some(item => item.name === "wide")).toBe(
        true
      )
    );
    expect(
      requests.every(
        request =>
          request.items.length <= 100 &&
          request.items.every(
            item => item.workspaces.length > 0 && item.workspaces.length <= 100
          ) &&
          request.items.reduce((sum, item) => sum + item.workspaces.length, 0) <= 5000
      )
    ).toBe(true);
    expect(
      requests
        .flatMap(request => request.items)
        .some(item => item.name === base.name && item.workspaces.length === 0)
    ).toBe(false);
  });
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
