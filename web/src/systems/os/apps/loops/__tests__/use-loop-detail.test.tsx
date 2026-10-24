import { act, renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  createMutateAsync: vi.fn(),
  deleteMutateAsync: vi.fn(),
  deleteReset: vi.fn(),
  navigate: vi.fn(),
  toastError: vi.fn(),
  toastSuccess: vi.fn(),
  loopSource: "marketplace" as "marketplace" | "workspace",
  useAutomationJobs: vi.fn(),
  useAutomationTriggers: vi.fn(),
}));

vi.mock("@tanstack/react-router", () => ({
  useChildMatches: () => [],
  useNavigate: () => mocks.navigate,
  useRouter: () => ({ history: { back: vi.fn(), canGoBack: () => false } }),
}));

vi.mock("sonner", () => ({
  toast: { error: mocks.toastError, success: mocks.toastSuccess },
}));

// Outside a window frame the window's data scope is the menubar destination.
vi.mock("@/systems/workspace/hooks/use-active-workspace", () => ({
  useActiveWorkspace: () => ({ dataScope: { kind: "project", workspaceId: "ws_default" } }),
  useDataScope: () => ({ kind: "project", workspaceId: "ws_default" }),
}));

vi.mock("@/systems/automation", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/automation")>()),
  useAutomationJobs: mocks.useAutomationJobs,
  useAutomationTimeZone: () => undefined,
  useAutomationTriggers: mocks.useAutomationTriggers,
}));

vi.mock("@/systems/loops/adapters/loops-api", () => {
  class LoopsApiError extends Error {
    status: number;

    constructor(message: string, status: number) {
      super(message);
      this.status = status;
    }
  }

  return { LoopsApiError };
});

vi.mock("@/systems/loops/lib/loop-graph", () => ({
  readLoopGraph: vi.fn(),
}));

vi.mock("@/systems/loops/hooks/use-loop-actions", () => ({
  useCreateLoop: () => ({ isPending: false, mutateAsync: mocks.createMutateAsync }),
  useDeleteLoop: () => ({
    error: null,
    isPending: false,
    mutateAsync: mocks.deleteMutateAsync,
    reset: mocks.deleteReset,
  }),
}));

vi.mock("@/systems/loops/hooks/use-loops", () => ({
  useLoop: () => ({ data: { source: mocks.loopSource } }),
  useLoopConfig: () => ({ data: null }),
  useLoopRuns: () => ({ data: [] }),
  useLoops: () => ({ loops: [] }),
}));

const { useLoopDetail } = await import("@/systems/os/apps/loops/use-loop-detail");
const { WindowDataBindingContext } = await import("@/systems/workspace");
const { LoopsApiError } = await import("@/systems/loops/adapters/loops-api");

