// Suite: attention notifier activation target
// Invariant: an agent notification keeps its session and workspace identity
// even before the attention catalog resolves the sending session.
// A layout upgrade is offered to the toast once per occurrence per client, across reloads.
// Owning layer: OS notifier. No prior suite owned this delivery-to-jump mapping.
import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { ReactElement } from "react";

import { toast } from "@compozy/ui";

import type { OsAttentionRow, OsLayoutUpgradeAttentionRow } from "../../lib/attention-model";
import {
  takeUntoldLayoutUpgrades,
  TOLD_LAYOUT_UPGRADES_CAP,
} from "../../lib/layout-upgrade-toasts";
import { attentionJumpTargetFor, planAttentionToasts } from "../use-attention-notifier";
import { useLayoutUpgradeToast, type LayoutUpgradeToastOptions } from "../use-layout-upgrade-toast";

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

  it("Should not toast again an occurrence that left the list and is listed again", () => {
    const listed = Array.from({ length: 70 }, (_, index) => layoutUpgrade(`occ-${index}`));
    expect(takeUntoldLayoutUpgrades(listed, new Set())).toHaveLength(70);

    // A reload with more unread upgrade items than the bell shows at once: none toasts again.
    expect(takeUntoldLayoutUpgrades(listed, new Set())).toEqual([]);

    // A list without the first occurrence records a new one; the first then comes back.
    const next = layoutUpgrade("occ-70");
    expect(takeUntoldLayoutUpgrades([...listed.slice(1), next], new Set())).toEqual([next]);
    expect(takeUntoldLayoutUpgrades([...listed, next], new Set())).toEqual([]);
  });

  it("Should bound the record by dropping unlisted occurrences before listed ones", () => {
    const old = Array.from({ length: TOLD_LAYOUT_UPGRADES_CAP }, (_, index) =>
      layoutUpgrade(`old-${index}`)
    );
    takeUntoldLayoutUpgrades(old, new Set());
    const listed = [old[0]!, layoutUpgrade("new-1")];

    expect(takeUntoldLayoutUpgrades(listed, new Set())).toEqual([listed[1]]);
    const stored: string[] = JSON.parse(localStorage.getItem("compozy.layout-upgrade.toasted")!);
    expect(stored).toHaveLength(TOLD_LAYOUT_UPGRADES_CAP);
    expect(stored).toContain("old-0");
    expect(stored).toContain("new-1");
    expect(stored).not.toContain("old-1");
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

// Invariant: an occurrence suppressed by an open bell, a muted workspace or toasts turned off counts
// as told and never toasts later; occurrences that arrive together share one toast.
describe("useLayoutUpgradeToast", () => {
  beforeEach(() => {
    localStorage.clear();
    vi.restoreAllMocks();
  });

  function render(
    finished: OsLayoutUpgradeAttentionRow[],
    options: Partial<Omit<LayoutUpgradeToastOptions, "sections">> = {}
  ) {
    const custom = vi.spyOn(toast, "custom").mockReturnValue("toast");
    const initial: LayoutUpgradeToastOptions = {
      sections: { needsYou: [], finished },
      bellOpen: false,
      toasts: true,
      mutedWorkspaceIds: new Set(),
      onOpenBell: () => undefined,
      ...options,
    };
    const hook = renderHook((props: LayoutUpgradeToastOptions) => useLayoutUpgradeToast(props), {
      initialProps: initial,
    });
    return { custom, hook, initial };
  }

  it.each([
    ["the bell is open", { bellOpen: true }],
    ["toasts are off", { toasts: false }],
    ["the workspace is muted", { mutedWorkspaceIds: new Set(["ws-atlas"]) }],
  ])("Should count an occurrence as told without a toast when %s", (_, suppression) => {
    const row = layoutUpgrade("occ-1");
    const { custom, hook, initial } = render([row], suppression);
    expect(custom).not.toHaveBeenCalled();

    hook.rerender({
      ...initial,
      bellOpen: false,
      toasts: true,
      mutedWorkspaceIds: new Set(),
      sections: { needsYou: [], finished: [{ ...row }] },
    });
    expect(custom).not.toHaveBeenCalled();
  });

  it("Should raise one toast for occurrences that arrive together, skipping muted ones", () => {
    const rows = [
      layoutUpgrade("occ-1"),
      layoutUpgrade("occ-2", { id: "ws-beta", workspaceId: "ws-beta", workspaceLabel: "beta" }),
      layoutUpgrade("occ-3", { id: "ws-gamma", workspaceId: "ws-gamma", workspaceLabel: "gamma" }),
    ];
    const { custom } = render(rows, { mutedWorkspaceIds: new Set(["ws-gamma"]) });

    expect(custom).toHaveBeenCalledOnce();
    expect(custom.mock.calls[0]?.[1]).toMatchObject({ id: "layout-upgrade:occ-1,occ-2" });
    const frame = custom.mock.calls[0]?.[0]("toast") as ReactElement<{
      body?: string;
      meta: string;
    }>;
    expect(frame.props.body).toBe("Floating and minimized windows are tabs now.");
    expect(frame.props.meta).toBe("atlas · beta");
  });
});
