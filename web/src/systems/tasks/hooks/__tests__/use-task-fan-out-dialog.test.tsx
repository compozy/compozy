import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { useTaskFanOutDialog } from "../use-task-fan-out-dialog";
import { createTaskFanOutDialogLogic } from "../task-fan-out-dialog-store";

const submitEvent = { preventDefault: vi.fn() } as never;

describe("useTaskFanOutDialog", () => {
  it("Should keep an older submission result from closing a newer attempt", () => {
    const setOpen = vi.fn();
    const store = createTaskFanOutDialogLogic().createStore();
    let snapshot = store.getInitialSnapshot();
    [snapshot] = store.transition(snapshot, {
      type: "designationsChanged",
      value: "Investigate checkout",
    });
    [snapshot] = store.transition(snapshot, {
      type: "submitRequested",
      execute: vi.fn().mockResolvedValue(undefined),
      idempotencyKey: "fan-out-attempt-1",
      setOpen,
    });
    [snapshot] = store.transition(snapshot, {
      type: "submitSucceeded",
      attemptId: 0,
      result: null,
      setOpen,
    });

    expect(snapshot.context.phase).toBe("submitting");
    expect(setOpen).not.toHaveBeenCalled();
  });

  /**
   * Per-run worktree attribution exists only in the fan-out response, so an
   * isolated fan-out holds the dialog open to report it; a shared-root fan-out
   * keeps closing on success as before.
   */
  it("Should hold the dialog open to report per-run attribution after an isolated fan-out", () => {
    const setOpen = vi.fn();
    const store = createTaskFanOutDialogLogic().createStore();
    const response = {
      designation_group_id: "dg_1",
      runs: [{ id: "run_1", resolved_worktree_mode: "per_run" as const }],
    } as never;
    let snapshot = store.getInitialSnapshot();
    [snapshot] = store.transition(snapshot, {
      type: "designationsChanged",
      value: "Investigate checkout",
    });
    [snapshot] = store.transition(snapshot, { type: "worktreePerRunChanged", value: true });
    [snapshot] = store.transition(snapshot, {
      type: "submitRequested",
      execute: vi.fn().mockResolvedValue(response),
      idempotencyKey: "fan-out-attempt-1",
      setOpen,
    });
    [snapshot] = store.transition(snapshot, {
      type: "submitSucceeded",
      attemptId: snapshot.context.attemptId,
      result: response,
      setOpen,
    });

    expect(setOpen).not.toHaveBeenCalled();
    expect(snapshot.context.result).toBe(response);
    expect(snapshot.context.phase).toBe("editing");
  });

  /**
   * The daemon's default is shared-root execution, so an explicit `false` would
   * read back as a policy the operator never set. The flag rides only when asked.
   */
  it("Should send worktree_per_run only when isolation is requested", () => {
    function submitWith(isolate: boolean) {
      const store = createTaskFanOutDialogLogic().createStore();
      const execute = vi.fn().mockResolvedValue(undefined);
      store.trigger.designationsChanged({ value: "One\nTwo" });
      if (isolate) store.trigger.worktreePerRunChanged({ value: true });
      store.trigger.submitRequested({
        execute,
        idempotencyKey: "fan-out-attempt-1",
        setOpen: vi.fn(),
      });
      return execute.mock.calls[0][0] as Record<string, unknown>;
    }

    const shared = submitWith(false);
    expect(shared).toMatchObject({ designations: [{ brief: "One" }, { brief: "Two" }] });
    expect(shared).not.toHaveProperty("worktree_per_run");
    expect(submitWith(true)).toMatchObject({ worktree_per_run: true });
  });

  it("Should use the callbacks from the render that submits", async () => {
    const previousFanOut = vi.fn().mockResolvedValue(undefined);
    const previousOpenChange = vi.fn();
    const currentFanOut = vi.fn().mockResolvedValue(undefined);
    const currentOpenChange = vi.fn();
    const { result, rerender } = renderHook(props => useTaskFanOutDialog(props), {
      initialProps: {
        onFanOut: previousFanOut,
        onOpenChange: previousOpenChange,
      },
    });
    act(() => result.current.setDesignationsText("Investigate checkout"));

    rerender({ onFanOut: currentFanOut, onOpenChange: currentOpenChange });
    act(() => result.current.handleSubmit(submitEvent));

    await waitFor(() => expect(currentOpenChange).toHaveBeenCalledWith(false));
    expect(previousFanOut).not.toHaveBeenCalled();
    expect(previousOpenChange).not.toHaveBeenCalled();
    expect(currentFanOut).toHaveBeenCalledOnce();
  });

  it("Should submit one trimmed designation per non-empty line and reset on success", async () => {
    const onFanOut = vi.fn().mockResolvedValue(undefined);
    const onOpenChange = vi.fn();
    const { result } = renderHook(() => useTaskFanOutDialog({ onOpenChange, onFanOut }));

    act(() => result.current.setDesignationsText(" Investigate checkout \n\n Validate staging "));
    await act(async () => result.current.handleSubmit(submitEvent));

    expect(onFanOut).toHaveBeenCalledWith({
      designations: [{ brief: "Investigate checkout" }, { brief: "Validate staging" }],
      idempotency_key: expect.any(String),
      network_participation: { mode: "local" },
    });
    expect(onOpenChange).toHaveBeenCalledWith(false);
    expect(result.current.designationsText).toBe("");
  });

  it("Should keep the dialog state available when the runtime rejects fan-out", async () => {
    const onFanOut = vi.fn().mockRejectedValue(new Error("Task already has an open run"));
    const onOpenChange = vi.fn();
    const { result } = renderHook(() => useTaskFanOutDialog({ onOpenChange, onFanOut }));

    act(() => result.current.setDesignationsText("Investigate checkout"));
    await act(async () => result.current.handleSubmit(submitEvent));

    expect(onOpenChange).not.toHaveBeenCalledWith(false);
    expect(result.current.designationsText).toBe("Investigate checkout");
    expect(result.current.formError).toBe("Task already has an open run");
  });

  it("Should retry one unchanged intent with the same idempotency key", async () => {
    const onFanOut = vi.fn().mockRejectedValue(new Error("Temporary failure"));
    const { result } = renderHook(() => useTaskFanOutDialog({ onOpenChange: vi.fn(), onFanOut }));

    act(() => result.current.setDesignationsText("Investigate checkout"));
    await act(async () => result.current.handleSubmit(submitEvent));
    await act(async () => result.current.handleSubmit(submitEvent));

    const firstKey = onFanOut.mock.calls[0][0].idempotency_key;
    expect(firstKey).toEqual(expect.any(String));
    expect(onFanOut.mock.calls[1][0].idempotency_key).toBe(firstKey);

    act(() => result.current.setDesignationsText("Validate staging"));
    await act(async () => result.current.handleSubmit(submitEvent));
    expect(onFanOut.mock.calls[2][0].idempotency_key).not.toBe(firstKey);
  });

  it("Should execute only one fan-out for two synchronous submissions", async () => {
    let resolveFanOut: (() => void) | undefined;
    const onFanOut = vi.fn(
      () =>
        new Promise<void>(resolve => {
          resolveFanOut = resolve;
        })
    );
    const { result } = renderHook(() => useTaskFanOutDialog({ onOpenChange: vi.fn(), onFanOut }));
    act(() => result.current.setDesignationsText("Investigate checkout"));

    act(() => {
      result.current.handleSubmit(submitEvent);
      result.current.handleSubmit(submitEvent);
    });

    expect(onFanOut).toHaveBeenCalledTimes(1);
    await act(async () => resolveFanOut?.());
  });

  it("Should associate an empty submission with a correction message", async () => {
    const { result } = renderHook(() =>
      useTaskFanOutDialog({ onOpenChange: vi.fn(), onFanOut: vi.fn() })
    );

    await act(async () => result.current.handleSubmit(submitEvent));

    expect(result.current.formError).toBe("Add at least one assignment.");
  });
});
