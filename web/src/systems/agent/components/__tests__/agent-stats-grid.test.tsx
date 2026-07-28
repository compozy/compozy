import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { AgentStatsGrid } from "../agent-stats-grid";
import { formatAgentRuntimeDuration } from "../../lib/format-agent-runtime-duration";

describe("AgentStatsGrid", () => {
  it("Should render Active, Runtime, Failed, and Time-backed Last activity for Overview", () => {
    render(
      <AgentStatsGrid
        active={7}
        runtimeLabel="4h 12m"
        failed={1}
        lastActivityAt="2026-07-11T12:00:00Z"
        sessionsTotal={205}
      />
    );

    expect(within(screen.getByTestId("agent-stat-active")).getByText("7")).toBeInTheDocument();
    expect(
      within(screen.getByTestId("agent-stat-runtime")).getByText("4h 12m")
    ).toBeInTheDocument();
    expect(within(screen.getByTestId("agent-stat-failed")).getByText("1")).toBeInTheDocument();
    const lastActivity = within(screen.getByTestId("agent-stat-last-activity")).getByRole("time", {
      hidden: true,
    });
    expect(lastActivity).toHaveAttribute("datetime", "2026-07-11T12:00:00Z");
    expect(lastActivity).toHaveAttribute("data-mode", "relative");
    expect(lastActivity).toHaveAttribute("title");
    expect(screen.queryByTestId("agent-stat-total")).not.toBeInTheDocument();
    expect(screen.queryByTestId("agent-stat-resumable")).not.toBeInTheDocument();
  });

  it("Should render Total instead of Last activity in the Sessions variant", () => {
    render(
      <AgentStatsGrid
        variant="sessions"
        active={2}
        runtimeLabel="12m"
        failed={0}
        sessionsTotal={6}
      />
    );

    expect(within(screen.getByTestId("agent-stat-total")).getByText("6")).toBeInTheDocument();
    expect(screen.queryByTestId("agent-stat-last-activity")).not.toBeInTheDocument();
  });

  it("Should dash all four metrics when catalog aggregates are unavailable", () => {
    render(
      <AgentStatsGrid
        active={2}
        runtimeLabel={null}
        failed={null}
        lastActivityAt={null}
        metricsAvailable={false}
        sessionsTotal={40}
      />
    );

    expect(within(screen.getByTestId("agent-stat-active")).getByText("—")).toBeInTheDocument();
    expect(within(screen.getByTestId("agent-stat-runtime")).getByText("—")).toBeInTheDocument();
    expect(within(screen.getByTestId("agent-stat-failed")).getByText("—")).toBeInTheDocument();
    expect(
      within(screen.getByTestId("agent-stat-last-activity")).getByText("—")
    ).toBeInTheDocument();
    expect(screen.queryByText("Never")).not.toBeInTheDocument();
    expect(screen.queryByRole("time", { hidden: true })).not.toBeInTheDocument();
  });

  it("Should render Never and zero only after a successful empty aggregate", () => {
    render(
      <AgentStatsGrid
        active={0}
        runtimeLabel="0s"
        failed={0}
        lastActivityAt={null}
        metricsAvailable
        sessionsTotal={0}
      />
    );

    expect(within(screen.getByTestId("agent-stat-active")).getByText("0")).toBeInTheDocument();
    expect(within(screen.getByTestId("agent-stat-runtime")).getByText("0s")).toBeInTheDocument();
    expect(within(screen.getByTestId("agent-stat-failed")).getByText("0")).toBeInTheDocument();
    expect(
      within(screen.getByTestId("agent-stat-last-activity")).getByText("Never")
    ).toBeInTheDocument();
    expect(screen.queryByRole("time", { hidden: true })).not.toBeInTheDocument();
  });

  it("Should format runtime durations for display", () => {
    expect(formatAgentRuntimeDuration(0)).toBe("0s");
    expect(formatAgentRuntimeDuration(45)).toBe("45s");
    expect(formatAgentRuntimeDuration(125)).toBe("2m 5s");
    expect(formatAgentRuntimeDuration(3720)).toBe("1h 2m");
    expect(formatAgentRuntimeDuration(90_061)).toBe("1d 1h");
  });
});
