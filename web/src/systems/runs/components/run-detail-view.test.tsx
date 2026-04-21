import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactElement } from "react";
import { describe, expect, it, vi } from "vitest";

import { RunDetailView } from "./run-detail-view";
import type { RunSnapshot } from "../types";
import type { RunStreamStatus } from "../hooks/use-run-stream";

function buildSnapshot(overrides: Partial<RunSnapshot> = {}): RunSnapshot {
  return {
    run: {
      run_id: "run-1",
      mode: "task",
      presentation_mode: "text",
      workspace_id: "ws-1",
      workflow_slug: "alpha",
      status: "running",
      started_at: "2026-01-01T00:00:00Z",
    },
    jobs: [
      {
        index: 0,
        job_id: "job-1",
        status: "running",
        updated_at: "2026-01-01T00:01:00Z",
      },
    ],
    transcript: [
      {
        content: "hello",
        role: "assistant",
        sequence: 1,
        stream: "stdout",
        timestamp: "2026-01-01T00:01:30Z",
      },
    ],
    usage: { input_tokens: 12, output_tokens: 7, total_tokens: 19 },
    ...overrides,
  };
}

interface RenderProps {
  snapshot?: RunSnapshot;
  streamStatus?: RunStreamStatus;
  streamEventCount?: number;
  lastHeartbeatAt?: number | null;
  overflowReason?: string | null;
  streamError?: string | null;
  cancelDisabled?: boolean;
  isCancelling?: boolean;
  cancelError?: string | null;
  cancelSuccess?: string | null;
  onReconnectStream?: () => void;
  onCancelRun?: () => void;
  isRefreshingSnapshot?: boolean;
}

async function renderRunDetail(props: RenderProps = {}) {
  const rootRoute = createRootRoute();
  const indexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: function IndexRouteComponent(): ReactElement {
      return (
        <RunDetailView
          cancelDisabled={props.cancelDisabled ?? false}
          cancelError={props.cancelError ?? null}
          cancelSuccess={props.cancelSuccess ?? null}
          isCancelling={props.isCancelling ?? false}
          isRefreshingSnapshot={props.isRefreshingSnapshot ?? false}
          lastHeartbeatAt={props.lastHeartbeatAt ?? null}
          onCancelRun={props.onCancelRun ?? (() => {})}
          onReconnectStream={props.onReconnectStream ?? (() => {})}
          overflowReason={props.overflowReason ?? null}
          snapshot={props.snapshot ?? buildSnapshot()}
          streamError={props.streamError ?? null}
          streamEventCount={props.streamEventCount ?? 0}
          streamStatus={props.streamStatus ?? "connecting"}
        />
      );
    },
  });
  const workflowsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/workflows",
    component: function WorkflowsRouteComponent(): ReactElement {
      return <div data-testid="workflows-stub" />;
    },
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([indexRoute, workflowsRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    defaultPreload: false,
  });
  await router.load();
  await act(async () => {
    render(<RouterProvider router={router} />);
    await Promise.resolve();
  });
}

describe("RunDetailView", () => {
  it("Should render status badge, jobs, and transcript", async () => {
    await renderRunDetail();
    expect(screen.getByTestId("run-detail-view")).toBeInTheDocument();
    expect(screen.getByTestId("run-detail-status")).toHaveTextContent("running");
    expect(screen.getByTestId("run-detail-job-row-job-1")).toBeInTheDocument();
    expect(screen.getByTestId("run-detail-transcript-1")).toHaveTextContent("hello");
  });

  it("Should show the empty transcript state", async () => {
    await renderRunDetail({ snapshot: buildSnapshot({ transcript: [] }) });
    expect(screen.getByTestId("run-detail-transcript-empty")).toBeInTheDocument();
  });

  it("Should show the stream status badge", async () => {
    await renderRunDetail({ streamStatus: "open", streamEventCount: 3 });
    expect(screen.getByTestId("run-detail-stream-status")).toHaveTextContent("stream open");
    expect(screen.getByTestId("run-detail-stream-events")).toHaveTextContent("3");
  });

  it("Should show the overflow notice when the stream overflowed", async () => {
    await renderRunDetail({
      streamStatus: "overflowed",
      overflowReason: "replay boundary exceeded",
    });
    expect(screen.getByTestId("run-detail-stream-overflow")).toHaveTextContent(
      "replay boundary exceeded"
    );
  });

  it("Should show a stream error when provided", async () => {
    await renderRunDetail({ streamError: "disconnected" });
    expect(screen.getByTestId("run-detail-stream-error")).toHaveTextContent("disconnected");
  });

  it("Should call the reconnect and cancel handlers", async () => {
    const onReconnectStream = vi.fn();
    const onCancelRun = vi.fn();
    await renderRunDetail({ onReconnectStream, onCancelRun });
    await userEvent.click(screen.getByTestId("run-detail-reconnect"));
    await userEvent.click(screen.getByTestId("run-detail-cancel"));
    expect(onReconnectStream).toHaveBeenCalledTimes(1);
    expect(onCancelRun).toHaveBeenCalledTimes(1);
  });

  it("Should disable the cancel action when requested", async () => {
    await renderRunDetail({ cancelDisabled: true });
    expect(screen.getByTestId("run-detail-cancel")).toBeDisabled();
  });

  it("Should render the cancel success banner", async () => {
    await renderRunDetail({ cancelSuccess: "Cancellation requested" });
    expect(screen.getByTestId("run-detail-cancel-success")).toHaveTextContent(
      "Cancellation requested"
    );
  });

  it("Should render the cancel error banner", async () => {
    await renderRunDetail({ cancelError: "could not cancel" });
    expect(screen.getByTestId("run-detail-cancel-error")).toHaveTextContent("could not cancel");
  });

  it("Should show a snapshot refresh indicator when provided", async () => {
    await renderRunDetail({ isRefreshingSnapshot: true });
    expect(screen.getByTestId("run-detail-jobs-refreshing")).toBeInTheDocument();
  });
});
