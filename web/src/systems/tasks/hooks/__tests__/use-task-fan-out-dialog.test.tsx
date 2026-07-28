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
      setOpen,
    });
    [snapshot] = store.transition(snapshot, {
      type: "submitSucceeded",
      attemptId: 0,
      setOpen,
    });

    expect(snapshot.context.phase).toBe("submitting");
    expect(setOpen).not.toHaveBeenCalled();
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
