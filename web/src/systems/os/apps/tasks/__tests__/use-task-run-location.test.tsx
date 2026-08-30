// Suite: Task run location controller
// Invariant: an inactive retained run window disables every route-owned task read.
// Boundary IN: run-location liveness wiring for the page, timeline, sibling runs, and stream.
// Boundary OUT: task query execution, owned by the task hook suites.
import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const hooks = vi.hoisted(() => ({
  liveDataEnabled: true,
  navigate: vi.fn(),
  useTaskRunPage: vi.fn(),
  useTaskRuns: vi.fn(),
  useTaskStream: vi.fn(),
  useTaskTimeline: vi.fn(),
}));

vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => hooks.navigate,
}));

vi.mock("../../../hooks/use-window-live-data-enabled", () => ({
  useCurrentWindowLiveDataEnabled: () => hooks.liveDataEnabled,
}));

vi.mock("@/systems/tasks", () => ({
  computeElapsed: vi.fn(),
  taskRunCanRecover: vi.fn(() => false),
  useForceFailDialog: vi.fn(() => ({})),
  useLiveElapsed: vi.fn(),
  useTaskRunPage: hooks.useTaskRunPage,
  useTaskRuns: hooks.useTaskRuns,
  useTaskStream: hooks.useTaskStream,
  useTaskTimeline: hooks.useTaskTimeline,
}));

import { useTaskRunLocation } from "../use-task-run-location";

beforeEach(() => {
  vi.clearAllMocks();
  hooks.liveDataEnabled = true;
  hooks.useTaskRunPage.mockReturnValue({
    handleForceFailRun: vi.fn(),
    isLive: true,
    run: { run: { id: "run_001", status: "running", task_id: "task_001" } },
    session: { agent_name: "builder", session_id: "sess_001" },
    task: { task: { latest_event_seq: 12, max_attempts: 1 } },
  });
  hooks.useTaskRuns.mockReturnValue({ data: [], error: null, isLoading: false });
  hooks.useTaskTimeline.mockReturnValue({ data: [], error: null, isLoading: false });
});

describe("useTaskRunLocation", () => {
  it("Should release every task read when the retained run window becomes inactive", () => {
    const { rerender } = renderHook(() => useTaskRunLocation("task_001", "run_001"));

    expect(hooks.useTaskRunPage).toHaveBeenLastCalledWith("task_001", "run_001", {
      liveDataEnabled: true,
    });
    expect(hooks.useTaskTimeline).toHaveBeenLastCalledWith(
      "task_001",
      {},
      expect.objectContaining({ enabled: true })
    );
    expect(hooks.useTaskRuns).toHaveBeenLastCalledWith(
      "task_001",
      {},
      expect.objectContaining({ enabled: true })
    );
    expect(hooks.useTaskStream).toHaveBeenLastCalledWith(
      "task_001",
      expect.objectContaining({ enabled: true })
    );

    hooks.liveDataEnabled = false;
    rerender();

    expect(hooks.useTaskRunPage).toHaveBeenLastCalledWith("task_001", "run_001", {
      liveDataEnabled: false,
    });
    expect(hooks.useTaskTimeline).toHaveBeenLastCalledWith(
      "task_001",
      {},
      expect.objectContaining({ enabled: false })
    );
    expect(hooks.useTaskRuns).toHaveBeenLastCalledWith(
      "task_001",
      {},
      expect.objectContaining({ enabled: false })
    );
    expect(hooks.useTaskStream).toHaveBeenLastCalledWith(
      "task_001",
      expect.objectContaining({ enabled: false })
    );
  });

  it("Should open the canonical session route when the run owns its agent identity", () => {
    const { result } = renderHook(() => useTaskRunLocation("task_001", "run_001"));

    result.current.openSession("sess_001");

    expect(hooks.navigate).toHaveBeenCalledWith({
      to: "/agents/$name/sessions/$id",
      params: { name: "builder", id: "sess_001" },
    });
  });

  it("Should retain the session permalink fallback when agent identity is unavailable", () => {
    hooks.useTaskRunPage.mockReturnValue({
      handleForceFailRun: vi.fn(),
      isLive: true,
      run: { run: { id: "run_001", status: "running", task_id: "task_001" } },
      session: { session_id: "sess_001" },
      task: { task: { latest_event_seq: 12, max_attempts: 1 } },
    });
    const { result } = renderHook(() => useTaskRunLocation("task_001", "run_001"));

    result.current.openSession("sess_001");

    expect(hooks.navigate).toHaveBeenCalledWith({
      to: "/session/$id",
      params: { id: "sess_001" },
    });
  });
});
