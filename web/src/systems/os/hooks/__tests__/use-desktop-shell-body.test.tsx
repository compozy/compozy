// Suite: desktop shell palette handoff
// Invariant: a successful new-tab open forwards the focused window as
// stackTargetWindowId, closes the command palette, and publishes the returned
// window as destination intent; invoke identity is the registered client.
// Owning layer: use-desktop-shell-body.ts. Canonical suite: this file.
import { afterEach, describe, expect, it, vi } from "vitest";

import { windowManagerStore } from "../../stores/window-manager-store";
import { completePaletteNewTabOpen, paletteInvokeIdentity } from "../use-desktop-shell-body";

afterEach(() => {
  windowManagerStore.trigger.paletteIntentCleared();
});

describe("desktop shell palette new-tab handoff", () => {
  it("Should forward the focused window, close the palette, and publish destination [RA0288]", async () => {
    const closePalette = vi.fn();
    const userOpen = vi.fn(async () => "window:new-tab");

    await completePaletteNewTabOpen({
      closePalette,
      stackTargetWindowId: "w-focused",
      userOpen,
    });

    expect(closePalette).toHaveBeenCalledOnce();
    expect(userOpen).toHaveBeenCalledExactlyOnceWith({
      app: "new-tab",
      stackTargetWindowId: "w-focused",
    });
    expect(windowManagerStore.getSnapshot().context.paletteIntent).toEqual({
      kind: "destination",
      windowId: "window:new-tab",
    });
  });

  it("Should omit stackTargetWindowId and skip destination when open returns null [RA0288]", async () => {
    const closePalette = vi.fn();
    const userOpen = vi.fn(async () => null);

    await completePaletteNewTabOpen({
      closePalette,
      stackTargetWindowId: null,
      userOpen,
    });

    expect(closePalette).toHaveBeenCalledOnce();
    expect(userOpen).toHaveBeenCalledExactlyOnceWith({ app: "new-tab" });
    expect(windowManagerStore.getSnapshot().context.paletteIntent).toBeNull();
  });
});

describe("desktop shell palette invoke identity", () => {
  it("Should thread registered client id and token together [RD0082]", () => {
    expect(
      paletteInvokeIdentity({
        clientId: "client-web",
        attachmentToken: "attachment-token",
      })
    ).toEqual({
      clientId: "client-web",
      attachmentToken: "attachment-token",
    });
    expect(paletteInvokeIdentity(null)).toEqual({});
  });
});
