import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { buildTaskInspectFixture, buildTaskRunDetailFixture } from "../../mocks/fixtures";
import { TaskRunInspectDrawer } from "../task-run-inspect-drawer";

describe("TaskRunInspectDrawer", () => {
  it("Should render heartbeat time neutrally and normalize the truncated claim hash", () => {
    const baseInspect = buildTaskInspectFixture();
    const { container } = render(
      <TaskRunInspectDrawer
        inspect={
          buildTaskInspectFixture({
            current_run: {
              ...baseInspect.current_run,
              heartbeat_at: "2026-04-17T18:00:00Z",
            },
            target: "run",
          } as never) as never
        }
        onOpenChange={vi.fn()}
        open
        run={buildTaskRunDetailFixture()}
      />
    );

    expect(screen.getByText("sha256 · abcdef12")).toBeInTheDocument();
    expect(screen.queryByText(/sha256 · sha256:/)).toBeNull();
    expect(container.querySelector('[data-slot="status-dot"]')).toBeNull();
  });

  it("Should distinguish inspection loading and error states from an empty snapshot", () => {
    const { rerender } = render(
      <TaskRunInspectDrawer
        inspect={null}
        isLoading
        onOpenChange={vi.fn()}
        open
        run={buildTaskRunDetailFixture()}
      />
    );
    expect(screen.getByRole("status", { name: "Loading run inspection" })).toBeInTheDocument();

    rerender(
      <TaskRunInspectDrawer
        errorMessage="Inspection failed"
        inspect={null}
        onOpenChange={vi.fn()}
        open
        run={buildTaskRunDetailFixture()}
      />
    );
    expect(screen.getByRole("alert")).toHaveTextContent("Inspection failed");
    expect(screen.queryByText("No inspect snapshot is available.")).toBeNull();
  });
});
