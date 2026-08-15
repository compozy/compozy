// Suite: worktree create dialog model
// Invariant (UT-134 create half / US-001, US-002): the generated name lives in the placeholder, the
// preview derives branch and path truthfully, field refusals clear when their field is edited, and
// an accepted 202 exposes the pending row so the creation can be cancelled daemon-side by id.
// Owning layer: workspace create-dialog hook. Canonical suite: this hook test.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const createWorktreeMock = vi.fn();
const cancelWorktreeCreateMock = vi.fn();

vi.mock("../../adapters/worktree-api", async () => {
  const actual = await vi.importActual<object>("../../adapters/worktree-api");
  return {
    ...actual,
    createWorktree: (...args: unknown[]) => createWorktreeMock(...args),
    cancelWorktreeCreate: (...args: unknown[]) => cancelWorktreeCreateMock(...args),
  };
});

import { buildWorktreeFixture, worktreeListingFixture } from "../../mocks/worktree-fixtures";
import { WorktreeApiError } from "../../adapters/worktree-api";
import { workspaceKeys } from "../../lib/query-keys";
import { useWorktreeCreateDialog } from "../use-worktree-create-dialog";

const WORKSPACE = "ws_launch_hq";

const pendingRow = buildWorktreeFixture({
  name: "docs-refresh",
  state: "pending",
  pending_phase: "branch",
});

function setup(listing = worktreeListingFixture) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  const hook = renderHook(
    () => useWorktreeCreateDialog(WORKSPACE, listing, { generatedName: "steady-heron" }),
    { wrapper }
  );
  return { ...hook, queryClient };
}

