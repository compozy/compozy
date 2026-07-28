// Suite: task editor local draft identity
// Invariant: one source identity preserves edits; a new task/workspace source is visible in every render.
// Boundary IN: task/profile Query projections, active workspace, and template route search.
// Boundary OUT: mutations and the presentational editor surface.
import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const editorMocks = vi.hoisted(() => ({
  activeWorkspaceId: "ws_alpha",
  detail: undefined as unknown,
  profile: undefined as unknown,
  create: vi.fn(),
  createChild: vi.fn(),
  enqueue: vi.fn(),
  update: vi.fn(),
}));

vi.mock("../use-task-actions", () => ({
  useCreateTask: () => ({ isPending: false, mutateAsync: editorMocks.create }),
  useCreateChildTask: () => ({ isPending: false, mutateAsync: editorMocks.createChild }),
  useEnqueueTaskRun: () => ({ isPending: false, mutateAsync: editorMocks.enqueue }),
  useUpdateTask: () => ({ isPending: false, mutateAsync: editorMocks.update }),
}));

vi.mock("../use-task-profile", () => ({
  useTaskExecutionProfile: () => ({
    data: editorMocks.profile,
    isLoading: false,
  }),
}));

vi.mock("../use-tasks", () => ({
  useTask: () => ({
    data: editorMocks.detail,
    isLoading: false,
  }),
}));

vi.mock("@/systems/workspace", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/workspace")>();
  const workspaces = [
    { id: "ws_alpha", name: "Alpha", root_dir: "/workspace/alpha" },
    { id: "ws_beta", name: "Beta", root_dir: "/workspace/beta" },
  ];
  return {
    ...actual,
    useActiveWorkspace: () => ({
      activeWorkspace: workspaces.find(item => item.id === editorMocks.activeWorkspaceId),
      workspaces,
    }),
    useUserHomeDir: () => "/Users/operator",
  };
});

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
});

describe("task editor state identity", () => {
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
