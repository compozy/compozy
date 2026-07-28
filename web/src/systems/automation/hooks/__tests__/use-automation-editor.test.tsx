import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const createAutomationJobMock = vi.fn();
const updateAutomationJobMock = vi.fn();
const createAutomationTriggerMock = vi.fn();
const updateAutomationTriggerMock = vi.fn();

vi.mock("@/systems/agent", () => ({
  useAgents: () => ({ data: [], error: null, isLoading: false }),
}));

vi.mock("../use-automation-actions", () => ({
  useCreateAutomationJob: () => ({ mutateAsync: createAutomationJobMock }),
  useUpdateAutomationJob: () => ({ mutateAsync: updateAutomationJobMock }),
  useCreateAutomationTrigger: () => ({ mutateAsync: createAutomationTriggerMock }),
  useUpdateAutomationTrigger: () => ({ mutateAsync: updateAutomationTriggerMock }),
}));

vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

import { useAutomationJobEditor } from "../use-automation-editor";

function deferred<T>() {
  let resolve: (value: T) => void;
  const promise = new Promise<T>(resolvePromise => {
    resolve = resolvePromise;
  });
  return { promise, resolve: (value: T) => resolve(value) };
}

describe("useAutomationJobEditor", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("clears the resource-bound editor when the active workspace changes", () => {
    const { result, rerender } = renderHook(
      ({ workspaceId }: { workspaceId: string | null }) =>
        useAutomationJobEditor({ activeWorkspaceId: workspaceId }),
      { initialProps: { workspaceId: "ws_alpha" } }
    );

    act(() => result.current.openCreate());
    expect(result.current.editor).toMatchObject({ mode: "create" });

    rerender({ workspaceId: "ws_beta" });

    expect(result.current.editor).toBeNull();
  });

  it("keeps a newly opened editor when an older submission resolves", async () => {
    const pending = deferred<{ id: string; name: string }>();
    createAutomationJobMock.mockReturnValue(pending.promise);
    const { result } = renderHook(() => useAutomationJobEditor({ activeWorkspaceId: "ws_alpha" }));

    act(() => result.current.openCreate());
    act(() => result.current.editorDialogProps.editor?.onSubmit());
    act(() => {
      result.current.close();
      result.current.openCreate();
    });

    await act(async () => {
      pending.resolve({ id: "job_old", name: "old" });
      await pending.promise;
    });

    await waitFor(() => expect(result.current.editor).toMatchObject({ mode: "create" }));
  });
});
