// Suite: task catalog worktree scoping
// Invariant: the worktree filter is server-side, enters both the query key and the cursor-bound page
// request, and can only ever accompany a workspace — matching the daemon's validation.
// Owning layer: tasks query layer. Canonical suite: this lib test.
import { describe, expect, it } from "vitest";

import { tasksKeys } from "../query-keys";
import { defaultTaskCatalogFilter, taskInboxFilterFromRouteSearch } from "../task-catalog-filter";
import {
  normalizeTaskInboxFilter,
  taskInboxPageRequest,
  taskInboxStableFilter,
} from "../task-inbox-query";
import {
  normalizeTaskListFilter,
  taskListPageRequest,
  taskListStableFilter,
} from "../task-list-query";
describe("task list worktree filter", () => {
  const scoped = {
    scope: "workspace",
    workspace: "ws_launch_hq",
    worktree: "wt_payments",
  } as const;

  it("Should carry the active worktree into list and inbox route filters", () => {
    expect(defaultTaskCatalogFilter(scoped).worktree).toBe("wt_payments");
    expect(taskInboxFilterFromRouteSearch(scoped, {}).worktree).toBe("wt_payments");
  });

  it("Should keep the worktree in the stable filter that backs the query key", () => {
    const stable = taskListStableFilter({ workspace: "ws_launch_hq", worktree: "wt_payments" });

    expect(stable.worktree).toBe("wt_payments");
  });

  it("Should give two window scopes two distinct query keys", () => {
    const windowOne = tasksKeys.list({ workspace: "ws_launch_hq", worktree: "wt_payments" });
    const windowTwo = tasksKeys.list({ workspace: "ws_launch_hq", worktree: "wt_auth" });

    expect(windowOne).not.toEqual(windowTwo);
  });

  it("Should give a fallback scope the same key as the unscoped workspace", () => {
    expect(tasksKeys.list({ workspace: "ws_launch_hq", worktree: undefined })).toEqual(
      tasksKeys.list({ workspace: "ws_launch_hq" })
    );
  });

  it("Should re-send the worktree with every cursor-bound page", () => {
    const stable = taskListStableFilter({ workspace: "ws_launch_hq", worktree: "wt_payments" });

    const page = taskListPageRequest(stable, "cursor-2");

    // Cursor identity includes the filter, so a page can never straddle scopes.
    expect(page.worktree).toBe("wt_payments");
    expect(page.cursor).toBe("cursor-2");
  });

  it("Should normalize a blank worktree away instead of sending an empty filter", () => {
    expect(
      normalizeTaskListFilter({ workspace: "ws_launch_hq", worktree: "  " }).worktree
    ).toBeUndefined();
  });
});

describe("task inbox worktree filter", () => {
  it("Should keep the worktree in the stable filter and every cursor page", () => {
    const stable = taskInboxStableFilter({
      workspace: "ws_launch_hq",
      worktree: "wt_payments",
    });

    expect(stable.worktree).toBe("wt_payments");
    expect(taskInboxPageRequest(stable, "cursor-2")).toMatchObject({
      worktree: "wt_payments",
      cursor: "cursor-2",
    });
  });

  it("Should normalize a blank inbox worktree away", () => {
    expect(normalizeTaskInboxFilter({ worktree: "  " }).worktree).toBeUndefined();
  });
});
