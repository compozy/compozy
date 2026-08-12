// Suite: task editor local draft identity
// Invariant: one source identity preserves edits, a new task/workspace source is visible in every
// render, and each editor admits only one concurrent submission across source changes.
// Boundary IN: task/profile Query projections, active workspace, and template route search.
// Boundary OUT: mutations and the presentational editor surface.
import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const editorMocks = vi.hoisted(() => ({
  activeWorkspaceId: "ws_alpha",
  detail: undefined as unknown,
  detailOptions: undefined as unknown,
  profile: undefined as unknown,
  profileOptions: undefined as unknown,
  create: vi.fn(),
  createChild: vi.fn(),
  enqueue: vi.fn(),
  activeWorkspaceOptions: undefined as unknown,
  userHomeDirOptions: undefined as unknown,
  update: vi.fn(),
}));

const editorWorkspaces = vi.hoisted(() => [
  { id: "ws_alpha", name: "Alpha", root_dir: "/workspace/alpha" },
  { id: "ws_beta", name: "Beta", root_dir: "/workspace/beta" },
]);

vi.mock("../use-task-actions", () => ({
  useCreateTask: () => ({ isPending: false, mutateAsync: editorMocks.create }),
  useCreateChildTask: () => ({ isPending: false, mutateAsync: editorMocks.createChild }),
  useEnqueueTaskRun: () => ({ isPending: false, mutateAsync: editorMocks.enqueue }),
  useUpdateTask: () => ({ isPending: false, mutateAsync: editorMocks.update }),
}));

vi.mock("../use-task-profile", () => ({
  useTaskExecutionProfile: (_id: string, options: unknown) => {
    editorMocks.profileOptions = options;
    return {
      data: editorMocks.profile,
      isLoading: false,
    };
  },
}));

vi.mock("../use-tasks", () => ({
  useTask: (_id: string, options: unknown) => {
    editorMocks.detailOptions = options;
    return {
      data: editorMocks.detail,
      isLoading: false,
    };
  },
}));

vi.mock("@/systems/workspace/hooks/use-active-workspace", () => ({
  useActiveWorkspace: (options: unknown) => {
    editorMocks.activeWorkspaceOptions = options;
    return {
      activeWorkspace: editorWorkspaces.find(item => item.id === editorMocks.activeWorkspaceId),
      workspaces: editorWorkspaces,
    };
  },
}));

vi.mock("@/systems/workspace/hooks/use-user-home-dir", () => ({
  useUserHomeDir: (options: unknown) => {
    editorMocks.userHomeDirOptions = options;
    return "/Users/operator";
  },
}));

import { buildDetailFixture, buildTaskExecutionProfileFixture } from "../../mocks/fixtures";
import { useTaskCreateState } from "../use-task-create-state";
import { useTaskEditState } from "../use-task-edit-state";

function buildEditorDetail(id: string, title: string, updatedAt: string) {
  const detail = buildDetailFixture();
  return buildDetailFixture({
    task: {
      ...detail.task,
      id,
      title,
      updated_at: updatedAt,
    },
  });
}

beforeEach(() => {
  vi.clearAllMocks();
  editorMocks.activeWorkspaceId = "ws_alpha";
  editorMocks.detail = buildEditorDetail("task_alpha", "Alpha source", "2026-07-25T10:00:00Z");
  editorMocks.profile = buildTaskExecutionProfileFixture({
    task_id: "task_alpha",
    updated_at: "2026-07-25T10:00:00Z",
  });
  editorMocks.detailOptions = undefined;
  editorMocks.profileOptions = undefined;
  editorMocks.activeWorkspaceOptions = undefined;
  editorMocks.userHomeDirOptions = undefined;
});

