import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { createMarketplaceInstalledCardConfirmLogic } from "../marketplace-installed-card-confirm-store";
import { useMarketplaceInstalledCardConfirmation } from "../use-marketplace-installed-card-confirmation";

describe("marketplace installed card confirmation store", () => {
  it("Should retain the active confirmation when an obsolete completion arrives", () => {
    const logic = createMarketplaceInstalledCardConfirmLogic();
    const store = logic.createStore();
    let snapshot = store.getInitialSnapshot();

    [snapshot] = store.transition(snapshot, { type: "confirmationOpened", action: "remove" });
    [snapshot] = store.transition(snapshot, {
      type: "confirmationRequested",
      execute: vi.fn(),
    });
    const requestId = snapshot.context.requestId;
    [snapshot] = store.transition(snapshot, { type: "confirmationOpened", action: "deactivate" });
    [snapshot] = store.transition(snapshot, { type: "confirmationSucceeded", requestId });

    expect(snapshot.context).toMatchObject({ phase: "confirming", action: "deactivate" });
  });

  it("Should execute the callback from the render that confirms", async () => {
    const previousExecute = vi.fn().mockResolvedValue(undefined);
    const currentExecute = vi.fn().mockResolvedValue(undefined);
    const { result, rerender } = renderHook(
      ({ execute }) => useMarketplaceInstalledCardConfirmation(execute),
      { initialProps: { execute: previousExecute } }
    );
    act(() => result.current.openConfirmation("remove"));

    rerender({ execute: currentExecute });
    act(() => result.current.confirm());

    await waitFor(() => expect(result.current.open).toBe(false));
    expect(previousExecute).not.toHaveBeenCalled();
    expect(currentExecute).toHaveBeenCalledWith("remove");
  });
});
