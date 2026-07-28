import { describe, expect, it } from "vitest";

import {
  isSelectedLayoutProfile,
  layoutProfileResourceKey,
} from "../window-manager-layout-profile-key";
import type { WindowManagerLayoutResourceRecord } from "../window-manager-layout-types";

function record(scope: "global" | "workspace"): WindowManagerLayoutResourceRecord {
  return {
    kind: "window_layout",
    id: "two-up-review",
    version: 1,
    scope: { kind: scope, id: scope === "workspace" ? "workspace-a" : "" },
    spec: {
      version: 1,
      id: "two-up-review",
      displayName: "Two-up review",
      aspectVariant: "any",
      participantSlots: [],
      overflowPolicy: "stack",
      document: {
        version: 2,
        workspaceId: "workspace-a",
        desktops: [],
        windows: {},
        overrides: {},
      },
    },
    createdAt: "2026-07-25T00:00:00Z",
    updatedAt: "2026-07-25T00:00:00Z",
  };
}

describe("layoutProfileResourceKey", () => {
  it("Should keep workspace overrides distinct from a global layout with the same id", () => {
    const global = record("global");
    const workspace = record("workspace");

    expect(layoutProfileResourceKey(global)).not.toBe(layoutProfileResourceKey(workspace));
    expect(isSelectedLayoutProfile(layoutProfileResourceKey(workspace), global)).toBe(false);
    expect(isSelectedLayoutProfile(layoutProfileResourceKey(workspace), workspace)).toBe(true);
  });
});
