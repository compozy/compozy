import { act, renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { useTaskFanOutDialog } from "../use-task-fan-out-dialog";

const submitEvent = { preventDefault: vi.fn() } as never;

describe("useTaskFanOutDialog", () => {
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
  });

  it("Should associate an empty submission with a correction message", async () => {
    const { result } = renderHook(() =>
      useTaskFanOutDialog({ onOpenChange: vi.fn(), onFanOut: vi.fn() })
    );

    await act(async () => result.current.handleSubmit(submitEvent));

    expect(result.current.formError).toBe("Add at least one assignment.");
  });
});
