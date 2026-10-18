import { fireEvent, render, screen } from "@testing-library/react";
import type { AnchorHTMLAttributes } from "react";
import { describe, expect, it, vi } from "vitest";

vi.mock("@tanstack/react-router", () => ({
  Link: ({
    to,
    search,
    children,
    ...props
  }: AnchorHTMLAttributes<HTMLAnchorElement> & { to: string; search?: Record<string, string> }) => (
    <a href={search ? `${to}?${new URLSearchParams(search).toString()}` : to} {...props}>
      {children}
    </a>
  ),
}));

import { LoopStartBindingsPanel } from "../detail/loop-start-bindings-panel";
import type { LoopBindingRow } from "../../lib/loop-bindings";

const DECLARED = ["manual", "cli", "http", "uds", "native_tool", "schedule"];
const BINDINGS: LoopBindingRow[] = [
  {
    id: "job_nightly",
    name: "nightly",
    kind: "schedule",
    enabled: false,
    meta: "Cron 0 3 * * * · next in 6h",
  },
];

/** The rail card folds by default; open it the way an operator would. */
function openStart() {
  fireEvent.click(screen.getByRole("button", { name: /Automations/ }));
}

describe("LoopStartBindingsPanel", () => {
  it("Should render attached automation rows without the raw declared start kinds", () => {
    render(
      <LoopStartBindingsPanel
        loopName="implement-tasks"
        declaredKinds={DECLARED}
        bindings={BINDINGS}
      />
    );
    openStart();
    expect(screen.queryByTestId("loop-declared-kind")).not.toBeInTheDocument();
    expect(screen.queryByText(DECLARED.join(" · "))).not.toBeInTheDocument();
    const row = screen.getByTestId("loop-binding-row");
    expect(row).toHaveAttribute("data-enabled", "false");
    expect(row).toHaveTextContent("nightly");
    expect(row).toHaveTextContent("Cron 0 3 * * *");
  });

  it("Should show the empty state when no automations are attached", () => {
    render(
      <LoopStartBindingsPanel loopName="implement-tasks" declaredKinds={DECLARED} bindings={[]} />
    );
    openStart();
    expect(screen.getByTestId("loop-bindings-empty")).toHaveTextContent(
      "Runs only when you start it."
    );
    expect(screen.queryByTestId("loop-binding-row")).not.toBeInTheDocument();
  });

  it("Should link the automation count to the Loop's Automations list [UT-114]", () => {
    render(
      <LoopStartBindingsPanel
        loopName="implement-tasks"
        declaredKinds={DECLARED}
        bindings={[...BINDINGS, { ...BINDINGS[0]!, id: "trg_review", kind: "trigger" }]}
      />
    );
    expect(screen.getByRole("button", { name: /Automations/ })).toHaveTextContent("2 automations");
    openStart();
    const link = screen.getByTestId("loop-bindings-open-automations");
    expect(link).toHaveTextContent("2 automations");
    expect(link).toHaveAttribute("href", "/automations?loop=implement-tasks");
  });

  it("Should read Manual only with no link when nothing starts the Loop", () => {
    render(
      <LoopStartBindingsPanel loopName="implement-tasks" declaredKinds={["manual"]} bindings={[]} />
    );
    expect(screen.getByRole("button", { name: /Automations/ })).toHaveTextContent("Manual only");
    openStart();
    expect(screen.queryByTestId("loop-bindings-open-automations")).not.toBeInTheDocument();
    expect(screen.getByText("This Loop can only be started by hand.")).toBeInTheDocument();
  });

  it("Should disclose partial binding totals and page each automation kind explicitly", () => {
    const loadMoreJobs = vi.fn();
    const { rerender } = render(
      <LoopStartBindingsPanel
        loopName="implement-tasks"
        bindings={BINDINGS}
        declaredKinds={DECLARED}
        jobs={{
          error: null,
          hasMore: true,
          isFetchingMore: false,
          loadMore: loadMoreJobs,
          loaded: 50,
          total: 72,
        }}
        triggers={{
          error: null,
          hasMore: false,
          isFetchingMore: false,
          loadMore: vi.fn(),
          loaded: 18,
          total: 18,
        }}
      />
    );
    openStart();

    expect(screen.getByTestId("loop-bindings-progress")).toHaveTextContent("Showing 68 of 90");
    fireEvent.click(screen.getByRole("button", { name: "Load more schedules" }));
    expect(loadMoreJobs).toHaveBeenCalledOnce();

    rerender(
      <LoopStartBindingsPanel
        loopName="implement-tasks"
        bindings={BINDINGS}
        declaredKinds={DECLARED}
        jobs={{
          error: new Error("Could not load the next schedules page"),
          hasMore: true,
          isFetchingMore: true,
          loadMore: loadMoreJobs,
          loaded: 50,
          total: 72,
        }}
        triggers={{
          error: null,
          hasMore: false,
          isFetchingMore: false,
          loadMore: vi.fn(),
          loaded: 18,
          total: 18,
        }}
      />
    );
    expect(screen.getByTestId("loop-binding-row")).toBeInTheDocument();
    expect(screen.getByRole("alert")).toHaveTextContent("Could not load");
    expect(screen.getByTestId("loop-bindings-load-more-jobs")).toBeDisabled();
    expect(screen.getByTestId("loop-bindings-load-more-jobs")).toHaveAttribute("aria-busy", "true");
  });
});
