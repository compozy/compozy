// Suite: attention notifier activation target
// Invariant: an agent notification keeps its session and workspace identity
// even before the attention catalog resolves the sending session.
// A layout upgrade is offered to the toast once per occurrence per client, across reloads.
// Owning layer: OS notifier. No prior suite owned this delivery-to-jump mapping.
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { OsAttentionRow, OsLayoutUpgradeAttentionRow } from "../../lib/attention-model";
import { takeUntoldLayoutUpgrades } from "../../lib/layout-upgrade-toasts";
import { attentionJumpTargetFor, planAttentionToasts } from "../use-attention-notifier";

function needsYouDelivery(id: string) {
  return {
    kind: "needs-you" as const,
    key: `needs-you:${id}`,
    target: {
      id,
      title: id,
      agentName: "claude",
      workspaceId: "ws-alpha",
      workspaceLabel: "alpha",
      badge: "waiting-for-input" as const,
      reason: "Question pending",
    },
  };
}

describe("attentionJumpTargetFor", () => {
  it("Should preserve an unresolved agent notification's session identity", () => {
    expect(
      attentionJumpTargetFor({
        kind: "agent",
        key: "agent:ntf-1",
        notification: {
          profile_id: "00000000000000000000000000",
          profile_name: "default",
          notification_id: "ntf-1",
          session_id: "sess-alpha",
          workspace_id: "ws-alpha",
          title: "Review ready",
          body: "",
          at: "2026-07-13T12:05:00Z",
        },
        target: null,
      })
    ).toEqual({ sessionId: "sess-alpha", workspaceId: "ws-alpha" });
  });

  it("Should keep the four newest needs-you toasts and fold older ones into a ledge", () => {
    const plan = planAttentionToasts(
      ["one", "two", "three", "four", "five", "six"].map(needsYouDelivery)
    );

    expect(plan.deliveries.map(delivery => delivery.key)).toEqual([
      "needs-you:three",
      "needs-you:four",
      "needs-you:five",
      "needs-you:six",
    ]);
    expect(plan.overflowNeedsYou).toBe(2);
  });
});

function layoutUpgrade(
  occurrence: string,
  overrides: Partial<OsLayoutUpgradeAttentionRow> = {}
): OsLayoutUpgradeAttentionRow {
  return {
    kind: "layout-upgrade",
    id: "ws-atlas",
    notificationId: occurrence,
    title: "Your windows are now tabs",
    workspaceId: "ws-atlas",
    workspaceLabel: "atlas",
    reason: "4 windows became 2 tabs.",
    occurredAt: "2026-11-13T09:00:00Z",
    stale: false,
    ...overrides,
  };
}

describe("takeUntoldLayoutUpgrades", () => {
  beforeEach(() => localStorage.clear());

  it("Should offer each occurrence once, also to a fresh page after a reload", () => {
    const first = layoutUpgrade("occ-1");
    const task: OsAttentionRow = { kind: "task", id: "task-1", title: "Task", reason: "" };

    expect(takeUntoldLayoutUpgrades([task, first], new Set())).toEqual([first]);
    expect(takeUntoldLayoutUpgrades([first], new Set())).toEqual([]);

    const second = layoutUpgrade("occ-2");
    expect(takeUntoldLayoutUpgrades([first, second], new Set())).toEqual([second]);
  });

  it("Should not count a row from a stale list as told", () => {
    const row = layoutUpgrade("occ-1");

    expect(takeUntoldLayoutUpgrades([{ ...row, stale: true }], new Set())).toEqual([]);
    expect(takeUntoldLayoutUpgrades([row], new Set())).toEqual([row]);
  });

  it("Should never forget an occurrence the list still holds, and drop the ones that left it", () => {
    const listed = Array.from({ length: 70 }, (_, index) => layoutUpgrade(`occ-${index}`));
    expect(takeUntoldLayoutUpgrades(listed, new Set())).toHaveLength(70);

    // A reload with more unread upgrade items than any fixed cap: none toasts again.
    expect(takeUntoldLayoutUpgrades(listed, new Set())).toEqual([]);

    // The first was read and left the list; a new one arrives.
    const next = layoutUpgrade("occ-70");
    expect(takeUntoldLayoutUpgrades([...listed.slice(1), next], new Set())).toEqual([next]);
    const stored: string[] = JSON.parse(localStorage.getItem("compozy.layout-upgrade.toasted")!);
    expect(stored).toHaveLength(70);
    expect(stored).not.toContain("occ-0");
    expect(stored).toContain("occ-70");
  });

  it("Should read storage only for an occurrence the page has not seen", () => {
    const told = new Set<string>();
    const row = layoutUpgrade("occ-1");
    expect(takeUntoldLayoutUpgrades([row], told)).toEqual([row]);
    const read = vi.spyOn(Storage.prototype, "getItem");

    // The notifier re-renders with the same list.
    expect(takeUntoldLayoutUpgrades([row], told)).toEqual([]);
    expect(read).not.toHaveBeenCalled();

    const next = layoutUpgrade("occ-2");
    expect(takeUntoldLayoutUpgrades([row, next], told)).toEqual([next]);
    expect(read).toHaveBeenCalledOnce();
    read.mockRestore();
  });

  it("Should keep telling once per page when storage is unreadable", () => {
    localStorage.setItem("compozy.layout-upgrade.toasted", "{not json");
    const told = new Set<string>();
    const row = layoutUpgrade("occ-1");

    expect(takeUntoldLayoutUpgrades([row], told)).toEqual([row]);
    expect(takeUntoldLayoutUpgrades([row], told)).toEqual([]);
  });
});
