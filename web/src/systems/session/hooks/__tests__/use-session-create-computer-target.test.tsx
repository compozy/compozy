// Suite: View options folder/manual rules (T-F9) and fixed computer creation target (T-F10)
// Invariant: Multi-computer views disable toolbar New folder with the canonical message (Q-G)
//            and update the manual-sort note (O3); opening session creation with computerId and
//            fixedComputer fixes Run on to that computer instead of Auto (R14, R15).
// Owning layer: unit (systems/session/lib & stores)
import { describe, expect, it } from "vitest";

import {
  manualSortNote,
  newFolderUnavailableReason,
  SESSION_LIST_MANUAL_SORT_MULTI_COMPUTER_NOTE,
  SESSION_LIST_MANUAL_SORT_NOTE,
  SESSION_LIST_NEW_FOLDER_MULTI_COMPUTER_NOTE,
} from "../../lib/session-list-view-helpers";
import { createSessionCreateStore } from "../../stores/session-create-store";

describe("View options, toolbar folder & manual-sort rules (T-F9)", () => {
  it("Should disable toolbar New folder with exact prototype copy while multiple computers are shown", () => {
    expect(
      newFolderUnavailableReason(
        false,
        false,
        { allProjects: true, allProfiles: false },
        "project",
        true
      )
    ).toBe(SESSION_LIST_NEW_FOLDER_MULTI_COMPUTER_NOTE);
    expect(SESSION_LIST_NEW_FOLDER_MULTI_COMPUTER_NOTE).toBe(
      "Folders live on one computer. Use the + of a computer or project."
    );
  });

  it("Should allow toolbar New folder under one-computer scope using single-computer rules", () => {
    expect(
      newFolderUnavailableReason(
        false,
        false,
        { allProjects: true, allProfiles: false },
        "project",
        false
      )
    ).toBeNull();
    expect(
      newFolderUnavailableReason(
        true,
        true,
        { allProjects: false, allProfiles: false },
        "none",
        false
      )
    ).toBeNull();
  });

  it("Should name one computer in the manual-sort note when multiple computers are visible (O3)", () => {
    expect(manualSortNote("manual", true, false)).toBe(SESSION_LIST_MANUAL_SORT_NOTE);
    expect(manualSortNote("manual", true, true)).toBe(SESSION_LIST_MANUAL_SORT_MULTI_COMPUTER_NOTE);
    expect(SESSION_LIST_MANUAL_SORT_MULTI_COMPUTER_NOTE).toBe(
      "Manual order applies inside one project and profile on one computer."
    );
  });
});

describe("Creation target computer preselection (T-F10)", () => {
  it("Should store computerId, fixedComputer, project and profile when opened from a computer/project +", () => {
    const store = createSessionCreateStore();
    store.trigger.dialogOpened({
      agentName: "claude",
      workspaceId: "",
      computerId: "cmp_devbox",
      fixedComputer: true,
      target: { project: "ws_remote_1", profile: "default" },
    });

    const ctx = store.getSnapshot().context;
    expect(ctx.open).toBe(true);
    expect(ctx.draft.computerId).toBe("cmp_devbox");
    expect(ctx.fixedComputer).toBe(true);
    expect(ctx.target).toEqual({ project: "ws_remote_1", profile: "default" });

    // Switching computer manually clears fixedComputer so Auto becomes available again.
    store.trigger.computerSelected({ computerId: "auto" });
    expect(store.getSnapshot().context.fixedComputer).toBe(false);
  });
});
