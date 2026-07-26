import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

vi.mock("@agh/ui", async importOriginal => {
  const actual = await importOriginal<typeof import("@agh/ui")>();
  return {
    ...actual,
    CodeBlock: ({ code }: { code: ReactNode }) => <pre>{code}</pre>,
  };
});

import { buildDetailFixture, buildTaskInspectFixture } from "../../mocks/fixtures";
import type { TaskStreamState } from "../../lib/task-stream-state";
import { TaskInspectDrawer } from "../task-inspect-drawer";

const bridges = {
  onCreate: vi.fn(),
  onDelete: vi.fn(),
  subscriptions: [],
};

function renderDrawer(state: TaskStreamState, errorMessage: string | null = null) {
  return render(
    <TaskInspectDrawer
      activeTab="stream"
      bridges={bridges}
      detail={buildDetailFixture()}
      inspect={null}
      onOpenChange={vi.fn()}
      onTabChange={vi.fn()}
      open
      stream={{
        errorMessage,
        latestEventSeq: 18,
        seedSequence: 14,
        state,
      }}
    />
  );
}

describe("TaskInspectDrawer", () => {
  it.each([
    ["disabled", "Disabled"],
    ["idle", "Idle"],
    ["receiving", "Receiving"],
    ["error", "Error"],
  ] as const)("Should render %s as an explicit stream state", (state, label) => {
    renderDrawer(state, state === "error" ? "Stream unavailable" : null);

    expect(screen.getByTestId("tasks-inspect-stream")).toHaveTextContent(label);
    expect(screen.getByTestId("tasks-inspect-stream")).toHaveTextContent("18");
    expect(screen.getByTestId("tasks-inspect-stream")).toHaveTextContent("14");
    if (state === "error") {
      expect(screen.getByText("Stream unavailable")).toBeInTheDocument();
    }
  });

  it("Should render an inspect failure instead of the empty-snapshot message", () => {
    render(
      <TaskInspectDrawer
        activeTab="diagnostics"
        bridges={bridges}
        detail={buildDetailFixture()}
        inspect={null}
        inspectErrorMessage="Inspect snapshot failed"
        onOpenChange={vi.fn()}
        onTabChange={vi.fn()}
        open
        stream={{
          errorMessage: null,
          latestEventSeq: null,
          seedSequence: 0,
          state: "idle",
        }}
      />
    );

    expect(screen.getByRole("alert")).toHaveTextContent("Inspect snapshot failed");
    expect(screen.queryByTestId("tasks-inspect-diagnostics-empty")).toBeNull();
    expect(screen.queryByText("No inspect snapshot is available for this task state.")).toBeNull();
  });

  it("Should keep a refresh failure visible beside a stale inspect snapshot", () => {
    render(
      <TaskInspectDrawer
        activeTab="diagnostics"
        bridges={bridges}
        detail={buildDetailFixture()}
        inspect={buildTaskInspectFixture()}
        inspectErrorMessage="Inspect refresh failed"
        onOpenChange={vi.fn()}
        onTabChange={vi.fn()}
        open
        stream={{
          errorMessage: null,
          latestEventSeq: null,
          seedSequence: 0,
          state: "idle",
        }}
      />
    );

    expect(screen.getByRole("alert")).toHaveTextContent("Inspect refresh failed");
    expect(screen.getByTestId("tasks-inspect-diagnostics")).toBeInTheDocument();
  });
});
