import { fireEvent, render, screen, within } from "@testing-library/react";
import type { ReactNode } from "react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { SessionLifecycleActionHandlers, SessionPayload } from "@/systems/session";
import { primarySessionFixture } from "@/systems/session/testing";
import { AgentSessionsList } from "../agent-sessions-list";

vi.mock("@tanstack/react-router", () => ({
  Link: ({
    children,
    to,
    params,
    ...props
  }: {
    children: ReactNode;
    to: string;
    params?: Record<string, string>;
    [key: string]: unknown;
  }) => {
    const href = params
      ? Object.entries(params).reduce((acc, [key, value]) => acc.replace(`$${key}`, value), to)
      : to;
    return (
      <a href={href} {...props}>
        {children}
      </a>
    );
  },
}));

function makeSession(overrides: Partial<SessionPayload>): SessionPayload {
  return {
    ...primarySessionFixture,
    ...overrides,
  };
}

function sessionActions(): SessionLifecycleActionHandlers {
  return {
    pendingAction: null,
    pendingSessionId: null,
    onArchive: vi.fn(),
    onDelete: vi.fn(),
    onRename: vi.fn(),
    onStop: vi.fn(),
    onUnarchive: vi.fn(),
  };
}

describe("AgentSessionsList", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("renders custom empty-state copy when provided", () => {
    render(
      <AgentSessionsList
        agentName="codex-agent"
        sessions={[]}
        status="ready"
        emptyTitle="No archived sessions"
        emptyDescription="Archived sessions for codex-agent appear after completed work."
      />
    );

    expect(screen.getByTestId("agent-sessions-empty")).toBeInTheDocument();
    expect(screen.getByText("No archived sessions")).toBeInTheDocument();
    expect(
      screen.getByText("Archived sessions for codex-agent appear after completed work.")
    ).toBeInTheDocument();
  });

  it("does not claim the agent has no sessions when only archived rows exist", () => {
    render(
      <AgentSessionsList
        agentName="codex-agent"
        sessions={[]}
        archivedSessions={[
          makeSession({
            id: "sess_archived_only",
            state: "stopped",
            badge: "stopped",
            archived_at: "2026-08-04T12:00:00Z",
          }),
        ]}
        status="ready"
      />
    );

    expect(screen.queryByTestId("agent-sessions-empty")).toBeNull();
    expect(screen.getByTestId("agent-sessions-archived")).toBeInTheDocument();
  });

  it("renders the daemon-generated session name instead of fallback identity", () => {
    render(
      <AgentSessionsList
        agentName="codex-agent"
        sessions={[makeSession({ id: "sess_auto_title", name: "Checkout Retry Fencing" })]}
        status="ready"
      />
    );

    const row = screen.getByTestId("agent-session-row-sess_auto_title");
    expect(within(row).getByText("Checkout Retry Fencing")).toBeInTheDocument();
    expect(screen.queryByText("New session")).not.toBeInTheDocument();
  });

  it("formats relative times against one render-pass timestamp", () => {
    vi.spyOn(Date, "now").mockReturnValue(Date.parse("2026-04-17T18:11:00Z"));
    const sessions = [
      makeSession({
        id: "sess_one",
        updated_at: "2026-04-17T18:10:30Z",
        activity: {
          elapsed_ms: 60_000,
          elapsed_seconds: 60,
          idle_seconds: 0,
          iteration_current: 1,
          iteration_max: 2,
          last_activity_at: "2026-04-17T18:10:30Z",
        },
      }),
      makeSession({
        id: "sess_two",
        updated_at: "2026-04-17T18:10:30Z",
        activity: {
          elapsed_ms: 60_000,
          elapsed_seconds: 60,
          idle_seconds: 0,
          iteration_current: 1,
          iteration_max: 2,
          last_activity_at: "2026-04-17T18:10:30Z",
        },
      }),
    ];

    render(<AgentSessionsList agentName="codex-agent" sessions={sessions} status="ready" />);

    expect(
      within(screen.getByTestId("agent-session-row-sess_one")).getByText("just now")
    ).toBeInTheDocument();
    expect(
      within(screen.getByTestId("agent-session-row-sess_two")).getByText("just now")
    ).toBeInTheDocument();
    expect(Date.now).toHaveBeenCalledTimes(1);
  });

  it("renders zero elapsed duration as zero seconds", () => {
    const sessions = [
      makeSession({
        id: "sess_zero_duration",
        activity: {
          elapsed_ms: 0,
          elapsed_seconds: 0,
          idle_seconds: 0,
          iteration_current: 0,
          iteration_max: 4,
          last_activity_at: "2026-04-17T18:10:30Z",
        },
      }),
    ];

    render(<AgentSessionsList agentName="codex-agent" sessions={sessions} status="ready" />);

    expect(
      within(screen.getByTestId("agent-session-row-sess_zero_duration")).getByText("0s")
    ).toBeInTheDocument();
  });

  it("shows a running status spinner from daemon activity truth", () => {
    render(
      <AgentSessionsList
        agentName="codex-agent"
        sessions={[
          makeSession({
            id: "sess_user_running",
            type: "user",
            state: "active",
            badge: "idle",
            activity: {
              turn_id: "turn_001",
              elapsed_ms: 4_000,
              elapsed_seconds: 4,
              idle_seconds: 0,
              iteration_current: 1,
              iteration_max: 4,
            },
          }),
          makeSession({ id: "sess_spawned_running", type: "spawned", badge: "running" }),
          makeSession({ id: "sess_system_running", type: "system", badge: "running" }),
          makeSession({ id: "sess_coordinator_running", type: "coordinator", badge: "running" }),
        ]}
        status="ready"
      />
    );

    const status = screen.getByTestId("agent-session-status-sess_user_running");
    expect(status).toHaveTextContent("RUNNING");
    expect(screen.getByTestId("agent-session-status-sess_spawned_running")).toHaveTextContent(
      "RUNNING"
    );
    expect(screen.getByTestId("agent-session-status-sess_system_running")).toHaveTextContent(
      "RUNNING"
    );
    expect(screen.getByTestId("agent-session-status-sess_coordinator_running")).toHaveTextContent(
      "RUNNING"
    );
  });

  it("does not show the running spinner for idle active or stopped sessions", () => {
    render(
      <AgentSessionsList
        agentName="codex-agent"
        sessions={[
          makeSession({ id: "sess_idle", state: "active", badge: "idle", attachable: true }),
          makeSession({ id: "sess_stopped", state: "stopped", badge: "stopped" }),
        ]}
        status="ready"
      />
    );

    expect(screen.getByTestId("agent-session-status-sess_idle")).toHaveTextContent("ACTIVE");
    expect(screen.getByTestId("agent-session-status-sess_idle")).not.toHaveAttribute("aria-label");
    expect(screen.getByTestId("agent-session-status-sess_stopped")).toHaveTextContent("DONE");
    expect(screen.getByTestId("agent-session-status-sess_stopped")).not.toHaveAttribute(
      "aria-label"
    );
  });

  it("surfaces unhealthy daemon badges without treating them as running", () => {
    render(
      <AgentSessionsList
        agentName="codex-agent"
        sessions={[
          makeSession({ id: "sess_hung", state: "active", badge: "hung" }),
          makeSession({ id: "sess_unhealthy", state: "active", badge: "unhealthy" }),
        ]}
        status="ready"
      />
    );

    expect(screen.getByTestId("agent-session-status-sess_hung")).toHaveTextContent("HUNG");
    expect(screen.getByTestId("agent-session-status-sess_hung")).not.toHaveTextContent("RUNNING");
    expect(screen.getByTestId("agent-session-status-sess_unhealthy")).toHaveTextContent(
      "UNHEALTHY"
    );
    expect(screen.getByTestId("agent-session-status-sess_unhealthy")).not.toHaveTextContent(
      "RUNNING"
    );
  });

  it("does not mask hung or unhealthy sessions with stale activity", () => {
    const staleActivity = {
      turn_id: "turn_stale",
      elapsed_ms: 20_000,
      elapsed_seconds: 20,
      idle_seconds: 0,
      iteration_current: 2,
      iteration_max: 4,
    };

    render(
      <AgentSessionsList
        agentName="codex-agent"
        sessions={[
          makeSession({
            id: "sess_hung_activity",
            state: "active",
            badge: "hung",
            activity: staleActivity,
          }),
          makeSession({
            id: "sess_unhealthy_activity",
            state: "active",
            badge: "unhealthy",
            activity: staleActivity,
          }),
        ]}
        status="ready"
      />
    );

    expect(screen.getByTestId("agent-session-status-sess_hung_activity")).toHaveTextContent("HUNG");
    expect(screen.getByTestId("agent-session-status-sess_hung_activity")).not.toHaveTextContent(
      "RUNNING"
    );
    expect(screen.getByTestId("agent-session-status-sess_unhealthy_activity")).toHaveTextContent(
      "UNHEALTHY"
    );
    expect(
      screen.getByTestId("agent-session-status-sess_unhealthy_activity")
    ).not.toHaveTextContent("RUNNING");
  });

  it("loads the next server page from an accessible loading-aware control", () => {
    const onLoadMore = vi.fn();
    const { rerender } = render(
      <AgentSessionsList
        agentName="codex-agent"
        sessions={[makeSession({ id: "sess_page_one" })]}
        status="ready"
        paginationStatus="available"
        onLoadMore={onLoadMore}
      />
    );

    const button = screen.getByRole("button", { name: "Load more sessions" });
    fireEvent.click(button);
    expect(onLoadMore).toHaveBeenCalledTimes(1);

    rerender(
      <AgentSessionsList
        agentName="codex-agent"
        sessions={[makeSession({ id: "sess_page_one" })]}
        status="ready"
        paginationStatus="loading"
        onLoadMore={onLoadMore}
      />
    );

    expect(screen.getByRole("button", { name: "Loading more sessions" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Loading more sessions" })).toHaveAttribute(
      "aria-busy",
      "true"
    );
  });

  it("keeps archived sessions collapsed and gates each row menu by lifecycle state", async () => {
    const user = userEvent.setup();
    const actions = sessionActions();
    const active = makeSession({ id: "sess_active", state: "active", badge: "running" });
    const stopped = makeSession({ id: "sess_stopped", state: "stopped", badge: "stopped" });
    const archived = makeSession({
      id: "sess_archived",
      state: "stopped",
      badge: "stopped",
      archived_at: "2026-08-04T12:00:00Z",
    });

    render(
      <AgentSessionsList
        agentName="codex-agent"
        sessions={[active, stopped]}
        archivedSessions={[archived]}
        archivedTotal={4}
        sessionActions={actions}
        status="ready"
      />
    );

    fireEvent.click(screen.getByTestId("session-row-actions-sess_active"));
    expect(screen.getByTestId("session-row-stop-sess_active")).toBeInTheDocument();
    expect(screen.queryByTestId("session-row-archive-sess_active")).toBeNull();
    await user.keyboard("{Escape}");

    fireEvent.click(screen.getByTestId("session-row-actions-sess_stopped"));
    expect(screen.getByTestId("session-row-archive-sess_stopped")).toBeInTheDocument();
    expect(screen.queryByTestId("session-row-stop-sess_stopped")).toBeNull();
    await user.keyboard("{Escape}");

    const archivedDisclosure = screen.getByRole("button", { name: "Archived sessions (4)" });
    expect(archivedDisclosure).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByTestId("agent-session-row-sess_archived")).toBeNull();
    await user.click(archivedDisclosure);
    expect(screen.getByTestId("agent-session-row-sess_archived")).toHaveTextContent("ARCHIVED");

    fireEvent.click(screen.getByTestId("session-row-actions-sess_archived"));
    expect(screen.getByTestId("session-row-unarchive-sess_archived")).toBeInTheDocument();
    expect(screen.queryByTestId("session-row-archive-sess_archived")).toBeNull();
    expect(screen.queryByTestId("session-row-stop-sess_archived")).toBeNull();
  });
});
