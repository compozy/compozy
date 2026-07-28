import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

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

describe("LoopStartBindingsPanel", () => {
  it("Should render the declared start kinds and attached automation rows", () => {
    render(<LoopStartBindingsPanel declaredKinds={DECLARED} bindings={BINDINGS} />);
    const kinds = screen.getAllByTestId("loop-declared-kind").map(node => node.textContent);
    expect(kinds).toEqual(DECLARED);
    const row = screen.getByTestId("loop-binding-row");
    expect(row).toHaveAttribute("data-enabled", "false");
    expect(row).toHaveTextContent("nightly");
    expect(row).toHaveTextContent("Cron 0 3 * * *");
  });

  it("Should show the empty state when no automations are attached", () => {
    render(<LoopStartBindingsPanel declaredKinds={DECLARED} bindings={[]} />);
    expect(screen.getByTestId("loop-bindings-empty")).toBeInTheDocument();
    expect(screen.queryByTestId("loop-binding-row")).not.toBeInTheDocument();
  });

  it("Should gate the Add CTAs to the kinds the allowlist permits", () => {
    const onAddSchedule = vi.fn();
    render(
      <LoopStartBindingsPanel
        declaredKinds={DECLARED}
        bindings={[]}
        onAddSchedule={onAddSchedule}
      />
    );
    // software-delivery declares schedule but not trigger/webhook.
    expect(screen.getByTestId("loop-add-schedule")).toBeInTheDocument();
    expect(screen.queryByTestId("loop-add-trigger")).not.toBeInTheDocument();
    fireEvent.click(screen.getByTestId("loop-add-schedule"));
    expect(onAddSchedule).toHaveBeenCalledTimes(1);
  });

  it("Should offer Add trigger when the loop declares a trigger or webhook start", () => {
    render(<LoopStartBindingsPanel declaredKinds={["manual", "webhook"]} bindings={[]} />);
    expect(screen.getByTestId("loop-add-trigger")).toBeInTheDocument();
    expect(screen.queryByTestId("loop-add-schedule")).not.toBeInTheDocument();
  });

  it("Should disclose partial binding totals and page each automation kind explicitly", () => {
    const loadMoreJobs = vi.fn();
    const { rerender } = render(
      <LoopStartBindingsPanel
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

    expect(screen.getByTestId("loop-bindings-progress")).toHaveTextContent(
      "68 of 90 attached loaded"
    );
    fireEvent.click(screen.getByRole("button", { name: "Load more schedules" }));
    expect(loadMoreJobs).toHaveBeenCalledOnce();

    rerender(
      <LoopStartBindingsPanel
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
