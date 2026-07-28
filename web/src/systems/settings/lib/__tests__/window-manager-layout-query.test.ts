// Suite: window-manager layout Query projection
// Invariant: Settings observes the exact daemon snapshot cache updated by the runtime stream;
// its document and CAS revision are projected atomically from that one cache value.
// Owning layer: Settings Query options. Boundary OUT: stream transport and editor rendering.
import { describe, expect, it } from "vitest";

import { windowManagerSnapshotFixture } from "@/systems/os/mocks";
import { windowManagerKeys } from "@/systems/os";
import { parseWindowManagerSnapshot } from "@/systems/os/lib/window-manager-schemas";

import { windowManagerLayoutOptions } from "../window-manager-layout-query";

describe("windowManagerLayoutOptions", () => {
  it("Should select layout state from the runtime snapshot query key", () => {
    const snapshot = parseWindowManagerSnapshot(windowManagerSnapshotFixture);
    const options = windowManagerLayoutOptions(snapshot.workspaceId);

    expect(options.queryKey).toEqual(windowManagerKeys.snapshot(snapshot.workspaceId));
    expect(options.select?.(snapshot)).toMatchObject({
      revision: snapshot.revision,
      document: {
        version: snapshot.version,
        workspaceId: snapshot.workspaceId,
      },
    });
  });
});
