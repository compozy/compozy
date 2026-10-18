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

const { TasksListRow } = await import("../tasks-list-row");
const { TaskLoopRow } = await import("../task-loop-row");
type TaskListItem = import("../../types").TaskListItem;
type TaskLoopProvenance = import("../../lib/task-loop-identity").TaskLoopProvenance;

function buildTask(overrides: Partial<TaskListItem> = {}): TaskListItem {
  return {
    id: "task_abcdef0_tail",
    title: "Summarize feedback",
    identifier: "TASK-1",
    status: "in_progress",
    scope: "workspace",
    origin: { kind: "web", ref: "op" },
    created_at: "2026-04-11T09:00:00Z",
    updated_at: "2026-04-11T09:00:00Z",
    created_by: { kind: "human", ref: "op" },
    ...overrides,
  } as TaskListItem;
}

function inTable(row: React.ReactElement) {
  return (
    <table>
      <tbody>{row}</tbody>
    </table>
  );
}

describe("TasksListRow", () => {
  const baseRow = {
    link: <a aria-label="Open Summarize feedback" href="/tasks/task_xyz" />,
    title: "Summarize feedback",
    state: "running" as const,
    statusLabel: "In progress",
  };

  it("Should pair the state glyph with the literal status word", () => {
    const { container } = render(inTable(<TasksListRow {...baseRow} />));
    const status = container.querySelector('[data-slot="tasks-list-row-status"]');
    expect(status).toHaveTextContent("In progress");
    expect(status?.querySelector('[data-slot="state-glyph"]')).toHaveAttribute(
      "data-state",
      "running"
    );
  });

  it("Should render the identifier as bare mono text, preserving its case", () => {
    render(inTable(<TasksListRow {...baseRow} id="TASK-42" />));
    expect(screen.getByText("TASK-42")).toHaveAttribute("data-slot", "tasks-list-row-id");
  });

  it("Should wrap the title in the row link", () => {
    render(inTable(<TasksListRow {...baseRow} />));
    const link = screen.getByRole("link", { name: "Open Summarize feedback" });
    expect(link).toHaveTextContent("Summarize feedback");
  });

  it("Should keep the owner cell outside the link region", () => {
    render(inTable(<TasksListRow {...baseRow} owner={<span data-testid="owner-chip">PL</span>} />));
    const link = screen.getByRole("link", { name: "Open Summarize feedback" });
    expect(link).not.toContainElement(screen.getByTestId("owner-chip"));
  });

  it("Should render an inert row without a link", () => {
    render(inTable(<TasksListRow {...baseRow} link={null} />));
    expect(screen.queryByRole("link")).toBeNull();
    expect(screen.getByText("Summarize feedback")).toBeInTheDocument();
  });
});

// Revealed Loop execution records share this row's geometry and its identity
// contract: plain words lead, machine ids stay in secondary text.
describe("TaskLoopRow", () => {
  const cellLoop: TaskLoopProvenance = {
    role: "cell",
    run_id: "looprun-8f3ab2c1d4e5f607",
    loop_name: "revisao-paralela",
    generation: 1,
    node_id: "revisor-perf",
    item_index: 0,
  };
  const cellTask = buildTask({
    id: "loop.looprun-8f3ab2c1d4e5f607.g1.node.revisor-perf.0",
    identifier: undefined,
    title: "Loop revisao-paralela node revisor-perf",
    status: "in_progress",
  });

  // UT-040
  it("Should render plain identity, the loop glyph and a run link, never the raw task id", () => {
    const { container } = render(inTable(<TaskLoopRow loop={cellLoop} task={cellTask} />));

    const identity = container.querySelector('[data-slot="task-loop-row-identity"]');
    expect(identity).toHaveTextContent("revisao-paralela · round 1 · step revisor-perf");
    expect(identity).not.toHaveTextContent(cellTask.id);
    expect(container.querySelector('[data-slot="task-loop-row-role"]')).toHaveTextContent(
      "Loop step"
    );
    expect(container.querySelector('[data-slot="state-glyph"]')).toHaveAttribute(
      "data-state",
      "running"
    );

    const link = screen.getByRole("link", {
      name: "Open run for revisao-paralela · round 1 · step revisor-perf",
    });
    expect(link).toHaveAttribute("href", "/loop-runs/$runId");
    expect(link).toHaveAttribute(
      "data-params",
      JSON.stringify({ runId: "looprun-8f3ab2c1d4e5f607" })
    );
    // The loop owns its own retries across generations, so the task-level
    // attempt ceiling never appears on a revealed record.
    expect(screen.queryByText(/attempt/i)).toBeNull();
  });

  it("Should lead the parent record with the loop name and the literal word run", () => {
    const { container } = render(
      inTable(
        <TaskLoopRow
          loop={{
            role: "coordinator",
            run_id: "looprun-8f3ab2c1d4e5f607",
            loop_name: "revisao-paralela",
          }}
          task={buildTask({
            id: "loop.looprun-8f3ab2c1d4e5f607.coordinator",
            identifier: undefined,
          })}
        />
      )
    );
    expect(container.querySelector('[data-slot="task-loop-row-identity"]')).toHaveTextContent(
      "revisao-paralela · run"
    );
    expect(container.querySelector('[data-slot="task-loop-row-role"]')).toHaveTextContent(
      "Loop run"
    );
  });

  it("Should disambiguate fan-out workers past the first by item index", () => {
    const { container } = render(
      inTable(
        <TaskLoopRow
          loop={{ ...cellLoop, node_id: "revisores", item_index: 2 }}
          task={buildTask({ id: "loop.run.g1.node.revisores.2", identifier: undefined })}
        />
      )
    );
    expect(container.querySelector('[data-slot="task-loop-row-identity"]')).toHaveTextContent(
      "revisao-paralela · round 1 · step revisores · item 2"
    );
  });

  // UT-042 (list half): a record whose run retention removed keeps its
  // provenance but offers no link to follow.
  it("Should render the run-gone degrade with no link when the loop name is unrecoverable", () => {
    const { container } = render(
      inTable(
        <TaskLoopRow
          loop={{ role: "cell", run_id: "looprun-77aa01b2c3d4e5f6", generation: 2 }}
          task={buildTask({
            id: "loop.looprun-77aa01b2c3d4e5f6.g2.node.saida.0",
            identifier: undefined,
          })}
        />
      )
    );
    expect(container.querySelector('[data-slot="task-loop-row-identity"]')).toHaveTextContent(
      "Loop step"
    );
    expect(screen.getByText("Run no longer available")).toBeInTheDocument();
    expect(screen.queryByRole("link")).toBeNull();
    expect(container.querySelector('[data-slot="task-loop-row-run-id"]')).toHaveTextContent(
      "looprun-77aa01b2c3d4e5f6"
    );
  });
});
