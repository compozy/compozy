// Suite: derive landing
// Invariant: one derive outcome lands exactly once — New window through the host's `userOpen`
// path (which focuses a window the child already owns), This window through the host's in-place
// retarget, never both; a replayed outcome opens the child it already created; a deleted child
// opens nothing.
// Owning layer: `landDerivedSession`, shared by the Continue and Fork dialogs.
import { describe, expect, it, vi } from "vitest";

import { continuedSessionFixture, deriveResultFixture } from "../../mocks/derive-fixtures";
import type { SessionPayload } from "../../types";
import { landDerivedSession } from "../use-session-derive";

function handlers(withThisWindow = true) {
  return {
    openInNewWindow: vi.fn<(child: SessionPayload) => void>(),
    ...(withThisWindow ? { openInThisWindow: vi.fn<(child: SessionPayload) => void>() } : {}),
  };
}

describe("landDerivedSession", () => {
  const child = continuedSessionFixture();

  it("Should open the child in a new window and leave this window alone", () => {
    const placement = handlers();

    expect(landDerivedSession(deriveResultFixture(child), "new-window", placement)).toBe("opened");
    expect(placement.openInNewWindow).toHaveBeenCalledExactlyOnceWith(child);
    expect(placement.openInThisWindow).not.toHaveBeenCalled();
  });

  it("Should retarget this window without opening another", () => {
    const placement = handlers();

    expect(landDerivedSession(deriveResultFixture(child), "this-window", placement)).toBe("opened");
    expect(placement.openInThisWindow).toHaveBeenCalledExactlyOnceWith(child);
    expect(placement.openInNewWindow).not.toHaveBeenCalled();
  });

  it("Should fall back to a new window where the host has no window of its own", () => {
    const placement = handlers(false);

    landDerivedSession(deriveResultFixture(child), "this-window", placement);
    expect(placement.openInNewWindow).toHaveBeenCalledExactlyOnceWith(child);
  });

  it("Should open the existing child for a replayed outcome", () => {
    const placement = handlers();
    const replayed = deriveResultFixture(child, { replayed: true });

    expect(landDerivedSession(replayed, "new-window", placement)).toBe("opened");
    expect(placement.openInNewWindow).toHaveBeenCalledExactlyOnceWith(child);
  });

  it("Should open nothing once the recorded child was deleted", () => {
    const placement = handlers();
    const { derived } = deriveResultFixture(child, { replayed: true, child_deleted: true });

    expect(landDerivedSession({ derived }, "new-window", placement)).toBe("child_deleted");
    expect(placement.openInNewWindow).not.toHaveBeenCalled();
    expect(placement.openInThisWindow).not.toHaveBeenCalled();
  });
});