describe("task editor state identity", () => {
  it("Should disable create scope observers while preserving the cached draft", () => {
    const { result } = renderHook(() =>
      useTaskCreateState({}, vi.fn(), { liveDataEnabled: false })
    );

    expect(editorMocks.activeWorkspaceOptions).toEqual({ enabled: false });
    expect(editorMocks.userHomeDirOptions).toEqual({ enabled: false });
    expect(result.current.draft.workspaceId).toBe("ws_alpha");
  });

  it("Should preserve create edits across template changes and reset before rendering a new workspace", () => {
    const renderedTitles: string[] = [];
    const navigate = vi.fn();
    const { result, rerender } = renderHook(
      ({ template }: { template: "one_shot" | "human_in_loop" }) => {
        const state = useTaskCreateState({ template }, navigate);
        renderedTitles.push(state.draft.title);
        return state;
      },
      { initialProps: { template: "one_shot" } }
    );

    act(() => {
      result.current.setDraft(current => ({ ...current, title: "Operator draft" }));
    });
    rerender({ template: "human_in_loop" });
    expect(result.current.draft.title).toBe("Operator draft");

    renderedTitles.length = 0;
    editorMocks.activeWorkspaceId = "ws_beta";
    rerender({ template: "human_in_loop" });

    expect(renderedTitles.length).toBeGreaterThan(0);
    expect(renderedTitles.every(title => title === "")).toBe(true);
    expect(result.current.draft.workspaceId).toBe("ws_beta");
  });

  it("Should preserve catalog filters when changing the create template", () => {
    const navigate = vi.fn();
    const search = {
      inboxLane: "approvals" as const,
      inboxQuery: "review",
      inboxUnread: true as const,
      mode: "inbox" as const,
      owner: "agent_session:codex",
      priority: "high" as const,
      query: "release",
      sort: "priority" as const,
      status: "ready" as const,
      template: "one_shot" as const,
    };
    const { result } = renderHook(() => useTaskCreateState(search, navigate));

    act(() => {
      result.current.handleTemplateChange("human_in_loop");
    });

    expect(navigate).toHaveBeenCalledWith({
      pathname: "/tasks/new",
      search: { ...search, template: "human_in_loop" },
    });
  });

  it("Should admit only one create submission while the first request is pending", async () => {
    let resolveCreate: ((value: { draft: boolean; id: string }) => void) | undefined;
    editorMocks.create.mockReturnValueOnce(
      new Promise(resolve => {
        resolveCreate = resolve;
      })
    );
    const { result } = renderHook(() => useTaskCreateState({}, vi.fn()));
    const draft = { ...result.current.draft, title: "Single submission" };

    let first: ReturnType<typeof result.current.handleSubmit> | undefined;
    let second: ReturnType<typeof result.current.handleSubmit> | undefined;
    await act(async () => {
      first = result.current.handleSubmit(draft, true);
      second = result.current.handleSubmit(draft, true);
      await Promise.resolve();
    });

    expect(editorMocks.create).toHaveBeenCalledTimes(1);
    await expect(second).resolves.toBeNull();

    resolveCreate?.({ draft: true, id: "task-created" });
    await act(async () => {
      await first;
    });
  });

  it("Should keep the create guard while the template changes during submission", async () => {
    let resolveCreate: ((value: { draft: boolean; id: string }) => void) | undefined;
    editorMocks.create.mockReturnValueOnce(
      new Promise(resolve => {
        resolveCreate = resolve;
      })
    );
    const { result, rerender } = renderHook(
      ({ template }: { template: "one_shot" | "human_in_loop" }) =>
        useTaskCreateState({ template }, vi.fn()),
      { initialProps: { template: "one_shot" } }
    );
    const draft = { ...result.current.draft, title: "Single submission" };

    let first: ReturnType<typeof result.current.handleSubmit> | undefined;
    await act(async () => {
      first = result.current.handleSubmit(draft, true);
      await Promise.resolve();
    });

    rerender({ template: "human_in_loop" });
    const second = result.current.handleSubmit(result.current.draft, true);

    expect(editorMocks.create).toHaveBeenCalledTimes(1);
    await expect(second).resolves.toBeNull();

    resolveCreate?.({ draft: true, id: "task-created" });
    await act(async () => {
      await first;
    });
  });

  it("Should admit only one edit submission while the first request is pending", async () => {
    let resolveUpdate: (() => void) | undefined;
    editorMocks.update.mockReturnValueOnce(
      new Promise<void>(resolve => {
        resolveUpdate = resolve;
      })
    );
    const { result } = renderHook(() => useTaskEditState("task_alpha", vi.fn()));
    const draft = { ...result.current.draft, title: "Saved once" };

    let first: ReturnType<typeof result.current.handleSubmit> | undefined;
    let second: ReturnType<typeof result.current.handleSubmit> | undefined;
    await act(async () => {
      first = result.current.handleSubmit(draft);
      second = result.current.handleSubmit(draft);
      await Promise.resolve();
    });

    expect(editorMocks.update).toHaveBeenCalledTimes(1);
    await expect(second).resolves.toBeNull();

    resolveUpdate?.();
    await act(async () => {
      await first;
    });
  });

  it("Should disable task reads and polling while the edit window is inactive", () => {
    renderHook(() =>
      useTaskEditState("task_alpha", vi.fn(), {
        liveDataEnabled: false,
      })
    );

    expect(editorMocks.detailOptions).toEqual({ enabled: false, refetchIntervalMs: false });
    expect(editorMocks.profileOptions).toEqual({ enabled: false, refetchIntervalMs: false });
  });

  it("Should preserve edits for one task identity and expose a replacement task in every render", () => {
    const renderedTitles: string[] = [];
    const navigate = vi.fn();
    const { result, rerender } = renderHook(
      ({ id }: { id: string }) => {
        const state = useTaskEditState(id, navigate);
        renderedTitles.push(state.draft.title);
        return state;
      },
      { initialProps: { id: "task_alpha" } }
    );

    act(() => {
      result.current.setDraft(current => ({ ...current, title: "Unsaved operator edit" }));
    });
    rerender({ id: "task_alpha" });
    expect(result.current.draft.title).toBe("Unsaved operator edit");

    editorMocks.detail = buildEditorDetail("task_beta", "Beta source", "2026-07-25T11:00:00Z");
    editorMocks.profile = buildTaskExecutionProfileFixture({
      task_id: "task_beta",
      updated_at: "2026-07-25T11:00:00Z",
    });
    renderedTitles.length = 0;
    rerender({ id: "task_beta" });

    expect(renderedTitles.length).toBeGreaterThan(0);
    expect(renderedTitles.every(title => title === "Beta source")).toBe(true);
    expect(result.current.draft.title).toBe("Beta source");
  });
});
