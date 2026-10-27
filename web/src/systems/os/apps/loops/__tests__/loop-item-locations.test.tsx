// Suite: Loop item routes waiting on their reads
// Invariant: a Loop route whose reads have not answered is waiting, not missing. "No data yet"
// (a query's `isPending`, which includes a read a suspended window has not started) holds the
// surface's own skeleton; only a failure is the "couldn't open this Loop" state.
// Boundary IN: the detail, configure and run-form locations' gating over their view-models.
// Boundary OUT: the skeletons' anatomy and the view-models' queries, owned by their own suites.

import { createElement, type ReactNode } from "react";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const state = vi.hoisted(() => ({
  detail: null as Record<string, unknown> | null,
  configure: null as Record<string, unknown> | null,
  form: null as Record<string, unknown> | null,
}));

vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => vi.fn(),
  Link: ({ to, children, ...props }: { to: string; children?: ReactNode }) =>
    createElement("a", { href: to, ...props }, children),
}));

vi.mock("../use-loop-detail", () => ({ useLoopDetail: () => state.detail }));
vi.mock("../hooks/use-loop-configure-location", () => ({
  useLoopConfigureLocation: () => state.configure,
}));
vi.mock("../use-loop-run-form-page", () => ({ useLoopRunFormPage: () => state.form }));

const { LoopDetailLocation } = await import("../loop-detail-location");
const { LoopConfigureLocation } = await import("../loop-configure-location");
const { LoopRunFormLocation } = await import("../loop-run-form-location");

const PENDING = { data: undefined, error: null, isPending: true, isLoadingError: false };
const FAILED = {
  data: undefined,
  error: new Error("Loop implement-tasks not found."),
  isPending: false,
  isLoadingError: true,
};

function detailState(loopQuery: object, configQuery: object) {
  return {
    scope: { kind: "project", workspaceId: "ws_a" },
    workspaceId: "ws_a",
    loopQuery,
    configQuery: { effectiveConfig: undefined, ...configQuery },
    catalogEntry: null,
    runsQuery: { data: undefined, isPending: true },
    bindings: { rows: [], isLoading: false, jobs: undefined, triggers: undefined },
    deleteLoop: { isPending: false, error: null },
    readGraph: vi.fn(),
    handlers: {},
  };
}

describe("Loop item routes waiting on their reads", () => {
  beforeEach(() => {
    state.detail = null;
    state.configure = null;
    state.form = null;
  });

  it("Should hold the detail page's skeleton while either read is pending, and not call it missing", () => {
    state.detail = detailState(PENDING, { ...PENDING });
    const { rerender } = render(<LoopDetailLocation name="implement-tasks" />);
    expect(screen.getByTestId("loop-detail-loading")).toBeInTheDocument();
    expect(screen.queryByTestId("loop-detail-not-found")).not.toBeInTheDocument();

    // One read answered is not enough to render the page.
    state.detail = detailState({ ...PENDING, isPending: false, data: {} }, PENDING);
    rerender(<LoopDetailLocation name="implement-tasks" />);
    expect(screen.getByTestId("loop-detail-loading")).toBeInTheDocument();
  });

  it("Should keep a failed detail read the 'couldn't open' state, never a skeleton", () => {
    state.detail = detailState(FAILED, { ...PENDING, isPending: false, data: {} });

    render(<LoopDetailLocation name="implement-tasks" />);

    expect(screen.getByTestId("loop-detail-not-found")).toHaveTextContent(
      "Loop implement-tasks not found."
    );
    expect(screen.queryByTestId("loop-detail-loading")).not.toBeInTheDocument();
  });

  it("Should open the configure dialog over placeholders while its reads are pending", () => {
    state.configure = {
      close: vi.fn(),
      configQuery: PENDING,
      loopQuery: PENDING,
      openEditor: vi.fn(),
      scope: { kind: "project", workspaceId: "ws_a" },
      workspaceId: "ws_a",
    };

    render(<LoopConfigureLocation name="implement-tasks" />);

    expect(screen.getByTestId("loop-configure-dialog")).toBeInTheDocument();
    expect(screen.getByTestId("loop-configure-loading")).toBeInTheDocument();
    expect(screen.queryByTestId("loop-configure-error")).not.toBeInTheDocument();
  });

  // The live-run check decides whether a notice leads the form, so the form waits for it as well:
  // a notice arriving after the form would push every field down.
  it("Should hold the run form's skeleton until the Loop, its settings and the live-run check answer", () => {
    const answered = { data: {}, error: null, isPending: false, isLoadingError: false };
    state.form = {
      activeRun: null,
      configQuery: { ...answered, effectiveConfig: {} },
      liveRunPending: true,
      loopQuery: answered,
      openLoop: vi.fn(),
      openLoops: vi.fn(),
      openRun: vi.fn(),
      scope: { kind: "project", workspaceId: "ws_a" },
      workspaceId: "ws_a",
    };

    render(<LoopRunFormLocation name="implement-tasks" />);

    expect(screen.getByTestId("loop-run-form-loading")).toBeInTheDocument();
    expect(screen.queryByTestId("loop-run-form")).not.toBeInTheDocument();
    expect(screen.queryByTestId("loop-run-form-error")).not.toBeInTheDocument();
  });
});
