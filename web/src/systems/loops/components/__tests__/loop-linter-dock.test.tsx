import { fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { LoopLinterDock } from "../editor/loop-linter-dock";
import { buildLintState, emptyLintState } from "../../lib/loop-editor-lint";

function expandDock() {
  const toggle = screen.getByTestId("loop-linter-toggle");
  if (toggle.getAttribute("aria-expanded") === "false") {
    fireEvent.click(toggle);
  }
}

describe("LoopLinterDock", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("Should start collapsed with the count chip visible in the header", () => {
    render(<LoopLinterDock lint={emptyLintState()} validateFailed={false} onReveal={vi.fn()} />);
    expect(screen.getByTestId("loop-linter-toggle")).toHaveAttribute("aria-expanded", "false");
    expect(screen.getByTestId("loop-linter-count")).toHaveTextContent("checking…");
    expect(screen.queryByTestId("loop-linter-issue")).not.toBeInTheDocument();
  });

  it("Should show a neutral pending state before the first daemon verdict", () => {
    render(<LoopLinterDock lint={emptyLintState()} validateFailed={false} onReveal={vi.fn()} />);
    expandDock();
    expect(screen.getByTestId("loop-linter-count")).toHaveTextContent("checking…");
    expect(screen.queryByTestId("loop-linter-issue")).not.toBeInTheDocument();
  });

  it("Should show an unavailable state (not a perpetual spinner) when validate fails first", () => {
    render(<LoopLinterDock lint={emptyLintState()} validateFailed onReveal={vi.fn()} />);
    expandDock();
    expect(screen.getByTestId("loop-linter-unavailable")).toHaveTextContent(
      /couldn't reach the shared linter/i
    );
    expect(screen.getByTestId("loop-linter-count")).toHaveTextContent("unavailable");
  });

  it("WT-007: Should render a warning without a blocking 422 gate (truthful-UI)", () => {
    const lint = buildLintState({
      valid: true,
      errors: [
        {
          node_id: "await_deploy_ack",
          code: "wait_expiry_without_path",
          message: "wait expiry has neither escalate effects nor route",
          severity: "warning",
        },
      ],
    });
    render(<LoopLinterDock lint={lint} validateFailed={false} onReveal={vi.fn()} />);
    expandDock();
    // A warning-only verdict shows ONLY the warning chip — no error counter is invented, and
    // nothing claims a 422 gate the daemon did not return.
    expect(screen.getByTestId("loop-linter-warning-count")).toHaveTextContent("1 warning");
    expect(screen.queryByTestId("loop-linter-error-count")).not.toBeInTheDocument();
    const row = screen.getByTestId("loop-linter-issue");
    expect(row).toHaveAttribute("data-severity", "warning");
    expect(within(row).getByText(/does not block Publish/i)).toBeInTheDocument();
    expect(within(row).queryByText(/returns 422/i)).not.toBeInTheDocument();
  });

  it("WT-007: Should render a blocking error with the 422 gate copy + reveal", () => {
    const lint = buildLintState({
      valid: false,
      errors: [
        {
          node_id: "implement",
          code: "fan_out_ceiling_exceeded",
          message: "too wide",
          severity: "error",
        },
      ],
    });
    const onReveal = vi.fn();
    render(<LoopLinterDock lint={lint} validateFailed={false} onReveal={onReveal} />);
    expandDock();
    expect(screen.getByTestId("loop-linter-error-count")).toHaveTextContent("1 error");
    expect(screen.queryByTestId("loop-linter-warning-count")).not.toBeInTheDocument();
    const row = screen.getByTestId("loop-linter-issue");
    expect(row).toHaveAttribute("data-severity", "error");
    expect(within(row).getByText(/returns 422/i)).toBeInTheDocument();
  });

  it("WT-007: Should split the counters when a verdict carries both severities", () => {
    const lint = buildLintState({
      valid: false,
      errors: [
        { node_id: "implement", code: "fan_out_ceiling_exceeded", message: "", severity: "error" },
        {
          node_id: "await_deploy_ack",
          code: "wait_expiry_without_path",
          message: "",
          severity: "warning",
        },
      ],
    });
    render(<LoopLinterDock lint={lint} validateFailed={false} onReveal={vi.fn()} />);
    expect(screen.getByTestId("loop-linter-error-count")).toHaveTextContent("1 error");
    expect(screen.getByTestId("loop-linter-warning-count")).toHaveTextContent("1 warning");
  });

  it("WT-007: Should render no counter at all for a clean verdict", () => {
    const lint = buildLintState({ valid: true, errors: [] });
    render(<LoopLinterDock lint={lint} validateFailed={false} onReveal={vi.fn()} />);
    // A zero issue count renders NO counter — not "0 issues" (US-028 AC-4).
    expect(screen.queryByTestId("loop-linter-count")).not.toBeInTheDocument();
    expect(screen.queryByTestId("loop-linter-error-count")).not.toBeInTheDocument();
    expect(screen.queryByTestId("loop-linter-warning-count")).not.toBeInTheDocument();
    expandDock();
    expect(screen.getByText(/All invariants pass/i)).toBeInTheDocument();
  });

  it("Should render duplicate daemon issues without React key collisions", () => {
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => undefined);
    const issue = {
      node_id: "implement",
      code: "fan_out_ceiling_exceeded",
      message: "too wide",
      severity: "error" as const,
    };
    const lint = buildLintState({ valid: false, errors: [issue, issue] });

    render(<LoopLinterDock lint={lint} validateFailed={false} onReveal={vi.fn()} />);
    expandDock();

    expect(screen.getAllByTestId("loop-linter-issue")).toHaveLength(2);
    expect(consoleError).not.toHaveBeenCalled();
  });
});
