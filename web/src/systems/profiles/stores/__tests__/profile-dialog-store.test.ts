// Suite: canonical profile lifecycle-dialog intent store
// Invariant: one intent replaces the previous intent, close clears it, and closing an idle store is a no-op.
// Owning layer: web/src/systems/profiles/stores/profile-dialog-store.ts.
import { beforeEach, describe, expect, it } from "vitest";

import { closeProfileDialog, openProfileDialog, profileDialogStore } from "../profile-dialog-store";

describe("profileDialogStore", () => {
  beforeEach(() => closeProfileDialog());

  it("Should replace the current lifecycle intent when another dialog opens", () => {
    openProfileDialog({ flow: "create", profile: "growth" });
    openProfileDialog({ flow: "archive", profile: "marketing" });

    expect(profileDialogStore.getSnapshot().context.intent).toEqual({
      flow: "archive",
      profile: "marketing",
    });
  });

  it("Should clear the current lifecycle intent when the dialog closes", () => {
    openProfileDialog({ flow: "rename", profile: "marketing" });

    closeProfileDialog();

    expect(profileDialogStore.getSnapshot().context.intent).toBeNull();
  });

  it("Should preserve the idle snapshot when close is repeated", () => {
    const idle = profileDialogStore.getSnapshot();

    closeProfileDialog();

    expect(profileDialogStore.getSnapshot()).toBe(idle);
  });
});
