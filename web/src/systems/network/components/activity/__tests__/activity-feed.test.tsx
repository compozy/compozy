// @vitest-environment jsdom

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";

vi.mock("sonner", () => ({
  toast: { error: vi.fn() },
}));

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

import { ActivityFeed } from "../activity-feed";
import { toast } from "sonner";

const WORKSPACE_ID = "ws_alpha";

describe("ActivityFeed", () => {
  it("Should sort entries by last_activity_at across both surfaces", () => {
    render(
      <ActivityFeed
        workspaceId={WORKSPACE_ID}
        channel="ops"
        directs={[
          {
            channel: "ops",
            direct_id: "direct-1",
            last_activity_at: "2026-04-17T17:00:00Z",
            last_message_preview: "Older direct",
            message_count: 1,
            open_work_count: 0,
            opened_at: "2026-04-17T16:00:00Z",
            session_a: "sess-self",
            session_b: "sess-remote",
          },
        ]}
        status="ready"
        threads={[
          {
            channel: "ops",
            last_activity_at: "2026-04-17T18:00:00Z",
            last_message_preview: "Newer thread",
            message_count: 1,
            open_work_count: 0,
            opened_at: "2026-04-17T17:00:00Z",
            opened_by_peer_id: "self",
            opened_session_id: "sess-1",
            participant_count: 2,
            root_message_id: "m1",
            thread_id: "thread-1",
            title: "Newer thread",
          },
        ]}
      />
    );

    const entries = screen
      .getAllByTestId(/network-activity-entry-/u)
      .map(node => node.getAttribute("data-testid") ?? "");
    expect(entries[0]).toContain("thread:thread-1");
    expect(entries[1]).toContain("direct:direct-1");
  });

  it("Should render actor-first rows without kind tag prefixes", () => {
    render(
      <ActivityFeed
        workspaceId={WORKSPACE_ID}
        channel="ops"
        directs={[
          {
            channel: "ops",
            direct_id: "direct-1",
            last_activity_at: "2026-04-17T17:00:00Z",
            last_message_preview: "Direct preview",
            message_count: 1,
            open_work_count: 0,
            opened_at: "2026-04-17T16:00:00Z",
            session_a: "sess-self",
            session_b: "sess-remote",
          },
        ]}
        status="ready"
        threads={[
          {
            channel: "ops",
            last_activity_at: "2026-04-17T18:00:00Z",
            last_message_preview: "Thread preview",
            message_count: 1,
            open_work_count: 0,
            opened_at: "2026-04-17T17:00:00Z",
            opened_by_peer_id: "self",
            opened_session_id: "sess-1",
            participant_count: 2,
            root_message_id: "m1",
            thread_id: "thread-1",
            title: "Thread title",
          },
        ]}
      />
    );

    expect(screen.queryByText("[TH]")).toBeNull();
    expect(screen.queryByText("[DM]")).toBeNull();
    expect(screen.getByText("Thread title")).toBeInTheDocument();
    expect(screen.getByText(/Direct preview/u)).toBeInTheDocument();
  });

  it("Should expose independent pagination for both counted surfaces", async () => {
    const user = userEvent.setup();
    const loadThreads = vi.fn();
    const loadDirects = vi.fn();
    render(
      <ActivityFeed
        workspaceId={WORKSPACE_ID}
        channel="ops"
        directs={[]}
        directTotal={3}
        status="ready"
        pagination={{
          threads: { status: "available", onLoadMore: loadThreads },
          directs: { status: "available", onLoadMore: loadDirects },
        }}
        threadTotal={4}
        threads={[]}
      />
    );

    await user.click(screen.getByRole("button", { name: "Load more threads" }));
    await user.click(screen.getByRole("button", { name: "Load more direct rooms" }));
    expect(loadThreads).toHaveBeenCalledTimes(1);
    expect(loadDirects).toHaveBeenCalledTimes(1);
  });

  it("Should report a rejected page load and allow the operator to retry", async () => {
    const user = userEvent.setup();
    const loadThreads = vi
      .fn<() => Promise<void>>()
      .mockRejectedValueOnce(new Error("thread page unavailable"))
      .mockResolvedValue(undefined);
    render(
      <ActivityFeed
        workspaceId={WORKSPACE_ID}
        channel="ops"
        directs={[]}
        status="ready"
        pagination={{ threads: { status: "available", onLoadMore: loadThreads } }}
        threads={[]}
      />
    );

    const loadMoreButton = screen.getByRole("button", { name: "Load more threads" });
    await user.click(loadMoreButton);
    expect(toast.error).toHaveBeenCalledWith("thread page unavailable");

    await user.click(loadMoreButton);
    expect(loadThreads).toHaveBeenCalledTimes(2);
  });
});
