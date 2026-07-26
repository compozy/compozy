// @vitest-environment jsdom

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";

vi.mock("@tanstack/react-router", () => ({
  Link: ({
    to,
    params,
    children,
    ...rest
  }: {
    to: string;
    params?: Record<string, string>;
    children: ReactNode;
    [key: string]: unknown;
  }) => {
    const path = Object.entries(params ?? {}).reduce(
      (acc, [key, value]) => acc.replace(`$${key}`, String(value)),
      to
    );
    return (
      <a href={path} {...(rest as Record<string, unknown>)}>
        {children}
      </a>
    );
  },
}));

import {
  CHANNEL_RAIL_COLLAPSE_BREAKPOINT,
  CHANNEL_RAIL_MD_BREAKPOINT,
  CHANNEL_RAIL_WIDTH_DEFAULT,
  CHANNEL_RAIL_WIDTH_MD,
  ChannelRail,
} from "../channel-rail";
import { ChannelRailRecents } from "../channel-rail-recents";
import type { NetworkChannelSummary, NetworkRecentEntry } from "@/systems/network";

const WORKSPACE_ID = "ws_alpha";

interface InstallMatchMediaArgs {
  matches: (query: string) => boolean;
}

function installMatchMedia({ matches }: InstallMatchMediaArgs) {
  const listenersByQuery = new Map<string, Set<EventListenerOrEventListenerObject>>();
  const original = window.matchMedia;
  window.matchMedia = vi.fn().mockImplementation((query: string) => {
    const listeners = listenersByQuery.get(query) ?? new Set();
    listenersByQuery.set(query, listeners);
    return {
      matches: matches(query),
      media: query,
      addEventListener: (_event: string, handler: EventListenerOrEventListenerObject) => {
        listeners.add(handler);
      },
      removeEventListener: (_event: string, handler: EventListenerOrEventListenerObject) => {
        listeners.delete(handler);
      },
      addListener: () => undefined,
      removeListener: () => undefined,
      dispatchEvent: () => false,
      onchange: null,
    } satisfies MediaQueryList;
  });
  return () => {
    window.matchMedia = original;
  };
}

const channels: NetworkChannelSummary[] = [
  {
    channel: "alpha",
    workspace_id: "w1",
    created_at: "2026-04-17T14:00:00Z",
    created_by: "ops",
    peer_count: 2,
  },
  {
    channel: "design",
    workspace_id: "w1",
    created_at: "2026-04-17T14:00:00Z",
    created_by: "ops",
    peer_count: 2,
  },
  {
    channel: "ops",
    workspace_id: "w1",
    created_at: "2026-04-17T14:00:00Z",
    created_by: "ops",
    peer_count: 4,
    last_activity_at: "2026-04-17T18:00:00Z",
  },
];

const recents: NetworkRecentEntry[] = [
  {
    surface: "thread",
    channel: "ops",
    containerId: "thread_ops_one",
    preview: "Plan the migration",
    lastActivityAt: "2026-04-17T18:16:00Z",
    hasUnread: true,
    participantLabel: "4 peers",
  },
  {
    surface: "direct",
    channel: "design",
    containerId: "direct_design_one",
    preview: "ETA 18:30",
    lastActivityAt: "2026-04-17T17:00:00Z",
    hasUnread: false,
    participantLabel: "two-party",
  },
];

interface HarnessProps {
  pinnedIds?: string[];
  togglePinned?: (channel: string) => void;
}

function Harness({ pinnedIds = ["alpha"], togglePinned = () => undefined }: HarnessProps) {
  const pinnedSet = new Set(pinnedIds);
  return (
    <ChannelRail
      workspaceId={WORKSPACE_ID}
      activeChannel="ops"
      activeDirectId={null}
      directs={[]}
      hasUnread={() => true}
      loading={{ channels: false, directs: false, recents: false }}
      isPinned={channel => pinnedSet.has(channel)}
      onTogglePinned={togglePinned}
      pinnedChannels={channels.filter(channel => pinnedSet.has(channel.channel))}
      recents={recents}
      selfSessionId={null}
      unpinnedChannels={channels.filter(channel => !pinnedSet.has(channel.channel))}
    />
  );
}

function renderRail(props: HarnessProps = {}) {
  return render(<Harness {...props} />);
}

