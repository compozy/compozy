import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { taskExecutionProfileFixture } from "../../mocks/fixtures";
import { useProfileEditor } from "../use-profile-editor";
import { createProfileEditorLogic } from "../profile-editor-store";

describe("useProfileEditor", () => {
  it("Should ignore an older submission completion after the attempt changes", () => {
    const store = createProfileEditorLogic().createStore();
    let snapshot = store.getInitialSnapshot();
    [snapshot] = store.transition(snapshot, {
      type: "opened",
      value: { task_id: "task_1" } as never,
    });
    [snapshot] = store.transition(snapshot, {
      type: "submitRequested",
      execute: vi.fn().mockResolvedValue(undefined),
      taskId: "task_1",
      updatedAt: "2026-07-25T12:00:00.000Z",
    });
    [snapshot] = store.transition(snapshot, { type: "submitSucceeded", attemptId: 0 });

    expect(snapshot.context.phase).toBe("submitting");
  });

  it("Should use the profile callback from the render that submits", async () => {
    const previousOnSetProfile = vi.fn().mockResolvedValue(undefined);
    const currentOnSetProfile = vi.fn().mockResolvedValue(undefined);
    const { result, rerender } = renderHook(
      ({ onSetProfile }) =>
        useProfileEditor({
          onSetProfile,
          profile: taskExecutionProfileFixture,
          taskId: "task_authoritative",
        }),
      { initialProps: { onSetProfile: previousOnSetProfile } }
    );
    act(() => result.current.setOpen(true));

    rerender({ onSetProfile: currentOnSetProfile });
    act(() => result.current.submit());

    await waitFor(() => expect(result.current.open).toBe(false));
    expect(previousOnSetProfile).not.toHaveBeenCalled();
    expect(currentOnSetProfile).toHaveBeenCalledOnce();
  });

  it("Should initialize typed editor state from the current profile and task id", () => {
    const { result } = renderHook(() =>
      useProfileEditor({
        onSetProfile: vi.fn(),
        profile: taskExecutionProfileFixture,
        taskId: "task_authoritative",
      })
    );

    act(() => result.current.setOpen(true));

    expect(result.current.open).toBe(true);
    expect(result.current.value).toEqual({
      ...taskExecutionProfileFixture,
      task_id: "task_authoritative",
    });
  });

  it("Should initialize a complete inheriting profile when no override exists", () => {
    const { result } = renderHook(() =>
      useProfileEditor({ onSetProfile: vi.fn(), profile: null, taskId: "task_new" })
    );

    act(() => result.current.setOpen(true));

    expect(result.current.value).toMatchObject({
      coordinator: { mode: "inherit" },
      participants: {},
      review: {},
      runtime: { mode: "default" },
      sandbox: { mode: "inherit" },
      task_id: "task_new",
      worker: { mode: "inherit" },
    });
  });

  it("Should submit the authoritative task id and close after success", async () => {
    const onSetProfile = vi.fn().mockResolvedValue(undefined);
    const { result } = renderHook(() =>
      useProfileEditor({
        onSetProfile,
        profile: taskExecutionProfileFixture,
        taskId: "task_authoritative",
      })
    );
    act(() => result.current.setOpen(true));

    act(() => {
      result.current.submit();
      result.current.submit();
    });
    await waitFor(() => expect(result.current.open).toBe(false));

    expect(onSetProfile).toHaveBeenCalledTimes(1);
    expect(onSetProfile).toHaveBeenCalledWith(
      expect.objectContaining({ task_id: "task_authoritative" })
    );
    expect(result.current.open).toBe(false);
  });

  it("Should keep the typed editor open when submission fails", async () => {
    const onSetProfile = vi.fn().mockRejectedValue(new Error("Profile rejected"));
    const { result } = renderHook(() =>
      useProfileEditor({
        onSetProfile,
        profile: taskExecutionProfileFixture,
        taskId: "task_authoritative",
      })
    );
    act(() => result.current.setOpen(true));

    act(() => result.current.submit());
    await waitFor(() => expect(result.current.error).toBe("Profile rejected"));

    expect(result.current.open).toBe(true);
    expect(result.current.value).not.toBeNull();
    expect(result.current.error).toBe("Profile rejected");
  });
});
