import { act, renderHook } from "@testing-library/react";
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

vi.mock("@/systems/workspace", () => ({
  useActiveWorkspace: () => ({ activeWorkspaceId: "ws_default" }),
}));

vi.mock("@/systems/automation", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/automation")>()),
  useAutomationJobs: mocks.useAutomationJobs,
  useAutomationTriggers: mocks.useAutomationTriggers,
}));

vi.mock("@/systems/loops", () => {
  class LoopsApiError extends Error {
    status: number;

    constructor(message: string, status: number) {
      super(message);
      this.status = status;
    }
  }

  return {
    LoopsApiError,
    readLoopGraph: vi.fn(),
    useCreateLoop: () => ({ isPending: false, mutateAsync: mocks.createMutateAsync }),
    useDeleteLoop: () => ({
      error: null,
      isPending: false,
      mutateAsync: mocks.deleteMutateAsync,
      reset: mocks.deleteReset,
    }),
    useLoop: () => ({ data: { source: mocks.loopSource } }),
    useLoopConfig: () => ({ data: null }),
    useLoopRuns: () => ({ data: [] }),
    useLoops: () => ({ loops: [] }),
  };
});

const { useLoopDetail } = await import("@/systems/os/apps/loops/use-loop-detail");
const { LoopsApiError } = await import("@/systems/loops");

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
      isLoading: false,
      jobs: [],
      total: 0,
    });
    mocks.useAutomationTriggers.mockReturnValue({
      error: null,
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
      isLoading: false,
      total: 0,
      triggers: [],
    });
  });

  it("Should surface non-conflict fork failures instead of navigating silently", async () => {
    mocks.createMutateAsync.mockRejectedValue(new Error("daemon unavailable"));
    const { result } = renderHook(() => useLoopDetail("reviews-watch"));

    await act(async () => {
      await result.current.handlers.onOpenEditor();
    });

    expect(mocks.toastError).toHaveBeenCalledWith("daemon unavailable");
    expect(mocks.navigate).not.toHaveBeenCalledWith({
      to: "/loops/$name/editor",
      params: { name: "reviews-watch" },
    });
  });

  it("Should tolerate an existing workspace fork and navigate to the editor", async () => {
    mocks.createMutateAsync.mockRejectedValue(new LoopsApiError("Loop already exists", 409));
    const { result } = renderHook(() => useLoopDetail("reviews-watch"));

    await act(async () => {
      await result.current.handlers.onOpenEditor();
    });

    expect(mocks.toastError).not.toHaveBeenCalled();
    expect(mocks.navigate).toHaveBeenCalledWith({
      to: "/loops/$name/editor",
      params: { name: "reviews-watch" },
    });
  });

  it("Should await workspace deletion before replacing detail with the catalog", async () => {
    mocks.loopSource = "workspace";
    mocks.deleteMutateAsync.mockResolvedValue(undefined);
    const { result } = renderHook(() => useLoopDetail("reviews-watch"));

    await act(async () => {
      await result.current.handlers.onDelete();
    });

    expect(mocks.deleteMutateAsync).toHaveBeenCalledWith({
      workspaceId: "ws_default",
      name: "reviews-watch",
    });
    expect(mocks.toastSuccess).toHaveBeenCalledWith("Deleted workspace loop reviews-watch");
    expect(mocks.navigate).toHaveBeenCalledWith({ to: "/loops", replace: true });
  });

  it("Should keep the detail route when workspace deletion fails", async () => {
    mocks.loopSource = "workspace";
    mocks.deleteMutateAsync.mockRejectedValue(new Error("delete failed"));
    const { result } = renderHook(() => useLoopDetail("reviews-watch"));

    await act(async () => {
      await result.current.handlers.onDelete();
    });

    expect(mocks.navigate).not.toHaveBeenCalledWith({ to: "/loops", replace: true });
    expect(mocks.toastSuccess).not.toHaveBeenCalled();
  });

  it("Should query exact loop bindings inside the target workspace with pageable limits", () => {
    const { result } = renderHook(() => useLoopDetail("reviews-watch"));

    expect(mocks.useAutomationJobs).toHaveBeenCalledWith(
      {
        limit: 50,
        loop: "reviews-watch",
        scope: "workspace",
        workspace_id: "ws_default",
      },
      { enabled: true }
    );
    expect(mocks.useAutomationTriggers).toHaveBeenCalledWith(
      {
        limit: 50,
        loop: "reviews-watch",
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
