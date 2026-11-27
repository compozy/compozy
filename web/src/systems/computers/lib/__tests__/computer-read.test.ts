// Suite: computer read-state mapper (T-F4)
// Invariant: registry status + facets query status + cache presence map deterministically
// to one ComputerRead state; failed/pending/offline_empty computers are never counted as 0
// and appear in `Not counted`; needs_pro is excluded from `Not counted`.
// Owning layer: unit (systems/computers/lib)
import { describe, expect, it } from "vitest";

import {
  computerCanOperateSessions,
  computerReadDetail,
  isComputerCounted,
  isComputerUncounted,
  mapComputerRead,
  uncountedComputerNames,
} from "../computer-read";

describe("mapComputerRead (T-F4)", () => {
  it("Should map registry and query statuses into the canonical ComputerRead states", () => {
    expect(mapComputerRead({ status: "connected", queryStatus: "success" })).toBe("ok");
    expect(mapComputerRead({ status: "connected", queryStatus: "pending" })).toBe("connecting");
    expect(mapComputerRead({ status: "connected", queryStatus: "error" })).toBe("failed");
    expect(mapComputerRead({ status: "connecting", queryStatus: "pending" })).toBe("connecting");
    expect(mapComputerRead({ status: "offline", hasCachedRows: true })).toBe("offline");
    expect(mapComputerRead({ status: "offline", hasCachedRows: false })).toBe("offline_empty");
    expect(mapComputerRead({ status: "needs_pro" })).toBe("needs_pro");
    expect(mapComputerRead({ status: "incompatible" })).toBe("incompatible");
    expect(mapComputerRead({ status: "needs_attention" })).toBe("incompatible");
    expect(mapComputerRead({ status: "connected", queryStatus: "pending", isLocal: true })).toBe(
      "ok"
    );
  });

  it("Should mark only ok and offline as counted and list unanswered computers in Not counted", () => {
    expect(isComputerCounted("ok")).toBe(true);
    expect(isComputerCounted("offline")).toBe(true);
    expect(isComputerCounted("offline_empty")).toBe(false);
    expect(isComputerCounted("connecting")).toBe(false);
    expect(isComputerCounted("failed")).toBe(false);
    expect(isComputerCounted("needs_pro")).toBe(false);
    expect(isComputerCounted("incompatible")).toBe(false);

    expect(isComputerUncounted("needs_pro")).toBe(false);
    expect(
      uncountedComputerNames([
        { name: "MacBook Pro", read: "ok" },
        { name: "devbox", read: "failed" },
        { name: "homelab", read: "offline_empty" },
        { name: "hetzner", read: "connecting" },
        { name: "mini-build", read: "needs_pro" },
        { name: "gpu-vm", read: "incompatible" },
        { name: "workstation", read: "offline" },
      ])
    ).toEqual(["devbox", "homelab", "hetzner", "gpu-vm"]);
  });

  it("Should format header detail strings and check sessions:operate scope", () => {
    expect(computerReadDetail(null, "ok", true)).toBe("This computer");
    expect(computerReadDetail({ routeInUse: null, lastConnectedLabel: "2h ago" }, "offline")).toBe(
      "Offline · 2h ago"
    );
    expect(computerReadDetail(null, "connecting")).toBe("Connecting…");
    expect(computerReadDetail(null, "needs_pro")).toBe("Needs Pro");
    expect(computerReadDetail(null, "incompatible")).toBe("Update needed");
    expect(
      computerReadDetail(
        {
          routeInUse: {
            id: "r1",
            kind: "direct",
            address: "10.0.0.2",
            label: "via LAN 10.0.0.2",
            inUse: true,
            learned: false,
            lastError: null,
          },
          lastConnectedLabel: "just now",
        },
        "ok"
      )
    ).toBe("via LAN 10.0.0.2");

    expect(computerCanOperateSessions(undefined, true)).toBe(true);
    expect(computerCanOperateSessions(["sessions:operate"], false)).toBe(true);
    expect(computerCanOperateSessions(["files:read"], false)).toBe(false);
  });
});
