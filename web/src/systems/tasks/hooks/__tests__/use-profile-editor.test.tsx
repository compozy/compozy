import { act, renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { taskExecutionProfileFixture } from "../../mocks/fixtures";
import { useProfileEditor } from "../use-profile-editor";

describe("useProfileEditor", () => {
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

    await act(async () => result.current.submit());

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

    await act(async () => result.current.submit());

    expect(result.current.open).toBe(true);
    expect(result.current.value).not.toBeNull();
  });
});
