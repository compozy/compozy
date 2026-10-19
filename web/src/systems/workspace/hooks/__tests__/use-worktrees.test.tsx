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
import { useCreateWorktree, useDismissWorktree, useRemoveWorktree } from "../use-worktrees";

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

// Invariant: immutable batch identity, per-item receipts, owner-scoped authority and
// failed-only retry belong to lifecycle hooks.
import { useWorktreeRemovalBatch } from "@/systems/workspace/hooks/use-worktree-removal-batch";
import { useWorktreeRemovalSelection } from "@/systems/workspace/hooks/use-worktree-removal-selection";
import { toWorktreeNestEntries } from "@/systems/workspace/lib/worktree-display";

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
    mocks.removeWorktree.mockResolvedValue(null);
    mocks.dismissWorktree
      .mockRejectedValueOnce(new Error("connection lost"))
      .mockResolvedValue(null);
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const { result } = renderHook(
      () => useWorktreeRemovalBatch({ workspaceId: "ws_alpha", worktrees: [ready, missing] }),
      { wrapper: createWrapper(client) }
    );
    await act(async () => {
      await Promise.all([result.current.run(), result.current.run()]);
    });
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
      deleteBranch: false,
    });
  });

  it("Should act as each row owner and report the opted-in branch outcome per row", async () => {
    const ready = buildWorktreeFixture({ id: "wt_ready", workspace_id: "ws_alpha" });
    const missing = buildWorktreeFixture({
      id: "wt_missing",
      workspace_id: "ws_alpha",
      state: "missing",
      branch: "fix/missing",
      profile_id: "profile-design",
      profile_name: "open-design",
    });
    mocks.inspectWorktree.mockImplementation(async (_workspace, id) => ({
      worktree: id === ready.id ? ready : missing,
    }));
    mocks.removeWorktree.mockResolvedValue({ name: ready.branch, deleted: true });
    mocks.dismissWorktree.mockResolvedValue({
      name: "fix/missing",
      deleted: false,
      reason: "not_fully_merged",
    });
    const { result } = renderHook(
      () => useWorktreeRemovalBatch({ workspaceId: "ws_alpha", worktrees: [ready, missing] }),
      { wrapper: createWrapper(new QueryClient()) }
    );
    act(() => result.current.setDeleteBranch(true));
    await act(async () => {
      await result.current.run();
    });
    // The opt-in is frozen once the confirmed batch has started.
    act(() => result.current.setDeleteBranch(false));
    expect(result.current.deleteBranch).toBe(true);
    expect(mocks.dismissWorktree).toHaveBeenCalledWith("ws_alpha", missing.id, {
      profile: "open-design",
      force: false,
      deleteBranch: true,
    });
    expect(result.current.results.map(row => [row.status, row.message])).toEqual([
      ["success", `Worktree removed. Branch ${ready.branch} deleted.`],
      [
        "success",
        "Record dismissed. Files and history preserved. Branch fix/missing kept: it has unmerged commits.",
      ],
    ]);
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
      return null;
    });
    const { result } = renderHook(
      () => useWorktreeRemovalBatch({ workspaceId: "ws_alpha", worktrees: rows }),
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
      () => useWorktreeRemovalBatch({ workspaceId: "ws_alpha", worktrees: [row] }),
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

  it("Should keep an unconfirmed branch step failed and retry only that step", async () => {
    const row = buildWorktreeFixture({ workspace_id: "ws_alpha" });
    mocks.inspectWorktree
      .mockResolvedValueOnce({ worktree: row })
      .mockResolvedValue({ worktree: { ...row, state: "removed" } });
    mocks.removeWorktree
      .mockRejectedValueOnce(new Error("worktree_operation_in_progress"))
      .mockResolvedValueOnce({ name: row.branch, deleted: true });
    const { result } = renderHook(
      () => useWorktreeRemovalBatch({ workspaceId: "ws_alpha", worktrees: [row] }),
      { wrapper: createWrapper(new QueryClient()) }
    );
    act(() => result.current.setDeleteBranch(true));
    await act(async () => {
      await result.current.run();
    });
    expect(result.current.results[0]).toMatchObject({
      status: "failed",
      message:
        "Worktree cleaned up, but the branch result is unconfirmed. Retry repeats only the branch step.",
    });
    await act(async () => {
      await result.current.run();
    });
    expect(result.current.results[0]).toMatchObject({
      status: "success",
      message: `Already removed or dismissed; confirmed by CompozyOS. Branch ${row.branch} deleted.`,
    });
    expect(mocks.removeWorktree).toHaveBeenLastCalledWith("ws_alpha", row.id, {
      profile: "default",
      force: false,
      deleteBranch: true,
    });
  });

  it("Should refuse changed ownership and state before a mutation", async () => {
    const row = buildWorktreeFixture({ workspace_id: "ws_alpha", state: "missing" });
    const { result } = renderHook(
      () => useWorktreeRemovalBatch({ workspaceId: "ws_alpha", worktrees: [row] }),
      { wrapper: createWrapper(new QueryClient()) }
    );
    for (const current of [
      { ...row, profile_id: "foreign" },
      { ...row, profile_archived: true },
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

  it("Should select ready and missing rows of any live owner and freeze the selection", () => {
    const row = buildWorktreeFixture({ workspace_id: "ws_alpha" });
    const foreignMissing = buildWorktreeFixture({
      id: "wt_foreign_missing",
      workspace_id: "ws_alpha",
      state: "missing",
      profile_id: "profile-design",
      profile_name: "open-design",
    });
    const orphan = buildWorktreeFixture({
      id: "wt_orphan",
      workspace_id: "ws_alpha",
      state: "missing",
      profile_name: "",
    });
    const archived = buildWorktreeFixture({
      id: "wt_archived",
      workspace_id: "ws_alpha",
      profile_archived: true,
    });
    const newer = buildWorktreeFixture({ id: "wt_new", workspace_id: "ws_alpha" });
    const onRemove = vi.fn();
    const entries = (rows: (typeof row)[]) =>
      toWorktreeNestEntries({ worktrees: rows, discovered: [] });
    const { result, rerender } = renderHook(
      ({ rows }) => useWorktreeRemovalSelection("ws_alpha", entries(rows), onRemove),
      { initialProps: { rows: [row, foreignMissing, orphan, archived] } }
    );
    const [, , orphanEntry, archivedEntry] = entries([row, foreignMissing, orphan, archived]);
    expect(result.current.reason(orphanEntry!)).toBe("Owning profile is unavailable");
    expect(result.current.reason(archivedEntry!)).toBe(
      "Archived profile — restore the profile first"
    );
    act(() => result.current.selectAll());
    rerender({ rows: [row, foreignMissing, orphan, archived, newer] });
    act(() => result.current.confirm());
    expect(onRemove.mock.calls[0]?.[0].worktrees.map((item: typeof row) => item.id)).toEqual([
      row.id,
      foreignMissing.id,
    ]);
  });
});

// Invariant: singular cleanup forwards the branch opt-in and reports its outcome, offering
// one forced retry only for an unmerged branch. Owning layer: lifecycle mutation hooks.
describe("singular cleanup branch outcome", () => {
  beforeEach(() => vi.clearAllMocks());

  it("Should forward the opt-in and offer a forced retry for an unmerged branch", async () => {
    mocks.removeWorktree
      .mockResolvedValueOnce({ name: "fix/a", deleted: false, reason: "not_fully_merged" })
      .mockResolvedValueOnce({ name: "fix/a", deleted: true });
    const { result } = renderHook(() => useRemoveWorktree("ws_alpha", "default"), {
      wrapper: createWrapper(new QueryClient()),
    });
    await act(async () => {
      await result.current.mutateAsync({ worktreeID: "wt_a", deleteBranch: true });
    });
    expect(mocks.removeWorktree).toHaveBeenCalledWith("ws_alpha", "wt_a", {
      force: undefined,
      deleteBranch: true,
      profile: "default",
    });
    const kept = mocks.notifyUser.mock.calls[0]?.[0];
    expect(kept).toMatchObject({
      message: "Branch fix/a kept: it has unmerged commits.",
      tone: "warning",
    });
    await act(async () => {
      kept.action.onClick();
      await Promise.resolve();
    });
    expect(mocks.removeWorktree).toHaveBeenLastCalledWith("ws_alpha", "wt_a", {
      force: true,
      deleteBranch: true,
      profile: "default",
    });
    expect(mocks.notifyUser).toHaveBeenLastCalledWith({
      message: "Branch fix/a deleted.",
      tone: "success",
      action: undefined,
    });
  });

  it("Should keep plain dismissal silent and never offer force for protected branches", async () => {
    mocks.dismissWorktree
      .mockResolvedValueOnce(null)
      .mockResolvedValueOnce({ name: "main", deleted: false, reason: "default_branch" });
    const { result } = renderHook(() => useDismissWorktree("ws_alpha", "default"), {
      wrapper: createWrapper(new QueryClient()),
    });
    await act(async () => {
      await result.current.mutateAsync({ worktreeID: "wt_a" });
    });
    expect(mocks.notifyUser).not.toHaveBeenCalled();
    await act(async () => {
      await result.current.mutateAsync({ worktreeID: "wt_b", deleteBranch: true, force: true });
    });
    expect(mocks.notifyUser).toHaveBeenCalledWith({
      message: "Branch main kept: it is the default branch.",
      tone: "warning",
      action: undefined,
    });
  });
});
