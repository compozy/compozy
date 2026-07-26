// @vitest-environment jsdom

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
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

import { DirectsList } from "../directs-list";
import type { NetworkDirectRoomSummary } from "../../../types";

const WORKSPACE_ID = "ws_alpha";

const directs: NetworkDirectRoomSummary[] = [
  {
    channel: "ops",
    direct_id: "direct-1",
    last_activity_at: "2026-04-17T18:00:00Z",
    last_message_preview: "Confirmed.",
    message_count: 4,
    open_work_count: 0,
    opened_at: "2026-04-17T17:00:00Z",
    session_a: "sess-self",
    session_b: "sess-remote",
  },
];

const members = [
  {
    peerId: "peer-remote",
    sessionId: "sess-remote",
    displayName: "Remote",
    role: "agent" as const,
    local: true,
    presenceState: "local" as const,
  },
];

describe("DirectsList", () => {
  it("Should resolve the other session to its peer identity", () => {
    render(
      <DirectsList
        activeDirectId={null}
        channel="ops"
        directs={directs}
        isLoading={false}
        members={members}
        selfSessionId="sess-self"
        workspaceId={WORKSPACE_ID}
      />
    );

    expect(screen.getByText("@peer-remote")).toBeInTheDocument();
    expect(screen.queryByText("@peer-self")).toBeNull();
  });

  it("Should mark the active direct row with aria-current and data-selected", () => {
    render(
      <DirectsList
        activeDirectId="direct-1"
        channel="ops"
        directs={directs}
        isLoading={false}
        members={members}
        selfSessionId="sess-self"
        workspaceId={WORKSPACE_ID}
      />
    );

    const row = screen.getByTestId("network-direct-list-row-direct-1");
    expect(row).toHaveAttribute("aria-current", "page");
    expect(row).toHaveAttribute("data-selected", "true");
  });

  it("Should render the empty state when no directs exist", () => {
    render(
      <DirectsList
        activeDirectId={null}
        channel="ops"
        directs={[]}
        isLoading={false}
        workspaceId={WORKSPACE_ID}
      />
    );
    expect(screen.getByText("No direct rooms yet.")).toBeInTheDocument();
  });

  it("Should render the loading skeleton when no directs are available yet", () => {
    render(
      <DirectsList
        activeDirectId={null}
        channel="ops"
        directs={[]}
        isLoading
        workspaceId={WORKSPACE_ID}
      />
    );
    expect(screen.getByTestId("network-direct-list-skeleton")).toBeInTheDocument();
  });

  it("Should expose an accessible load-more action when the server has another page", async () => {
    const user = userEvent.setup();
    const onLoadMore = vi.fn();
    render(
      <DirectsList
        activeDirectId={null}
        channel="ops"
        directs={directs}
        hasMore
        isLoading={false}
        isLoadingMore={false}
        onLoadMore={onLoadMore}
        total={2}
        workspaceId={WORKSPACE_ID}
      />
    );

    await user.click(screen.getByRole("button", { name: "Load more direct rooms" }));
    expect(onLoadMore).toHaveBeenCalledTimes(1);
  });
});
