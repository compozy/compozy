// Invariant: worktree creation forwards the active destination, and only aggregate creation
// reports the daemon-returned owner. Owning layer: useCreateWorktree; no prior suite owns it.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  aggregate: true,
  createWorktree: vi.fn(),
  removeWorktree: vi.fn(),
  dismissWorktree: vi.fn(),
  inspectWorktree: vi.fn(),
  destination: "default",
  notifyUser: vi.fn(),
}));

vi.mock("../../adapters/worktree-api", async importOriginal => ({
  ...(await importOriginal<typeof import("../../adapters/worktree-api")>()),
  adoptWorktree: vi.fn(),
  cancelWorktreeCreate: vi.fn(),
  createWorktree: mocks.createWorktree,
  dismissWorktree: mocks.dismissWorktree,
  removeWorktree: mocks.removeWorktree,
}));
vi.mock("../../adapters/worktree-exit-api", () => ({ inspectWorktree: mocks.inspectWorktree }));
vi.mock("@/lib/user-feedback", () => ({ notifyUser: mocks.notifyUser }));
vi.mock("@/systems/profiles", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/profiles")>()),
  useProfileReadScope: () => ({
    aggregate: mocks.aggregate,
    destination: mocks.destination,
    destinationOwner: {
      id: "00000000000000000000000000",
      name: mocks.destination,
      archived: false,
    },
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

// Invariant: immutable batch identity, per-item receipts and failed-only retry belong to lifecycle hooks.
import { useWorktreeRemovalBatch } from "@/systems/workspace/hooks/use-worktree-removal-batch";
import { useWorktreeRemovalSelection } from "@/systems/workspace/hooks/use-worktree-removal-selection";
import { toWorktreeNestEntries } from "@/systems/workspace/lib/worktree-display";
const removalProfile = { id: "00000000000000000000000000", name: "default", archived: false };

describe("worktree batch lifecycle", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.destination = "default";
  });

  it("Should preserve successful receipts and reconcile before retrying only failures", async () => {
    const ready = buildWorktreeFixture({ id: "wt_ready", workspace_id: "ws_alpha" });
    const missing = buildWorktreeFixture({
      id: "wt_missing",
      workspace_id: "ws_alpha",
      state: "missing",
    });
    mocks.inspectWorktree.mockImplementation(async (_workspace, id) => ({
      worktree: id === ready.id ? ready : missing,
    }));
    mocks.removeWorktree.mockResolvedValue(undefined);
    mocks.dismissWorktree
      .mockRejectedValueOnce(new Error("connection lost"))
      .mockResolvedValue(undefined);
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const { result } = renderHook(
      () =>
        useWorktreeRemovalBatch(
          {
            workspaceId: "ws_alpha",
            profile: removalProfile,
            worktrees: [ready, missing],
          },
          removalProfile
        ),
      { wrapper: createWrapper(client) }
    );
    await act(async () => {
      await Promise.all([result.current.run(), result.current.run()]);
    });
    expect(result.current.scopeMatches).toBe(true);
    expect(result.current.results.map(row => row.status)).toEqual(["success", "failed"]);
    await act(async () => {
      await result.current.run();
    });
    expect(result.current.results.map(row => row.status)).toEqual(["success", "success"]);
    expect(mocks.removeWorktree).toHaveBeenCalledTimes(1);
    expect(mocks.dismissWorktree).toHaveBeenCalledTimes(2);
    expect(mocks.removeWorktree).toHaveBeenCalledWith("ws_alpha", ready.id, {
      profile: "default",
      force: false,
    });
  });

  it("Should complete a selection larger than the repository queue without causing its own refusals", async () => {
    const rows = Array.from({ length: 12 }, (_, index) =>
      buildWorktreeFixture({ id: `wt_large_${index}`, workspace_id: "ws_alpha" })
    );
    mocks.inspectWorktree.mockImplementation(async (_workspace, id) => ({
      worktree: rows.find(row => row.id === id),
    }));
    let pending = 0;
    // The daemon allows one holder and eight waiters per repository.
    mocks.removeWorktree.mockImplementation(async () => {
      pending++;
      const saturated = pending > 9;
      await Promise.resolve();
      pending--;
      if (saturated) throw new Error("worktree_operation_in_progress");
    });
    const { result } = renderHook(
      () =>
        useWorktreeRemovalBatch(
          { workspaceId: "ws_alpha", profile: removalProfile, worktrees: rows },
          removalProfile
        ),
      { wrapper: createWrapper(new QueryClient()) }
    );
    await act(async () => {
      await result.current.run();
    });
    expect(result.current.results.map(row => row.status)).toEqual(rows.map(() => "success"));
    expect(mocks.removeWorktree).toHaveBeenCalledTimes(rows.length);
  });

  it("Should reconcile a lost successful response without another destructive request", async () => {
    const row = buildWorktreeFixture({ workspace_id: "ws_alpha" });
    mocks.inspectWorktree
      .mockResolvedValueOnce({ worktree: row })
      .mockResolvedValue({ worktree: { ...row, state: "removed" } });
    mocks.removeWorktree.mockRejectedValueOnce(new Error("connection lost"));
    const { result } = renderHook(
      () =>
        useWorktreeRemovalBatch(
          {
            workspaceId: "ws_alpha",
            profile: removalProfile,
            worktrees: [row],
          },
          removalProfile
        ),
      { wrapper: createWrapper(new QueryClient()) }
    );
    await act(async () => {
      await result.current.run();
    });
    expect(result.current.results[0]?.status).toBe("success");
    await act(async () => {
      await result.current.run();
    });
    expect(mocks.removeWorktree).toHaveBeenCalledTimes(1);
  });

  it("Should refuse changed ownership, state and profile before a mutation", async () => {
    const row = buildWorktreeFixture({ workspace_id: "ws_alpha", state: "missing" });
    const batch = { workspaceId: "ws_alpha", profile: removalProfile, worktrees: [row] };
    const { result, rerender } = renderHook(
      ({ profile }) => useWorktreeRemovalBatch(batch, profile),
      {
        initialProps: { profile: { ...removalProfile, id: "foreign" } },
        wrapper: createWrapper(new QueryClient()),
      }
    );
    await act(async () => {
      await result.current.run();
    });
    expect(mocks.inspectWorktree).not.toHaveBeenCalled();
    rerender({ profile: removalProfile });
    for (const current of [
      { ...row, profile_id: "foreign" },
      { ...row, state: "ready" as const },
      { ...row, agent_activity: "running" as const },
    ]) {
      mocks.inspectWorktree.mockResolvedValue({ worktree: current });
      await act(async () => {
        await result.current.run();
      });
      expect(result.current.results[0]?.status).toBe("failed");
    }
    expect(mocks.dismissWorktree).not.toHaveBeenCalled();
    expect(mocks.removeWorktree).not.toHaveBeenCalled();
  });

  it("Should freeze selected identities across live rows and reset on profile changes", () => {
    const row = buildWorktreeFixture({ workspace_id: "ws_alpha" });
    const newer = buildWorktreeFixture({ id: "wt_new", workspace_id: "ws_alpha" });
    const onRemove = vi.fn();
    const entries = (rows: (typeof row)[]) =>
      toWorktreeNestEntries({ worktrees: rows, discovered: [] });
    const { result, rerender } = renderHook(
      ({ rows, profile }) =>
        useWorktreeRemovalSelection("ws_alpha", entries(rows), profile, onRemove),
      { initialProps: { rows: [row], profile: removalProfile } }
    );
    act(() => result.current.selectAll());
    rerender({ rows: [row, newer], profile: removalProfile });
    act(() => result.current.confirm());
    expect(onRemove.mock.calls[0]?.[0].worktrees.map((item: typeof row) => item.id)).toEqual([
      row.id,
    ]);
    rerender({ rows: [row], profile: { ...removalProfile, id: "other" } });
    expect(result.current.count).toBe(0);
    expect(result.current.eligibleCount).toBe(0);
  });
});
