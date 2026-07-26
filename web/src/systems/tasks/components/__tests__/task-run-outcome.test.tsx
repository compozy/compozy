import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { buildTaskRunDetailFixture } from "../../mocks/fixtures";
import { TaskRunOutcome } from "../task-run-outcome";

describe("TaskRunOutcome", () => {
  it.each([
    { result: null, message: "No result was recorded." },
    { result: { summary: "done" }, message: "The result is ready below." },
  ])("Should describe completed result availability truthfully", ({ result, message }) => {
    render(
      <TaskRunOutcome
        nextAttempt={null}
        run={buildTaskRunDetailFixture({
          run: {
            ended_at: "2026-04-17T18:01:00Z",
            result,
            status: "completed",
          } as never,
        })}
      />
    );

    expect(screen.getByTestId("tasks-run-outcome-completed")).toHaveTextContent(message);
  });
});
