// Suite: Task detail location controller
// Invariant: inspector-owned reads run only while the inspector is open in a live task window, and
// the controller's task PATCH/DELETE verbs (plus the profile DELETE behind clear-setup) no-op at
// the origin when the latched tier cannot execute those local-only routes (BR-1).
// Boundary IN: task-detail location liveness, inspect URL state, and mutation-verb origin gating.
// Boundary OUT: operator query execution, owned by the task hook suites.
import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { latchGatewayTierForTest } from "@/test/gateway-tier";

const hooks = vi.hoisted(() => ({
  liveDataEnabled: true,
  navigate: vi.fn(),
  userOpen: vi.fn(),
  deleteMutateAsync: vi.fn(),
  updateMutateAsync: vi.fn(),
  useTaskSetupRuntime: vi.fn(),
  useTaskOperatorLayer: vi.fn(),
  useTaskDetailPage: vi.fn(),
}));

vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => hooks.navigate,
}));

vi.mock("../../../hooks/use-os-shell", () => ({
  useOsShell: () => ({ coordinator: { userOpen: hooks.userOpen } }),
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
  useDeleteTask: () => ({ isPending: false, mutateAsync: hooks.deleteMutateAsync }),
  useLiveElapsed: () => 0,
  useProfileEditor: () => ({}),
  useTaskDetailPage: hooks.useTaskDetailPage,
  useTaskOperatorLayer: hooks.useTaskOperatorLayer,
  useTaskPauseDialog: () => ({}),
  useTaskSetupRuntime: hooks.useTaskSetupRuntime,
  useUpdateTask: () => ({ isPending: false, mutateAsync: hooks.updateMutateAsync }),
}));

import { useTaskDetailLocation } from "../use-task-detail-location";

let unlatch: () => void;

beforeEach(() => {
  vi.clearAllMocks();
  hooks.liveDataEnabled = true;
  // The controller's PATCH/DELETE verbs gate on the latched tier at the
  // origin: latch `local` so the suite's default shape keeps firing them,
  // and unlatch so each case starts from a clean store.
  unlatch = latchGatewayTierForTest("local");
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

afterEach(() => {
  unlatch();
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

  // Invariant: the controller's task PATCH/DELETE verbs (delete, priority,
  // auto-enqueue, and the execution-profile DELETE behind clear-setup) hit
  // routes registered only on the local surface set (`routes.go`
  // `includeTaskMutations`), so at the origin they no-op on a remote tier —
  // a wiring regression cannot fire a doomed request (BR-1).
  // Owning layer: task-detail location controller.
  // Canonical suite: useTaskDetailLocation hook tests.
  it("Should fire the controller PATCH/DELETE verbs on the local tier", async () => {
    hooks.useTaskDetailPage.mockReturnValue({
      activeRun: null,
      detail: {
        task: { id: "task_001", priority: "low", auto_enqueue_on_ready: false },
      },
      handlePauseTask: vi.fn(),
      isLive: false,
      profile: null,
      runs: [],
      streamErrorMessage: null,
      streamSeedSequence: 0,
      streamState: "idle",
    });
    hooks.useTaskOperatorLayer.mockReturnValue({
      handleSetProfile: vi.fn(),
      handleDeleteProfile: vi.fn().mockResolvedValue(undefined),
    });

    const { result } = renderHook(() => useTaskDetailLocation("task_001", { tab: "overview" }));

    await act(async () => {
      await result.current.handleDeleteTask("task_001");
      await result.current.handlePriorityChange("high");
      await result.current.handleAutoEnqueueChange(true);
      await result.current.handleClearSetup();
    });

    expect(hooks.deleteMutateAsync).toHaveBeenCalledWith({ id: "task_001" });
    expect(hooks.updateMutateAsync).toHaveBeenCalledWith({
      id: "task_001",
      data: { priority: "high" },
    });
    expect(hooks.updateMutateAsync).toHaveBeenCalledWith({
      id: "task_001",
      data: { auto_enqueue_on_ready: true },
    });
    expect(hooks.useTaskOperatorLayer().handleDeleteProfile).toHaveBeenCalledTimes(1);
  });

  it("Should no-op the controller PATCH/DELETE verbs on a remote tier", async () => {
    unlatch();
    unlatch = latchGatewayTierForTest("private");
    hooks.useTaskDetailPage.mockReturnValue({
      activeRun: null,
      detail: {
        task: { id: "task_001", priority: "low", auto_enqueue_on_ready: false },
      },
      handlePauseTask: vi.fn(),
      isLive: false,
      profile: null,
      runs: [],
      streamErrorMessage: null,
      streamSeedSequence: 0,
      streamState: "idle",
    });

    const { result } = renderHook(() => useTaskDetailLocation("task_001", { tab: "overview" }));

    await act(async () => {
      await result.current.handleDeleteTask("task_001");
      await result.current.handlePriorityChange("high");
      await result.current.handleAutoEnqueueChange(true);
      await result.current.handleClearSetup();
    });

    expect(hooks.deleteMutateAsync).not.toHaveBeenCalled();
    expect(hooks.updateMutateAsync).not.toHaveBeenCalled();
    // The delete verb also skips its pre-mutation navigation: the operator
    // stays on the record they were reading.
    expect(hooks.userOpen).not.toHaveBeenCalled();
  });
});
