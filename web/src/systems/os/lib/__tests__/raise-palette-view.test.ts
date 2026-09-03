// Suite: palette view raise
// Invariant: a window can raise one palette view through the shell subscriber,
// and unsubscribing stops further raises.
// Owning layer: unit (raise-palette-view pub/sub).
// Boundary OUT: overlay open and view-stack push (desktop-shell-body).
import { describe, expect, it, vi } from "vitest";

import { raisePaletteView, subscribePaletteViewRaise } from "../raise-palette-view";

describe("raise palette view", () => {
  it("Should deliver a raise to current subscribers only", () => {
    const first = vi.fn();
    const second = vi.fn();
    const stopFirst = subscribePaletteViewRaise(first);
    const stopSecond = subscribePaletteViewRaise(second);

    raisePaletteView("sessions");
    stopFirst();
    raisePaletteView("commands");
    stopSecond();

    expect(first).toHaveBeenCalledExactlyOnceWith("sessions");
    expect(second).toHaveBeenCalledTimes(2);
    expect(second).toHaveBeenNthCalledWith(1, "sessions");
    expect(second).toHaveBeenNthCalledWith(2, "commands");
  });
});
