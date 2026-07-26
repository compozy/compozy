// Suite: browser gesture cancellation bridge
// Invariant: every browser terminal signal cancels the active layout gesture exactly once.
// Owning layer: browser event ↔ semantic gesture boundary. Boundary OUT: Zustand gesture state.
import { describe, expect, it, vi } from "vitest";

import { bindLayoutGestureCancellation } from "../layout-gesture-cancellation";

function eventWithProperties(type: string, properties: Record<string, unknown>): Event {
  const event = new Event(type, { cancelable: true });
  for (const [key, value] of Object.entries(properties)) {
    Object.defineProperty(event, key, { configurable: true, value });
  }
  return event;
}

describe("bindLayoutGestureCancellation", () => {
  it.each([
    {
      name: "pointer cancellation",
      event: () => eventWithProperties("pointercancel", { clientX: 17, clientY: 29 }),
      expected: ["pointer-cancel", { x: 17, y: 29 }],
    },
    {
      name: "touch cancellation",
      event: () =>
        eventWithProperties("touchcancel", {
          changedTouches: [{ clientX: 31, clientY: 43 }],
        }),
      expected: ["pointer-cancel", { x: 31, y: 43 }],
    },
    {
      name: "lost pointer capture",
      event: () => new Event("lostpointercapture"),
      expected: ["lost-capture"],
    },
    {
      name: "Escape",
      event: () => eventWithProperties("keydown", { key: "Escape" }),
      expected: ["escape"],
    },
  ])("Should cancel once on $name and detach cleanly", ({ event, expected }) => {
    const cancel = vi.fn();
    const unbind = bindLayoutGestureCancellation(window, cancel);

    window.dispatchEvent(event());

    expect(cancel).toHaveBeenCalledOnce();
    expect(cancel).toHaveBeenCalledWith(...expected);

    unbind();
    window.dispatchEvent(event());
    expect(cancel).toHaveBeenCalledOnce();
  });
});
