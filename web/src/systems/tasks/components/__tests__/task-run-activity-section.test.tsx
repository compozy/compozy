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
  });

  it("Should explain when the current run has no activity yet", () => {
    render(<TaskRunActivitySection isLive runId="run_current" timeline={[]} />);

    expect(screen.getByText("No events recorded for this attempt yet.")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-run-activity-live")).toBeInTheDocument();
  });
});