describe("useLoopDetail", () => {
  beforeEach(() => {
    mocks.createMutateAsync.mockReset();
    mocks.deleteMutateAsync.mockReset();
    mocks.deleteReset.mockReset();
    mocks.navigate.mockReset();
    mocks.toastError.mockReset();
    mocks.toastSuccess.mockReset();
    mocks.loopSource = "marketplace";
    mocks.useAutomationJobs.mockReset();
    mocks.useAutomationTriggers.mockReset();
    mocks.useAutomationJobs.mockReturnValue({
      error: null,
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
      isPending: false,
      jobs: [],
      total: 0,
    });
    mocks.useAutomationTriggers.mockReturnValue({
      error: null,
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
      isPending: false,
      total: 0,
      triggers: [],
    });
  });

  it("Should surface non-conflict fork failures instead of navigating silently", async () => {
    mocks.createMutateAsync.mockRejectedValue(new Error("daemon unavailable"));
    const { result } = renderHook(() => useLoopDetail("review-and-fix"));

    await act(async () => {
      await result.current.handlers.onOpenEditor();
    });

    expect(mocks.toastError).toHaveBeenCalledWith("daemon unavailable");
    expect(mocks.navigate).not.toHaveBeenCalled();
  });

  it("Should tolerate an existing workspace fork and navigate to the editor", async () => {
    mocks.createMutateAsync.mockRejectedValue(new LoopsApiError("Loop already exists", 409));
    const { result } = renderHook(() => useLoopDetail("review-and-fix"));

    await act(async () => {
      await result.current.handlers.onOpenEditor();
    });

    expect(mocks.toastError).not.toHaveBeenCalled();
    // An unbound window carries no binding into the editor route.
    expect(mocks.navigate).toHaveBeenCalledWith({
      to: "/loops/$name/editor",
      params: { name: "review-and-fix" },
      search: {},
    });
  });

  it("Should read a bound window's project and keep its binding on item navigations", () => {
    const wrapper = ({ children }: { children: ReactNode }) => (
      <WindowDataBindingContext value={{ partition: "global", binding: "ws_b" }}>
        {children}
      </WindowDataBindingContext>
    );
    const { result } = renderHook(() => useLoopDetail("review-and-fix"), { wrapper });

    act(() => result.current.handlers.onRun());
    act(() => result.current.handlers.onConfigure());

    expect(result.current.workspaceId).toBe("ws_b");
    expect(mocks.navigate.mock.calls).toEqual([
      [
        {
          to: "/loops/$name/run",
          params: { name: "review-and-fix" },
          search: { workspace: "ws_b" },
        },
      ],
      [
        {
          to: "/loops/$name/configure",
          params: { name: "review-and-fix" },
          search: { workspace: "ws_b" },
        },
      ],
    ]);
  });

  // Invariant: a Loop's bindings are pending until both automation reads have answered. A read
  // held back until its scope resolves is not fetching, but it has not answered either, so the
  // rail holds the place of the answer instead of saying the Loop starts by hand.
  it.each([
    { name: "the schedules read", jobs: true, triggers: false },
    { name: "the event automations read", jobs: false, triggers: true },
  ])("Should keep the bindings pending while $name has no data, fetching or not", pending => {
    const read = (isPending: boolean) => ({
      error: null,
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
      isLoading: false,
      isPending,
      jobs: [],
      total: 0,
      triggers: [],
    });
    mocks.useAutomationJobs.mockReturnValue(read(pending.jobs));
    mocks.useAutomationTriggers.mockReturnValue(read(pending.triggers));

    const { result } = renderHook(() => useLoopDetail("review-and-fix"));

    expect(result.current.bindings.isPending).toBe(true);
    expect(result.current.bindings.rows).toEqual([]);
  });

  it("Should settle the bindings once both automation reads have answered", () => {
    const { result } = renderHook(() => useLoopDetail("review-and-fix"));

    expect(result.current.bindings.isPending).toBe(false);
  });

  it("Should open the Automations editor locked to this Loop [UT-114]", () => {
    const { result } = renderHook(() => useLoopDetail("software-delivery"));

    act(() => result.current.handlers.onAutomate("schedule"));
    act(() => result.current.handlers.onAutomate("event"));

    expect(mocks.navigate.mock.calls).toEqual([
      [
        {
          to: "/automations",
          search: { create: "loop", start: "schedule", loop: "software-delivery" },
        },
      ],
      [
        {
          to: "/automations",
          search: { create: "loop", start: "event", loop: "software-delivery" },
        },
      ],
    ]);
  });

  it("Should await workspace deletion before replacing detail with the catalog", async () => {
    mocks.loopSource = "workspace";
    mocks.deleteMutateAsync.mockResolvedValue(undefined);
    const { result } = renderHook(() => useLoopDetail("review-and-fix"));

    await act(async () => {
      await result.current.handlers.onDelete();
    });

    expect(mocks.deleteMutateAsync).toHaveBeenCalledWith({
      workspaceId: "ws_default",
      name: "review-and-fix",
    });
    expect(mocks.toastSuccess).toHaveBeenCalledWith("Deleted loop review-and-fix");
    expect(mocks.navigate).toHaveBeenCalledWith({ to: "/loops", replace: true });
  });

  it("Should keep the detail route when workspace deletion fails", async () => {
    mocks.loopSource = "workspace";
    mocks.deleteMutateAsync.mockRejectedValue(new Error("delete failed"));
    const { result } = renderHook(() => useLoopDetail("review-and-fix"));

    await act(async () => {
      await result.current.handlers.onDelete();
    });

    expect(mocks.navigate).not.toHaveBeenCalledWith({ to: "/loops", replace: true });
    expect(mocks.toastSuccess).not.toHaveBeenCalled();
  });

  it("Should query exact loop bindings inside the target workspace with pageable limits", () => {
    const { result } = renderHook(() => useLoopDetail("review-and-fix"));

    expect(mocks.useAutomationJobs).toHaveBeenCalledWith(
      {
        limit: 50,
        loop: "review-and-fix",
        scope: "workspace",
        workspace_id: "ws_default",
      },
      { enabled: true }
    );
    expect(mocks.useAutomationTriggers).toHaveBeenCalledWith(
      {
        limit: 50,
        loop: "review-and-fix",
        scope: "workspace",
        workspace_id: "ws_default",
      },
      { enabled: true }
    );
    expect(result.current.bindings.jobs).toMatchObject({ loaded: 0, total: 0, hasMore: false });
    expect(result.current.bindings.triggers).toMatchObject({
      loaded: 0,
      total: 0,
      hasMore: false,
    });
  });
});
