import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import {
  schedulerAttentionStatusFixture,
  schedulerBacklogFixture,
  schedulerPausedStatusFixture,
  schedulerStatusFixture,
} from "../../mocks";
import { SchedulerControlsPanel } from "../scheduler-controls-panel";

describe("SchedulerControlsPanel", () => {
  it("does not publish scheduler state or enable controls before status loads", () => {
    render(
      <SchedulerControlsPanel
        backlog={null}
        isBacklogLoading
        isLoading
        onDrain={vi.fn()}
        onPause={vi.fn()}
        status={null}
      />
    );

    expect(screen.getByTestId("scheduler-controls-state")).toHaveTextContent("Loading");
    expect(screen.queryByTestId("scheduler-controls-meta")).not.toBeInTheDocument();
    expect(screen.getByTestId("scheduler-controls-pause")).toBeDisabled();
    expect(screen.getByTestId("scheduler-controls-drain")).toBeDisabled();
    expect(screen.getByRole("status", { name: "Loading scheduler status" })).toBeInTheDocument();
    expect(screen.getByRole("status", { name: "Loading scheduler backlog" })).toBeInTheDocument();
  });

  it("renders scheduler status and backlog pressure", () => {
    render(
      <SchedulerControlsPanel backlog={schedulerBacklogFixture} status={schedulerStatusFixture} />
    );

    expect(screen.getByTestId("scheduler-controls-state")).toHaveTextContent("Running");
    expect(screen.getByTestId("scheduler-controls-meta")).toHaveTextContent("1 active claims");
    expect(screen.getByTestId("scheduler-controls-starved-count")).toHaveTextContent(
      "0 starved runs"
    );
    expect(screen.getByTestId("scheduler-controls-needs-attention-count")).toHaveTextContent(
      "0 needs attention"
    );
    expect(screen.getByTestId("scheduler-backlog-total")).toHaveTextContent("2");
    expect(screen.getByTestId("scheduler-backlog-row-run_014")).toBeInTheDocument();
  });

  it("renders scheduler attention pressure with warning emphasis", () => {
    render(
      <SchedulerControlsPanel
        backlog={schedulerBacklogFixture}
        status={schedulerAttentionStatusFixture}
      />
    );

    expect(screen.getByTestId("scheduler-controls-starved-count")).toHaveTextContent(
      "2 starved runs"
    );
    expect(screen.getByTestId("scheduler-controls-starved-count")).toHaveClass("text-warning");
    expect(screen.getByTestId("scheduler-controls-needs-attention-count")).toHaveTextContent(
      "1 needs attention"
    );
    expect(screen.getByTestId("scheduler-controls-needs-attention-count")).toHaveClass(
      "text-warning"
    );
  });

  it("requires a reason before pausing scheduler dispatch", async () => {
    const onPause = vi.fn().mockResolvedValue(undefined);
    render(
      <SchedulerControlsPanel
        backlog={schedulerBacklogFixture}
        onPause={onPause}
        status={schedulerStatusFixture}
      />
    );

    fireEvent.click(screen.getByTestId("scheduler-controls-pause"));
    fireEvent.click(screen.getByTestId("scheduler-controls-pause-confirm"));
    expect(screen.getByTestId("scheduler-controls-pause-error")).toHaveTextContent(
      "Provide a pause reason."
    );
    expect(screen.getByTestId("scheduler-controls-pause-error")).toHaveAttribute("role", "alert");
    expect(screen.getByTestId("scheduler-controls-pause-reason")).toHaveAttribute(
      "aria-describedby",
      "scheduler-controls-pause-error"
    );

    fireEvent.change(screen.getByTestId("scheduler-controls-pause-reason"), {
      target: { value: "provider incident" },
    });
    fireEvent.click(screen.getByTestId("scheduler-controls-pause-confirm"));
    await waitFor(() => {
      expect(onPause).toHaveBeenCalledWith("provider incident");
    });
  });

  it("renders resume action while paused and forwards drain", () => {
    const onResume = vi.fn();
    const onDrain = vi.fn();
    render(
      <SchedulerControlsPanel
        backlog={schedulerBacklogFixture}
        onDrain={onDrain}
        onResume={onResume}
        status={schedulerPausedStatusFixture}
      />
    );

    fireEvent.click(screen.getByTestId("scheduler-controls-resume"));
    fireEvent.click(screen.getByTestId("scheduler-controls-drain"));

    expect(onResume).toHaveBeenCalledTimes(1);
    expect(onDrain).toHaveBeenCalledTimes(1);
  });
});
