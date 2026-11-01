// Suite: the agent's home
// Invariant: an on-demand agent shows no Stays on sections (UT-074), and no route behind activity
//   reads as unknown, never as no activity.
// Boundary IN: `AgentProfileHome` from its view model. Boundary OUT: the side panels and the plan
//   card, which read their own daemon routes.
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { UseAgentProfile } from "../../hooks/use-agent-profile";
import { agentPlaceFromInstance } from "../../lib/agent-team";
import { agentInstanceFixture } from "../../mocks/instance-fixtures";
import { activityFixture } from "../../mocks/team-fixtures";
import { AgentProfileHome } from "../agent-profile/agent-profile-home";

vi.mock("../agent-profile/agent-profile-panels", () => ({ AgentProfilePanels: () => null }));
vi.mock("@/systems/dashboard", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/dashboard")>()),
  HomePulseHeatmap: () => null,
}));
vi.mock("@/systems/provider-usage", () => ({
  ProviderUsageRow: () => null,
  useProviderUsage: () => ({ viewModel: null }),
  useUsageClock: () => 0,
}));
vi.mock("@/systems/profiles", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/profiles")>()),
  useProfileReadScope: () => ({ destination: "default" }),
}));

function pageFixture(overrides: Partial<UseAgentProfile> = {}): UseAgentProfile {
  return {
    identity: {
      name: "triage",
      label: "Triage Lead",
      description: "Watches issues",
      appearance: { avatar: "generated", seed: "triage" },
      provider: "claude",
      staysOn: true,
      readOnly: false,
    },
    places: [agentPlaceFromInstance(agentInstanceFixture(), new Map())],
    state: "idle",
    panel: null,
    setPanel: vi.fn(),
    activity: activityFixture(),
    findings: [],
    work: [],
    diagnostics: [],
    headline: "Resting in 1 project",
    fresh: false,
    capped: false,
    add: { capped: false, onAdd: vi.fn(), onSignIn: vi.fn(), places: [], signedOut: new Map() },
    newSessionDisabled: false,
    summaries: { behavior: "", model: "", checkIns: "Off", memory: "", tools: "" },
    placeActions: { onArchive: vi.fn(), onOpen: vi.fn(), onToggle: vi.fn() },
    onResumeAll: vi.fn(),
    onSaveIdentity: vi.fn(),
    onNewSession: vi.fn(),
    onOpenFinding: vi.fn(),
    onOpenSession: vi.fn(),
    onOpenWhere: vi.fn(),
    onOpenEditor: vi.fn(),
    ...overrides,
  };
}

describe("AgentProfileHome", () => {
  it("Should show where a stays-on agent works and its setup rows", () => {
    render(<AgentProfileHome page={pageFixture()} />);
    expect(screen.getByText("Where it's working")).toBeVisible();
    expect(screen.getByTestId("agent-setup-check-ins")).toBeVisible();
    expect(screen.getByTestId("agent-setup-memory")).toBeVisible();
  });

  it("Should omit every Stays on section for an on-demand agent", () => {
    render(
      <AgentProfileHome
        page={pageFixture({
          identity: { ...pageFixture().identity, staysOn: false },
          places: [],
        })}
      />
    );
    expect(screen.queryByText("Where it's working")).not.toBeInTheDocument();
    expect(screen.queryByTestId("agent-setup-check-ins")).not.toBeInTheDocument();
    expect(screen.queryByTestId("agent-setup-memory")).not.toBeInTheDocument();
    expect(screen.getByTestId("agent-page-new-session")).toBeVisible();
    expect(screen.getByTestId("agent-profile-kind")).toHaveTextContent("On demand");
  });

  it("Should say activity is unavailable when no route answers", () => {
    render(<AgentProfileHome page={pageFixture({ activity: null })} />);
    expect(screen.getByText("Activity isn't available yet")).toBeVisible();
    expect(screen.queryByText("What it cost")).not.toBeInTheDocument();
  });

  it("Should explain a place that can't resume, and offer no Resume", () => {
    const stopped = agentPlaceFromInstance(
      agentInstanceFixture({ state: "paused", pause_reason: "definition_missing" }),
      new Map()
    );
    render(<AgentProfileHome page={pageFixture({ places: [stopped], state: "paused" })} />);
    expect(screen.getByTestId("agent-profile-stopped")).toHaveTextContent("It can't resume");
    expect(screen.queryByRole("button", { name: "Resume" })).not.toBeInTheDocument();
  });
});