describe("ChannelRail", () => {
  let restoreMatchMedia: (() => void) | null = null;

  beforeEach(() => {
    restoreMatchMedia = installMatchMedia({ matches: () => false });
  });
  afterEach(() => {
    restoreMatchMedia?.();
  });

  it("renders pinned channels above unpinned, alphabetical thereafter", () => {
    renderRail({ pinnedIds: ["alpha"] });

    const channelOrder = screen
      .getAllByRole("link")
      .filter(link => link.getAttribute("data-testid")?.startsWith("network-channel-link-"))
      .map(link => link.getAttribute("data-testid")?.replace("network-channel-link-", "") ?? "");

    expect(channelOrder).toEqual(["alpha", "design", "ops"]);
  });

  it("renders cross-channel recents with surface-specific icons", () => {
    renderRail();
    const threadRecent = screen.getByTestId("network-recents-thread-thread_ops_one");
    const directRecent = screen.getByTestId("network-recents-direct-direct_design_one");
    expect(threadRecent).toBeDefined();
    expect(directRecent).toBeDefined();
    expect(threadRecent.querySelector("[aria-label='Thread']")).not.toBeNull();
    expect(directRecent.querySelector("[aria-label='Direct room']")).not.toBeNull();
  });

  it("Should render recents as a single truncated line with trailing freshness", () => {
    const longPreview =
      "Kicking off a new thread to coordinate a redesign of the network shell and recents rail.";
    const longRecents: NetworkRecentEntry[] = [
      {
        surface: "thread",
        channel: "design",
        containerId: "thread_long_preview",
        preview: longPreview,
        lastActivityAt: "2026-05-26T01:42:00Z",
        hasUnread: true,
        participantLabel: "3 peers",
      },
    ];

    render(
      <ChannelRailRecents workspaceId={WORKSPACE_ID} recents={longRecents} isLoading={false} />
    );

    const previewRow = screen.getByTestId("network-recents-preview-row-thread_long_preview");
    const preview = screen.getByTestId("network-recents-preview-thread_long_preview");
    const timestamp = screen.getByTestId("network-recents-time-thread_long_preview");

    expect(preview).toHaveClass("min-w-0", "flex-1", "truncate");
    expect(preview).toHaveAttribute("title", `${longPreview} — #design`);
    expect(previewRow).toContainElement(preview);
    expect(previewRow).toContainElement(timestamp);
    expect(screen.queryByTestId("network-recents-channel-row-thread_long_preview")).toBeNull();
  });

  it("invokes togglePinned when the pin affordance is clicked", async () => {
    const togglePinned = vi.fn();
    renderRail({ pinnedIds: [], togglePinned });
    const user = userEvent.setup();
    await user.click(screen.getByTestId("network-channel-pin-ops"));
    expect(togglePinned).toHaveBeenCalledWith("ops");
  });

  it("renders empty-state copy when no channels are visible", () => {
    render(
      <ChannelRail
        workspaceId={WORKSPACE_ID}
        activeChannel={null}
        activeDirectId={null}
        directs={[]}
        hasUnread={() => false}
        loading={{ channels: false, directs: false, recents: false }}
        isPinned={() => false}
        onTogglePinned={() => undefined}
        pinnedChannels={[]}
        recents={[]}
        selfSessionId={null}
        unpinnedChannels={[]}
      />
    );
    expect(screen.getByTestId("network-channels-empty")).toHaveTextContent("No channels yet.");
  });

  it("pins the rail width to 244 px at the default viewport", () => {
    expect(CHANNEL_RAIL_WIDTH_DEFAULT).toBe(244);
    renderRail();
    const rail = screen.getByTestId("network-channel-rail");
    expect(rail).toHaveAttribute("data-viewport", "default");
    expect(rail.style.width).toBe("244px");
  });

  it("collapses the rail width to 220 px below 1100 px (matching the sidebar md tier)", async () => {
    restoreMatchMedia?.();
    restoreMatchMedia = installMatchMedia({
      matches: query => query.includes(`max-width: ${CHANNEL_RAIL_MD_BREAKPOINT - 1}`),
    });
    expect(CHANNEL_RAIL_WIDTH_MD).toBe(220);
    renderRail();
    await waitFor(() => {
      const rail = screen.getByTestId("network-channel-rail");
      expect(rail).toHaveAttribute("data-viewport", "md");
      expect(rail.style.width).toBe("220px");
    });
  });

  it("hides the rail below 880 px (drawer tier)", async () => {
    restoreMatchMedia?.();
    restoreMatchMedia = installMatchMedia({
      matches: query =>
        query.includes(`max-width: ${CHANNEL_RAIL_COLLAPSE_BREAKPOINT - 1}`) ||
        query.includes(`max-width: ${CHANNEL_RAIL_MD_BREAKPOINT - 1}`),
    });
    renderRail();
    await waitFor(() => {
      expect(screen.queryByTestId("network-channel-rail")).toBeNull();
    });
  });

  it('paints active channel rows via <Item indicator="rail"> (--fg-strong via the primitive)', () => {
    renderRail();
    const activeRow = screen.getByTestId("network-channel-link-ops");
    // ItemSelectionIndicator data-indicator is added by the primitive when indicator="rail".
    const indicator = activeRow.querySelector('[data-indicator="rail"]');
    expect(indicator).not.toBeNull();
  });
});
