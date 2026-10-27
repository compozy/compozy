import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { buildTaskTimelineItemFixture } from "../../mocks/fixtures";
import { TaskRunActivitySection } from "../task-run-activity-section";

describe("TaskRunActivitySection", () => {
  it("Should render only activity owned by the current run", () => {
    render(
      <TaskRunActivitySection
        isLive={false}
        runId="run_current"
        timeline={[
          buildTaskTimelineItemFixture({
            event_id: "evt_current",
            payload: { message: "Current attempt event" },
            run: { id: "run_current" } as never,
          }),
          buildTaskTimelineItemFixture({
            event_id: "evt_sibling",
            payload: { message: "Sibling attempt event" },
            run: { id: "run_sibling" } as never,
          }),
          buildTaskTimelineItemFixture({
            event_id: "evt_task",
            payload: { message: "Task-wide event" },
            run: null,
          }),
        ]}
      />
    );

    expect(screen.getByText("Current attempt event")).toBeInTheDocument();
    expect(screen.queryByText("Sibling attempt event")).toBeNull();
    expect(screen.queryByText("Task-wide event")).toBeNull();
    // TimelineEvent rows are list items; they only lay out inside the Timeline list.
    const timeline = screen.getByRole("list", { name: "Run activity events" });
    expect(timeline).toContainElement(screen.getByText("Current attempt event"));
  });

  it("Should hold the feed's card as a named loading area until the activity arrives", () => {
    render(<TaskRunActivitySection isLive={false} isLoading runId="run_current" timeline={[]} />);

    const region = screen.getByRole("status");
    expect(region).toBe(screen.getByTestId("tasks-run-activity-loading"));
    expect(region).toHaveTextContent("Loading run activity");
    expect(screen.getByText("Run activity")).toBeInTheDocument();
    expect(screen.queryByText("No events recorded for this attempt yet.")).toBeNull();
  });

  it("Should explain when the current run has no activity yet", () => {
    render(<TaskRunActivitySection isLive runId="run_current" timeline={[]} />);

    expect(screen.getByText("No events recorded for this attempt yet.")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-run-activity-live")).toBeInTheDocument();
  });
});
