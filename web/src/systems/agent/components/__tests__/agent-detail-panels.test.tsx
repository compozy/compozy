import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { AnchorHTMLAttributes, ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { primaryAgentFixture } from "@/systems/agent/testing";
import type { SessionPayload } from "@/systems/session";
import { primarySessionFixture } from "@/systems/session/testing";

import type { AgentInstructionsTabViewModel } from "../../hooks/use-agent-instructions-tab";
import type { AgentPayload } from "../../types";
import { AgentConfigurationTab } from "../agent-configuration-tab";
import { AgentDiagnosticsBanner } from "../agent-diagnostics-banner";
import { AgentInstructionsTab } from "../agent-instructions-tab";
import { AgentOverviewTab } from "../agent-overview-tab";
import { AgentSessionsTab } from "../agent-sessions-tab";

vi.mock("@tanstack/react-router", () => ({
  Link: ({
    children,
    ...props
  }: AnchorHTMLAttributes<HTMLAnchorElement> & { children: ReactNode }) => (
    <a {...props}>{children}</a>
  ),
}));

function agent(overrides: Partial<AgentPayload> = {}): AgentPayload {
  return {
    ...primaryAgentFixture,
    model: undefined,
    command: undefined,
    skills: { disabled: ["release-notes"] },
    tools: ["agh__task_view"],
    deny_tools: ["agh__task_delete"],
    toolsets: [],
    mcp_servers: [],
    ...overrides,
  };
}

const runtimeControlStub = <div data-testid="agent-detail-runtime">Model control</div>;

function overviewProps(
  overrides: Partial<Parameters<typeof AgentOverviewTab>[0]> = {}
): Parameters<typeof AgentOverviewTab>[0] {
  return {
    agent: agent(),
    sessions: [],
    sessionsTotal: 0,
    activeSessionsTotal: 0,
    failedSessionsTotal: 0,
    runtimeSeconds: 0,
    metricsUnavailable: false,
    metricsLoading: false,
    lastSessionActivityAt: null,
    sessionsLoading: false,
    sessionsError: false,
    runtimeControl: runtimeControlStub,
    onEditRuntime: vi.fn(),
    onViewAllSessions: vi.fn(),
    ...overrides,
  };
}

function session(overrides: Partial<SessionPayload> = {}): SessionPayload {
  return {
    ...primarySessionFixture,
    id: "sess-active",
    name: "Release check",
    state: "active",
    activity: {
      ...primarySessionFixture.activity!,
      iteration_current: 2,
      iteration_max: 10,
      elapsed_seconds: 120,
    },
    ...overrides,
  };
}

function instructionsViewModel(
  overrides: Partial<AgentInstructionsTabViewModel> = {}
): AgentInstructionsTabViewModel {
  return {
    promptWordCount: "~4 words",
    soulMissing: true,
    heartbeatMissing: true,
    soul: {
      resourceKey: '["ws-test","coder","soul"]',
      payload: undefined,
      isLoading: false,
      isError: false,
      history: undefined,
      onValidate: vi.fn(),
      onSave: vi.fn(),
      onRestore: vi.fn(),
      onRetry: vi.fn(),
    },
    heartbeat: {
      resourceKey: '["ws-test","coder","heartbeat"]',
      payload: undefined,
      isLoading: false,
      isError: false,
      history: undefined,
      onValidate: vi.fn(),
      onSave: vi.fn(),
      onRestore: vi.fn(),
      onRetry: vi.fn(),
      status: undefined,
      statusLoading: false,
      statusError: false,
      onRetryStatus: vi.fn(),
      activeSessions: [],
      wakeSessionId: null,
      setWakeSessionId: vi.fn(),
      onWake: vi.fn(),
      waking: false,
      wakeDecision: undefined,
      wakeError: null,
    },
    ...overrides,
  };
}

describe("agent detail panels", () => {
  it("Should render AGENT.md truth, missing badges, and file navigation", async () => {
    const user = userEvent.setup();
    const onFileChange = vi.fn();
    const onEditAgentPrompt = vi.fn();

    render(
      <AgentInstructionsTab
        agent={agent({ prompt: "# Release\n\nShip carefully." })}
        file="agent"
        onFileChange={onFileChange}
        onEditAgentPrompt={onEditAgentPrompt}
        viewModel={instructionsViewModel()}
        onNewSession={vi.fn()}
      />
    );

    expect(screen.getByRole("heading", { name: "Release" })).toBeVisible();
    expect(screen.getByTestId("agent-file-meta")).toHaveTextContent("Read-only here");
    expect(screen.getByTestId("agent-file-soul-missing-badge")).toBeVisible();
    expect(screen.getByTestId("agent-file-heartbeat-missing-badge")).toBeVisible();
    await user.click(screen.getByTestId("agent-file-edit-prompt"));
    await user.click(screen.getByTestId("agent-file-tab-soul"));
    expect(onEditAgentPrompt).toHaveBeenCalledTimes(1);
    expect(onFileChange).toHaveBeenCalledWith("soul");
  });

  it("Should render Overview runtime truth, deep-link editing, and up to three live sessions", async () => {
    const user = userEvent.setup();
    const onEditRuntime = vi.fn();
    render(
      <AgentOverviewTab
        {...overviewProps({
          sessions: [
            session({ activity: { ...primarySessionFixture.activity!, elapsed_seconds: 59.5 } }),
            session({ id: "sess-2" }),
            session({ id: "sess-3" }),
            session({ id: "sess-4" }),
          ],
          sessionsTotal: 4,
          activeSessionsTotal: 4,
          failedSessionsTotal: 0,
          runtimeSeconds: 180,
          lastSessionActivityAt: "2026-04-01T01:00:00Z",
          onEditRuntime,
        })}
      />
    );

    expect(screen.getAllByText("Default")).toHaveLength(2);
    expect(within(screen.getByTestId("agent-overview-runtime")).getByText("Model")).toBeVisible();
    expect(screen.getByTestId("agent-detail-runtime")).toBeVisible();
    expect(screen.queryByText("Provider")).not.toBeInTheDocument();
    expect(screen.getByTestId("agent-overview-skills")).toHaveTextContent("1 skill disabled");
    expect(screen.getAllByTestId(/^agent-overview-live-sess-/)).toHaveLength(3);
    expect(screen.getByTestId("agent-overview-live-sess-active")).toHaveTextContent("59s");
    expect(screen.getByTestId("agent-overview-live-sess-active")).not.toHaveTextContent("1m");
    expect(screen.queryByText(String.fromCharCode(0x2014))).toBeNull();
    await user.click(screen.getByTestId("agent-overview-edit-runtime"));
    expect(onEditRuntime).toHaveBeenCalledTimes(1);
  });

  it("Should keep successful catalog metrics visible when the sessions page fails", () => {
    render(
      <AgentOverviewTab
        {...overviewProps({
          sessionsTotal: 11,
          activeSessionsTotal: 2,
          failedSessionsTotal: 1,
          runtimeSeconds: 90,
          lastSessionActivityAt: "2026-04-01T01:00:00Z",
          sessionsError: true,
        })}
      />
    );
    expect(screen.getByTestId("agent-overview-runtime")).toBeVisible();
    expect(screen.getByTestId("agent-overview-sessions-notice")).toHaveTextContent(
      "Session status unavailable"
    );
    expect(within(screen.getByTestId("agent-stat-active")).getByText("2")).toBeInTheDocument();
    expect(within(screen.getByTestId("agent-stat-failed")).getByText("1")).toBeInTheDocument();
  });

  it("Should dash metrics without turning the live-session list into an error when catalog fails", () => {
    render(
      <AgentOverviewTab
        {...overviewProps({
          sessions: [session()],
          failedSessionsTotal: null,
          runtimeSeconds: null,
          metricsUnavailable: true,
        })}
      />
    );
    expect(screen.queryByTestId("agent-overview-sessions-notice")).not.toBeInTheDocument();
    expect(within(screen.getByTestId("agent-stat-active")).getByText("—")).toBeInTheDocument();
    expect(within(screen.getByTestId("agent-stat-runtime")).getByText("—")).toBeInTheDocument();
    expect(within(screen.getByTestId("agent-stat-failed")).getByText("—")).toBeInTheDocument();
    expect(
      within(screen.getByTestId("agent-stat-last-activity")).getByText("—")
    ).toBeInTheDocument();
    expect(screen.getByTestId("agent-overview-live-sess-active")).toBeInTheDocument();
  });

  it("Should skeleton metrics from the catalog query while sessions rows stay independent", () => {
    render(
      <AgentOverviewTab
        {...overviewProps({
          sessions: [session()],
          failedSessionsTotal: null,
          runtimeSeconds: null,
          metricsUnavailable: true,
          metricsLoading: true,
        })}
      />
    );
    expect(screen.getByTestId("agent-overview-metrics-skeleton").children).toHaveLength(4);
    expect(screen.getByTestId("agent-overview-live-sess-active")).toBeInTheDocument();
  });

  it("Should render Configuration sections and deep-link each Edit action", async () => {
    const user = userEvent.setup();
    const onEditSection = vi.fn();
    render(<AgentConfigurationTab agent={agent()} onEditSection={onEditSection} />);
    expect(screen.getByTestId("agent-config-runtime")).toHaveTextContent("Default");
    expect(screen.queryByText("Provider")).not.toBeInTheDocument();
    expect(screen.getByTestId("agent-config-tools")).toHaveTextContent("agh__task_view");
    expect(screen.getByTestId("agent-config-deny-tools")).toHaveTextContent("agh__task_delete");
    expect(screen.getByTestId("agent-mcp-empty")).toBeVisible();
    await user.click(screen.getByTestId("agent-config-edit-runtime"));
    await user.click(screen.getByTestId("agent-config-edit-access"));
    await user.click(screen.getByTestId("agent-config-edit-mcp"));
    expect(onEditSection.mock.calls.map(call => call[0])).toEqual(["runtime", "access", "mcp"]);
  });

  it("Should synchronize the Sessions filter action and preserve metric skeleton geometry", async () => {
    const user = userEvent.setup();
    const onFilterChange = vi.fn();
    const view = render(
      <AgentSessionsTab
        agentName="coder"
        sessions={[session(), session({ id: "sess-done", state: "stopped" })]}
        total={2}
        active={1}
        failed={0}
        runtimeSeconds={120}
        metricsUnavailable={false}
        metricsLoading
        lastActivityAt="2026-04-01T01:00:00Z"
        status="ready"
        onLoadMore={vi.fn()}
        filter="all"
        onFilterChange={onFilterChange}
        onNewSession={vi.fn()}
        onClearFilter={vi.fn()}
      />
    );
    expect(screen.getByTestId("agent-sessions-metrics-loading").children).toHaveLength(4);
    expect(screen.getByTestId("agent-session-row-sess-done")).toBeInTheDocument();

    view.rerender(
      <AgentSessionsTab
        agentName="coder"
        sessions={[session(), session({ id: "sess-done", state: "stopped" })]}
        total={2}
        active={1}
        failed={0}
        runtimeSeconds={120}
        metricsUnavailable={false}
        metricsLoading={false}
        lastActivityAt="2026-04-01T01:00:00Z"
        status="ready"
        onLoadMore={vi.fn()}
        filter="all"
        onFilterChange={onFilterChange}
        onNewSession={vi.fn()}
        onClearFilter={vi.fn()}
      />
    );
    await user.click(screen.getByTestId("agent-sessions-filter-done"));
    expect(onFilterChange).toHaveBeenCalledWith("done");
  });

  it("Should keep the sessions list ready while catalog metrics are unavailable", () => {
    render(
      <AgentSessionsTab
        agentName="coder"
        sessions={[session()]}
        total={0}
        active={0}
        failed={null}
        runtimeSeconds={null}
        metricsUnavailable
        metricsLoading={false}
        lastActivityAt={null}
        status="ready"
        onLoadMore={vi.fn()}
        filter="all"
        onFilterChange={vi.fn()}
        onNewSession={vi.fn()}
        onClearFilter={vi.fn()}
      />
    );
    expect(within(screen.getByTestId("agent-stat-active")).getByText("—")).toBeInTheDocument();
    expect(within(screen.getByTestId("agent-stat-total")).getByText("—")).toBeInTheDocument();
    expect(screen.getByTestId("agent-session-row-sess-active")).toBeInTheDocument();
  });

  it("Should expose the task-oriented action for each empty sessions state", async () => {
    const user = userEvent.setup();
    const onNewSession = vi.fn();
    const onClearFilter = vi.fn();
    const view = render(
      <AgentSessionsTab
        agentName="coder"
        sessions={[]}
        total={0}
        active={0}
        failed={0}
        runtimeSeconds={0}
        metricsUnavailable={false}
        metricsLoading={false}
        lastActivityAt={null}
        status="ready"
        onLoadMore={vi.fn()}
        filter="all"
        onFilterChange={vi.fn()}
        onNewSession={onNewSession}
        onClearFilter={onClearFilter}
      />
    );

    await user.click(screen.getByTestId("agent-sessions-empty-new"));
    expect(onNewSession).toHaveBeenCalledTimes(1);

    view.rerender(
      <AgentSessionsTab
        agentName="coder"
        sessions={[]}
        total={0}
        active={0}
        failed={0}
        runtimeSeconds={0}
        metricsUnavailable={false}
        metricsLoading={false}
        lastActivityAt={null}
        status="ready"
        onLoadMore={vi.fn()}
        filter="failed"
        onFilterChange={vi.fn()}
        onNewSession={onNewSession}
        onClearFilter={onClearFilter}
      />
    );

    await user.click(screen.getByTestId("agent-sessions-show-all"));
    expect(onClearFilter).toHaveBeenCalledTimes(1);
  });

  it("Should list every diagnostic with kind, message, and source path", () => {
    render(
      <AgentDiagnosticsBanner
        diagnostics={[
          { error_kind: "frontmatter.invalid", message: "Unknown field", path: "AGENT.md:3" },
          { error_kind: "provider.missing", message: "Provider is required", path: "AGENT.md" },
        ]}
      />
    );
    const [frontmatterDiagnostic, providerDiagnostic] =
      screen.getAllByTestId("agent-diagnostic-item");
    expect(frontmatterDiagnostic).toHaveTextContent("frontmatter.invalid");
    expect(frontmatterDiagnostic).toHaveTextContent("Unknown field");
    expect(frontmatterDiagnostic).toHaveTextContent("AGENT.md:3");
    expect(providerDiagnostic).toHaveTextContent("provider.missing");
    expect(providerDiagnostic).toHaveTextContent("Provider is required");
    expect(providerDiagnostic).toHaveTextContent("AGENT.md");
  });
});
