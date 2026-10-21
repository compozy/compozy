// Invariant: worktree creation forwards the active destination, and only aggregate creation
// reports the daemon-returned owner. Owning layer: useCreateWorktree; no prior suite owns it.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, render, screen, fireEvent } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  aggregate: true,
  createWorktree: vi.fn(),
  removeWorktree: vi.fn(),
  removeDiscoveredWorktree: vi.fn(),
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
  removeDiscoveredWorktree: mocks.removeDiscoveredWorktree,
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
import { WorktreeBulkRemoveDialog } from "../../components/worktree-bulk-remove-dialog";
import { WorktreeApiError } from "../../adapters/worktree-api";
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

  it("Should require a second confirmation and force only eligible refused targets", async () => {
    // Invariant: force is explicit and scoped to quantified loss refusals; this lifecycle suite owns the dialog-to-API boundary.
    const rows = ["dirty", "unpushed", "active", "clean"].map(id =>
      buildWorktreeFixture({ id, name: id, workspace_id: "ws_alpha" })
    );
    mocks.inspectWorktree.mockImplementation(async (_workspace, id) => ({
      worktree: rows.find(row => row.id === id),
    }));
    mocks.removeWorktree.mockImplementation(async (_workspace, id, options) => {
      if (id === "clean" || options.force) return null;
      throw new WorktreeApiError("refused", 409, {
        code: id === "active" ? "worktree_session_active" : `worktree_${id}_requires_force`,
        risk: {
          changed_files: id === "dirty" ? 2 : 0,
          insertions: 4,
          deletions: 1,
          unpushed_commits: id === "unpushed" ? 1 : 0,
          exists_on_remote: false,
        },
      });
    });
    render(
      createElement(WorktreeBulkRemoveDialog, {
        batch: { workspaceId: "ws_alpha", worktrees: rows },
        onClose: vi.fn(),
      }),
      { wrapper: createWrapper(new QueryClient()) }
    );
    fireEvent.click(screen.getByRole("checkbox", { name: "Also delete local branches" }));
    fireEvent.click(screen.getByRole("button", { name: "Remove selected" }));
    await screen.findByText("1 succeeded · 3 failed · 0 pending");
    expect(screen.getByText("2 changed files")).toBeInTheDocument();
    expect(screen.getByText("1 unpushed commit")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Force remove… (2)" }));
    expect(screen.getByTestId("worktree-bulk-force-dialog")).toBeInTheDocument();
    expect(mocks.removeWorktree).toHaveBeenCalledTimes(4);
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(mocks.removeWorktree).toHaveBeenCalledTimes(4);
    fireEvent.click(screen.getByRole("button", { name: "Force remove… (2)" }));
    fireEvent.click(screen.getByRole("button", { name: "Force remove" }));
    await screen.findByText("3 succeeded · 1 failed · 0 pending");
    expect(
      mocks.removeWorktree.mock.calls.filter(call => call[2].force).map(call => call[1])
    ).toEqual(["dirty", "unpushed"]);
    expect(mocks.removeWorktree).toHaveBeenLastCalledWith("ws_alpha", "unpushed", {
      profile: "default",
      force: true,
      deleteBranch: true,
    });
    expect(screen.queryByRole("button", { name: /Force remove/ })).not.toBeInTheDocument();
  });

  it("Should remove discovered paths without a record and force only a refused live checkout", async () => {
    const discovered = (path: string, stale: boolean) => ({
      name: path.split("/").at(-1) ?? path,
      branch: "",
      path,
      detached: true,
      stale,
      selectable: !stale,
      unavailable: stale,
      reason: stale ? "gitdir file points to non-existent location" : undefined,
    });
    const stale = discovered("/private/tmp/detached-stale", true);
    const live = discovered("/Users/ada/dev/detached-live", false);
    const lost = discovered("/Users/ada/dev/detached-lost", false);
    const orphaned = discovered("/Users/ada/dev/workspace-gone", false);
    mocks.removeDiscoveredWorktree.mockImplementation(async (_workspace, path, options) => {
      if (path === live.path && !options.force) {
        throw new WorktreeApiError("refused", 409, {
          code: "worktree_unpushed_requires_force",
          risk: {
            changed_files: 0,
            insertions: 0,
            deletions: 0,
            unpushed_commits: 2,
            exists_on_remote: false,
          },
        });
      }
      if (path === lost.path) {
        throw new WorktreeApiError("gone", 404, { code: "worktree_not_found" });
      }
      if (path === orphaned.path) {
        throw new WorktreeApiError("workspace gone", 404, { code: "workspace_not_found" });
      }
    });
    const { result } = renderHook(
      () =>
        useWorktreeRemovalBatch({
          workspaceId: "ws_alpha",
          worktrees: [],
          discovered: [stale, live, lost, orphaned],
        }),
      { wrapper: createWrapper(new QueryClient()) }
    );
    await act(async () => {
      await result.current.run();
    });
    expect(result.current.results.map(row => [row.status, row.message])).toEqual([
      ["success", "Stale Git entry cleared. Branch kept."],
      ["failed", expect.any(String)],
      ["success", "No longer listed by Git; confirmed by CompozyOS."],
      ["failed", expect.any(String)],
    ]);
    expect(mocks.inspectWorktree).not.toHaveBeenCalled();
    expect(result.current.forceEligible).toHaveLength(1);
    act(() => result.current.requestForce());
    await act(async () => {
      await result.current.confirmForce();
    });
    expect(result.current.results[1]).toMatchObject({
      status: "success",
      message: "Worktree removed. Branch kept.",
    });
    expect(mocks.removeDiscoveredWorktree.mock.calls.map(call => [call[1], call[2].force])).toEqual(
      [
        [stale.path, false],
        [live.path, false],
        [lost.path, false],
        [orphaned.path, false],
        [live.path, true],
      ]
    );
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
    const discovered = {
      name: "detached-a217e52e",
      branch: "",
      path: "/private/tmp/detached-a217e52e",
      detached: true,
      stale: true,
      selectable: false,
      unavailable: true,
    };
    const onRemove = vi.fn();
    const entries = (rows: (typeof row)[]) =>
      toWorktreeNestEntries({ worktrees: rows, discovered: [discovered] });
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
    expect(onRemove.mock.calls[0]?.[0].discovered).toEqual([discovered]);
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

  it("Should report branch deletion failure when the branch name could not be read", async () => {
    mocks.removeWorktree.mockResolvedValueOnce({
      name: "",
      deleted: false,
      reason: "delete_failed",
    });
    const { result } = renderHook(() => useRemoveWorktree("ws_alpha", "default"), {
      wrapper: createWrapper(new QueryClient()),
    });
    await act(async () => {
      await result.current.mutateAsync({ worktreeID: "wt_a", deleteBranch: true });
    });
    expect(mocks.notifyUser).toHaveBeenCalledWith({
      message: "Branch deletion failed. The branch result is unconfirmed.",
      tone: "warning",
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
