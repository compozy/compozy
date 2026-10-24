import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { SessionCreateProvider, createSessionCreateStore } from "@/systems/session";
import { statusKeys } from "@/systems/status/lib/query-keys";
import { prepareWorktreeExitPlan, runWorktreeExitAction } from "../../adapters/worktree-exit-api";
import { useWorktreeDetailContext } from "../../hooks/use-worktree-detail-context";
import { workspaceKeys } from "../../lib/query-keys";
import { buildWorktreeFixture } from "../../mocks/worktree-fixtures";
import { exitPlanOpenPrFixture } from "../../mocks/worktree-exit-fixtures";
import type { WorktreeExitPlanPayload } from "../../types";
import { WorktreePrDialog } from "../worktree-pr-dialog";

import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { toWorktreeExitLadder } from "../../lib/worktree-exit-ladder";
import { exitPlanNoRemoteFixture } from "../../mocks/worktree-exit-fixtures";
import { WorktreeExitControl } from "../worktree-exit-control";

describe("WorktreeExitControl", () => {
  it("Should reveal daemon-computed alternatives without breaking the menu", async () => {
    const user = userEvent.setup();

    render(
      <WorktreeExitControl
        ladder={toWorktreeExitLadder(exitPlanNoRemoteFixture)}
        onAction={vi.fn()}
      />
    );

    await user.click(screen.getByRole("button", { name: "Git actions" }));

    const menu = await screen.findByRole("menu", { name: "Git actions" });

    expect(menu).toBeInTheDocument();
    expect(menu.querySelector("[data-slot='dropdown-menu-group']")).toBeInTheDocument();
  });
});

vi.mock("../../adapters/worktree-exit-api", async importOriginal => ({
  ...(await importOriginal<typeof import("../../adapters/worktree-exit-api")>()),
  prepareWorktreeExitPlan: vi.fn(),
  runWorktreeExitAction: vi.fn(),
}));

function ColdWorktreeExit() {
  const model = useWorktreeDetailContext({
    workspaceId: "ws_cold",
    worktreeId: "wt_cold",
    enabled: false,
  });
  return (
    <>
      {model.ladder ? (
        <WorktreeExitControl
          ladder={model.ladder}
          isRunning={model.isRunning}
          onAction={model.onAction}
        />
      ) : null}
      {model.prDialog ? <WorktreePrDialog {...model.prDialog} /> : null}
    </>
  );
}

describe("PR action preparation", () => {
  it("Should resolve a cold native capability before opening the PR dialog", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const prepared = { ...exitPlanOpenPrFixture, worktree_id: "wt_cold" };
    const cold: WorktreeExitPlanPayload = {
      ...prepared,
      forge: undefined,
      pr_prefill: undefined,
      actions: prepared.actions.map(row =>
        row.action === "open_pr"
          ? { ...row, url: "https://github.com/acme/repo/compare/main...feature" }
          : row
      ),
    };
    queryClient.setQueryData(workspaceKeys.list(), []);
    queryClient.setQueryData(statusKeys.current(), {});
    queryClient.setQueryData(workspaceKeys.worktrees("ws_cold"), {
      worktrees: [buildWorktreeFixture({ id: "wt_cold", workspace_id: "ws_cold" })],
      discovered: [],
    });
    queryClient.setQueryData(workspaceKeys.worktreeStatus("ws_cold", "wt_cold"), {
      worktree_id: "wt_cold",
      status: {},
    });
    queryClient.setQueryData(workspaceKeys.worktreeExit("ws_cold", "wt_cold"), cold);
    let resolvePreparation!: (plan: WorktreeExitPlanPayload) => void;
    vi.mocked(prepareWorktreeExitPlan).mockImplementation(
      () =>
        new Promise(resolve => {
          resolvePreparation = resolve;
        })
    );
    vi.mocked(runWorktreeExitAction).mockResolvedValue("op-cold");
    const openBrowser = vi.spyOn(window, "open").mockImplementation(() => null);
    const user = userEvent.setup();
    const view = render(
      <QueryClientProvider client={queryClient}>
        <SessionCreateProvider store={createSessionCreateStore()}>
          <ColdWorktreeExit />
        </SessionCreateProvider>
      </QueryClientProvider>
    );
    expect(prepareWorktreeExitPlan).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Open PR" }));
    await waitFor(() => expect(prepareWorktreeExitPlan).toHaveBeenCalledWith("ws_cold", "wt_cold"));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(openBrowser).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Open PR" })).toBeDisabled();
    await act(async () => resolvePreparation(prepared));
    expect(await screen.findByLabelText("Title")).toHaveValue(prepared.pr_prefill?.title);
    expect(screen.getByLabelText("Description")).toHaveValue(prepared.pr_prefill?.body);
    await user.click(screen.getByRole("button", { name: "PR actions" }));
    expect(await screen.findByRole("menuitem", { name: "Open PR (draft)" })).toBeEnabled();
    expect(openBrowser).not.toHaveBeenCalled();
    await user.click(screen.getByRole("menuitem", { name: "Open PR (draft)" }));
    await waitFor(() =>
      expect(runWorktreeExitAction).toHaveBeenCalledWith("ws_cold", "wt_cold", {
        action: "open_pr",
        draft: true,
        title: prepared.pr_prefill?.title,
        body: prepared.pr_prefill?.body,
        base: prepared.base,
      })
    );
    view.unmount();
    queryClient.clear();
    openBrowser.mockRestore();
  });
});
