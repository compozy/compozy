import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("@tanstack/react-router", async importOriginal => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    Link: ({ to, params, children, ...props }: Record<string, unknown>) => (
      <a
        href={typeof to === "string" ? to : "#"}
        data-params={JSON.stringify(params)}
        {...(props as Record<string, unknown>)}
      >
        {children as React.ReactNode}
      </a>
    ),
  };
});

const { TaskLoopProvenance } = await import("../task-loop-provenance");
type TaskLoopProvenanceData = import("../../lib/task-loop-identity").TaskLoopProvenance;

/**
 * Owns the invariant that Loop provenance on a task detail page is read from
 * projected fields and never recovered from the task id, and that a record whose
 * run is gone still renders — truthfully, and without a dead link. Absorbs the
 * regressions from the deleted `parseLoopTaskId` id-regex suite.
 */
describe("TaskLoopProvenance", () => {
  const cell: TaskLoopProvenanceData = {
    role: "cell",
    run_id: "looprun-8f3ab2c1d4e5f607",
    loop_name: "revisao-paralela",
    generation: 1,
    node_id: "revisor-perf",
    item_index: 0,
  };

  it("Should name the record, its loop and its position, and link back to the run", () => {
    render(<TaskLoopProvenance loop={cell} />);

    expect(screen.getByTestId("task-loop-provenance")).toHaveTextContent("Loop step");
    expect(screen.getByText("revisao-paralela")).toBeInTheDocument();
    expect(screen.getByText("Round").parentElement).toHaveTextContent("1");
    expect(screen.getByText("Step").parentElement).toHaveTextContent("revisor-perf");
    expect(screen.getByText("Item").parentElement).toHaveTextContent("0");

    const link = screen.getByTestId("task-loop-provenance-open-run");
    expect(link).toHaveTextContent("Open run");
    expect(link).toHaveAttribute("href", "/loop-runs/$runId");
    expect(link).toHaveAttribute(
      "data-params",
      JSON.stringify({ runId: "looprun-8f3ab2c1d4e5f607" })
    );
  });

  it("Should label the parent record a loop run without inventing step fields", () => {
    render(
      <TaskLoopProvenance
        loop={{
          role: "coordinator",
          run_id: "looprun-8f3ab2c1d4e5f607",
          loop_name: "revisao-paralela",
        }}
      />
    );

    expect(screen.getByTestId("task-loop-provenance")).toHaveTextContent("Loop run");
    expect(screen.queryByText("Round")).toBeNull();
    expect(screen.queryByText("Step")).toBeNull();
    expect(screen.queryByText("Item")).toBeNull();
    expect(screen.getByTestId("task-loop-provenance-open-run")).toBeInTheDocument();
  });

  // UT-042
  it("Should state the run is gone instead of linking when the loop name is unrecoverable", () => {
    render(
      <TaskLoopProvenance
        loop={{ role: "cell", run_id: "looprun-77aa01b2c3d4e5f6", generation: 2 }}
      />
    );

    expect(screen.getByTestId("task-loop-provenance-run-gone")).toHaveTextContent(
      "Run no longer available"
    );
    expect(screen.queryByTestId("task-loop-provenance-open-run")).toBeNull();
    expect(screen.queryByRole("link")).toBeNull();
    // The record keeps its provenance — the run id still identifies it.
    expect(screen.getByText("looprun-77aa01b2c3d4e5f6")).toBeInTheDocument();
    expect(screen.getByText("Round").parentElement).toHaveTextContent("2");
  });
});