describe("useWorktreeCreateDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("Should preview the generated name while the field is untouched", () => {
    const { result } = setup();

    // The suggestion is a placeholder, so an empty field still previews what
    // submitting would actually create.
    expect(result.current.draft.name).toBe("");
    expect(result.current.preview?.branch).toBe("steady-heron");
  });

  it("Should derive branch and path from the typed name", () => {
    const { result } = setup();

    act(() => result.current.setDraft({ name: "Payments Retry!" }));

    // Mirrors the daemon's sanitizer so the preview matches the real record.
    expect(result.current.preview?.branch).toBe("payments-retry");
    expect(result.current.preview?.path).toContain("/payments-retry");
  });

  it("Should omit the path when the placement root cannot be derived", () => {
    const { result } = setup({ ...worktreeListingFixture, worktrees: [] });

    act(() => result.current.setDraft({ name: "docs-refresh" }));

    // The root is operator-configurable and no endpoint exposes it, so it is
    // left out rather than guessed from a default.
    expect(result.current.preview?.branch).toBe("docs-refresh");
    expect(result.current.preview?.path).toBeNull();
  });

  it("Should mark held branches with the worktree already holding them", () => {
    const { result } = setup();

    const held = result.current.branchCandidates.find(
      candidate => candidate.branch === "payments-retry"
    );
    expect(held?.heldBy).toBe("payments-retry");
  });

  it("Should resolve the branch holder from the structured refusal path", async () => {
    const holder = worktreeListingFixture.worktrees[0];
    createWorktreeMock.mockRejectedValue(
      new WorktreeApiError("Failed to create worktree", 409, {
        code: "branch_held_by_worktree",
        error: `branch_held_by_worktree: ${holder.path}`,
        details: { worktree_path: holder.path },
      })
    );
    const { result } = setup();

    act(() => result.current.submit());

    await waitFor(() => expect(result.current.heldByWorktree?.id).toBe(holder.id));
    expect(result.current.refusal?.message).toBe(
      "This branch is already checked out in another worktree."
    );
  });

  it("Should surface the pending row after the create is accepted", async () => {
    createWorktreeMock.mockResolvedValue(pendingRow);
    const { result } = setup();

    act(() => result.current.submit());

    // The 202 payload is durable, so the row exists before materialization ends.
    await waitFor(() => expect(result.current.pendingWorktree?.id).toBe(pendingRow.id));
  });

  it("Should cancel the accepted creation daemon-side by worktree id", async () => {
    createWorktreeMock.mockResolvedValue(pendingRow);
    cancelWorktreeCreateMock.mockResolvedValue(undefined);
    const { result } = setup();

    act(() => result.current.submit());
    await waitFor(() => expect(result.current.pendingWorktree).not.toBeNull());

    act(() => result.current.cancelCreate());

    // Cancelling reaches the daemon with the id; closing the dialog would not.
    await waitFor(() =>
      expect(cancelWorktreeCreateMock).toHaveBeenCalledWith(WORKSPACE, pendingRow.id)
    );
    await waitFor(() => expect(result.current.pendingWorktree).toBeNull());
  });

  it("Should offer no cancel affordance before a worktree id exists", () => {
    const { result } = setup();

    // There is nothing durable to cancel yet, and aborting the in-flight POST
    // would not stop a creation the daemon already accepted.
    expect(result.current.pendingWorktree).toBeNull();
    act(() => result.current.cancelCreate());
    expect(cancelWorktreeCreateMock).not.toHaveBeenCalled();
  });

  it("Should retire the cancel affordance once the row reaches ready", async () => {
    createWorktreeMock.mockResolvedValue(pendingRow);
    const readyRow = buildWorktreeFixture({ ...pendingRow, state: "ready" });
    const { result, rerender } = renderHookWithListing(readyRow);

    act(() => result.current.submit());
    await waitFor(() => expect(createWorktreeMock).toHaveBeenCalled());
    rerender();

    // Past materialization there is nothing left to cancel.
    expect(result.current.pendingWorktree).toBeNull();
  });

  it("Should complete exactly once only after the accepted row reaches ready", async () => {
    createWorktreeMock.mockResolvedValue(pendingRow);
    const onCreated = vi.fn();
    let listing = { ...worktreeListingFixture, worktrees: [pendingRow] };
    const { result, rerender, queryClient } = renderHookWithDynamicListing(
      () => listing,
      onCreated
    );

    act(() => result.current.submit());

    await waitFor(() => expect(result.current.pendingWorktree?.id).toBe(pendingRow.id));
    expect(onCreated).not.toHaveBeenCalled();

    const readyRow = buildWorktreeFixture({ ...pendingRow, state: "ready" });
    act(() => {
      listing = { ...listing, worktrees: [readyRow] };
      queryClient.setQueryData(workspaceKeys.worktrees(WORKSPACE), listing);
      rerender();
    });

    await waitFor(() => expect(onCreated).toHaveBeenCalledWith(readyRow));
    expect(onCreated).toHaveBeenCalledOnce();
    expect(result.current.pendingWorktree).toBeNull();

    rerender();
    expect(onCreated).toHaveBeenCalledOnce();
  });

  it("Should treat a ready row with failed setup as completed creation", async () => {
    createWorktreeMock.mockResolvedValue(pendingRow);
    const onCreated = vi.fn();
    let listing = { ...worktreeListingFixture, worktrees: [pendingRow] };
    const { result, rerender, queryClient } = renderHookWithDynamicListing(
      () => listing,
      onCreated
    );

    act(() => result.current.submit());
    await waitFor(() => expect(result.current.pendingWorktree).not.toBeNull());

    const readyWithSetupFailure = buildWorktreeFixture({
      ...pendingRow,
      state: "ready",
      setup_state: "failed",
      setup_error: "bootstrap failed",
    });
    act(() => {
      listing = { ...listing, worktrees: [readyWithSetupFailure] };
      queryClient.setQueryData(workspaceKeys.worktrees(WORKSPACE), listing);
      rerender();
    });

    await waitFor(() => expect(onCreated).toHaveBeenCalledWith(readyWithSetupFailure));
    expect(onCreated).toHaveBeenCalledOnce();
  });

  it("Should surface an asynchronous materialization failure without completing", async () => {
    createWorktreeMock.mockResolvedValue(pendingRow);
    const onCreated = vi.fn();
    let listing = { ...worktreeListingFixture, worktrees: [pendingRow] };
    const { result, rerender, queryClient } = renderHookWithDynamicListing(
      () => listing,
      onCreated
    );

    act(() => result.current.submit());
    await waitFor(() => expect(result.current.pendingWorktree).not.toBeNull());

    act(() => {
      queryClient.setQueryData(
        workspaceKeys.worktreeMaterializationFailure(WORKSPACE, pendingRow.id),
        "Checkout failed and was rolled back."
      );
      listing = { ...listing, worktrees: [] };
      rerender();
    });

    await waitFor(() =>
      expect(result.current.creationError).toBe("Checkout failed and was rolled back.")
    );
    expect(result.current.pendingWorktree).toBeNull();
    expect(onCreated).not.toHaveBeenCalled();
  });
});

/** Renders with a listing that already carries the row in its final state. */
function renderHookWithListing(row: ReturnType<typeof buildWorktreeFixture>) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  const listing = { ...worktreeListingFixture, worktrees: [row] };
  const hook = renderHook(
    () => useWorktreeCreateDialog(WORKSPACE, listing, { generatedName: "steady-heron" }),
    { wrapper }
  );
  return { ...hook, queryClient };
}

function renderHookWithDynamicListing(
  listing: () => typeof worktreeListingFixture,
  onCreated: (worktree: ReturnType<typeof buildWorktreeFixture>) => void
) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  const hook = renderHook(
    () =>
      useWorktreeCreateDialog(WORKSPACE, listing(), {
        generatedName: "steady-heron",
        onCreated,
      }),
    { wrapper }
  );
  return { ...hook, queryClient };
}
