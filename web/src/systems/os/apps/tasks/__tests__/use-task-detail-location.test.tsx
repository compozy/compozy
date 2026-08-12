// Suite: Task detail location controller
// Invariant: inspector-owned reads run only while the inspector is open in a live task window.
// Boundary IN: task-detail location liveness and inspect URL state.
// Boundary OUT: operator query execution, owned by the task hook suites.
import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const hooks = vi.hoisted(() => ({
  liveDataEnabled: true,
  navigate: vi.fn(),
  useTaskSetupRuntime: vi.fn(),
  useTaskOperatorLayer: vi.fn(),
  useTaskDetailPage: vi.fn(),
}));

vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => hooks.navigate,
}));

vi.mock("../../../hooks/use-os-shell", () => ({
  useOsShell: () => ({ coordinator: { userOpen: vi.fn() } }),
}));

vi.mock("../../../hooks/use-window-live-data-enabled", () => ({
  useCurrentWindowLiveDataEnabled: () => hooks.liveDataEnabled,
}));

vi.mock("sonner", () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}));

vi.mock("@/systems/tasks", () => ({
  TASK_RESULT_ANCHOR_ID: "task-result",
  computeElapsed: vi.fn(),
  resolveTaskCommandState: vi.fn(),
  useDeleteTask: () => ({ isPending: false, mutateAsync: vi.fn() }),
  useLiveElapsed: () => 0,
  useProfileEditor: () => ({}),
  useTaskDetailPage: hooks.useTaskDetailPage,
  useTaskOperatorLayer: hooks.useTaskOperatorLayer,
  useTaskPauseDialog: () => ({}),
  useTaskSetupRuntime: hooks.useTaskSetupRuntime,
  useUpdateTask: () => ({ isPending: false, mutateAsync: vi.fn() }),
}));

import { useTaskDetailLocation } from "../use-task-detail-location";

beforeEach(() => {
  vi.clearAllMocks();
  hooks.liveDataEnabled = true;
  hooks.useTaskDetailPage.mockReturnValue({
    activeRun: null,
    detail: null,
    handlePauseTask: vi.fn(),
    isLive: false,
    profile: null,
    runs: [],
    streamErrorMessage: null,
    streamSeedSequence: 0,
    streamState: "idle",
  });
  hooks.useTaskOperatorLayer.mockReturnValue({});
  hooks.useTaskSetupRuntime.mockReturnValue({});
});

describe("useTaskDetailLocation", () => {
  it("Should gate inspector queries on both inspector visibility and window liveness", () => {
    const { rerender } = renderHook(
      ({ inspectOpen }: { inspectOpen: boolean }) =>
        useTaskDetailLocation("task_001", {
          ...(inspectOpen ? { inspect: "diagnostics" } : {}),
          tab: "overview",
        }),
      { initialProps: { inspectOpen: false } }
    );

    expect(hooks.useTaskOperatorLayer).toHaveBeenLastCalledWith(
      "task_001",
      expect.objectContaining({ enabled: false })
    );

    rerender({ inspectOpen: true });
    expect(hooks.useTaskOperatorLayer).toHaveBeenLastCalledWith(
      "task_001",
      expect.objectContaining({ enabled: true })
    );

    hooks.liveDataEnabled = false;
    rerender({ inspectOpen: true });
    expect(hooks.useTaskOperatorLayer).toHaveBeenLastCalledWith(
      "task_001",
      expect.objectContaining({ enabled: false })
    );
  });

  it("Should gate setup runtime reads on task availability and window liveness", () => {
    hooks.useTaskDetailPage.mockReturnValue({
      activeRun: null,
      detail: { task: { workspace_id: "ws_northstar" } },
      handlePauseTask: vi.fn(),
      isLive: false,
      profile: null,
      runs: [],
      streamErrorMessage: null,
      streamSeedSequence: 0,
      streamState: "idle",
    });

    const view = renderHook(() => useTaskDetailLocation("task_001", { tab: "overview" }));

    expect(hooks.useTaskSetupRuntime).toHaveBeenLastCalledWith("ws_northstar", {
      enabled: true,
    });

    hooks.liveDataEnabled = false;
    view.rerender();

    expect(hooks.useTaskSetupRuntime).toHaveBeenLastCalledWith("ws_northstar", {
      enabled: false,
    });
  });
});
