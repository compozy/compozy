// Invariant: while a run loads, the run page already has its shape — one named loading area for
// the content, and a head that already offers the way back — and nothing of the loaded page.
// Owning layer: Tasks run location composition. Boundary OUT: the run's own reads.
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { renderWithTopbar } from "@/test/render-with-topbar";

const mocks = vi.hoisted(() => ({
  controller: {
    backToTask: vi.fn(),
    backToTasks: vi.fn(),
    page: { runLoading: true, notFound: false, run: null, runError: null },
    record: null,
  },
}));

vi.mock("../use-task-run-location", () => ({
  useTaskRunLocation: () => mocks.controller,
}));

import { TaskRunLocation } from "../task-run-location";

describe("TaskRunLocation loading", () => {
  it("Should stand in with the run's shape and a head that offers the way back", async () => {
    const user = userEvent.setup();
    renderWithTopbar(<TaskRunLocation runId="run_1" taskId="task_1" />);

    const region = screen.getByRole("status");
    expect(region).toBe(screen.getByTestId("tasks-run-detail-loading"));
    expect(region).toHaveTextContent("Loading run");
    // The main column and the rail already stand under their real headings.
    for (const heading of ["Result", "Run activity", "Session", "Timing", "Other attempts"]) {
      expect(within(region).getByText(heading)).toBeInTheDocument();
    }
    expect(screen.queryByTestId("tasks-run-detail-content")).not.toBeInTheDocument();
    expect(screen.queryByTestId("tasks-run-detail-not-found")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Tasks" }));
    expect(mocks.controller.backToTasks).toHaveBeenCalledTimes(1);
  });
});
