// Invariant: worktree creation forwards the active destination, and only aggregate creation
// reports the daemon-returned owner. Owning layer: useCreateWorktree; no prior suite owns it.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  aggregate: true,
  createWorktree: vi.fn(),
  destination: "default",
  notifyUser: vi.fn(),
}));

vi.mock("../../adapters/worktree-api", () => ({
  adoptWorktree: vi.fn(),
  cancelWorktreeCreate: vi.fn(),
  createWorktree: mocks.createWorktree,
  dismissWorktree: vi.fn(),
  removeWorktree: vi.fn(),
}));
vi.mock("@/lib/user-feedback", () => ({ notifyUser: mocks.notifyUser }));
vi.mock("@/systems/profiles", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/profiles")>()),
  useProfileReadScope: () => ({
    aggregate: mocks.aggregate,
    destination: mocks.destination,
  }),
}));

import { buildWorktreeFixture } from "../../mocks/worktree-fixtures";
import { useCreateWorktree } from "../use-worktrees";

function createWrapper(queryClient: QueryClient) {
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

function renderCreateWorktree() {
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
  });
  return renderHook(() => useCreateWorktree("ws_alpha"), {
    wrapper: createWrapper(queryClient),
  });
}

describe("useCreateWorktree", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.aggregate = true;
    mocks.destination = "default";
  });

  it("Should forward the aggregate destination and report the persisted owner", async () => {
    const worktree = buildWorktreeFixture({ profile_name: "marketing" });
    mocks.createWorktree.mockResolvedValue(worktree);
    const { result } = renderCreateWorktree();

    await act(async () => {
      await result.current.mutateAsync({ name: "campaign-review" });
    });

    expect(mocks.createWorktree).toHaveBeenCalledWith(
      "ws_alpha",
      { name: "campaign-review" },
      "default"
    );
    expect(mocks.notifyUser).toHaveBeenCalledWith({
      message: "Created in marketing.",
      tone: "success",
    });
  });

  it("Should forward a scoped destination without aggregate feedback", async () => {
    mocks.aggregate = false;
    mocks.destination = "marketing";
    mocks.createWorktree.mockResolvedValue(buildWorktreeFixture({ profile_name: "marketing" }));
    const { result } = renderCreateWorktree();

    await act(async () => {
      await result.current.mutateAsync({ name: "campaign-review" });
    });

    expect(mocks.createWorktree).toHaveBeenCalledWith(
      "ws_alpha",
      { name: "campaign-review" },
      "marketing"
    );
    expect(mocks.notifyUser).not.toHaveBeenCalled();
  });
});
