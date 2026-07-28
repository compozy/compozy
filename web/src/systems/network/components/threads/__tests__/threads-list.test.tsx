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

import { ThreadsList } from "../threads-list";
import type { NetworkThreadSummary } from "../../../types";

const WORKSPACE_ID = "ws_alpha";

const threads: NetworkThreadSummary[] = [
  {
    channel: "ops",
    last_activity_at: "2026-04-17T18:00:00Z",
    last_message_preview: "Pricing decision pending.",
    message_count: 12,
    open_work_count: 0,
    opened_at: "2026-04-17T17:00:00Z",
    opened_by_peer_id: "peer-codex",
    opened_session_id: "sess-1",
    participant_count: 3,
    root_message_id: "msg-1",
    thread_id: "thread-1",
    title: "Launch pricing decision",
  },
];

describe("ThreadsList", () => {
  it("Should render rows with title, preview, and metadata", () => {
    render(
      <ThreadsList
        activeThreadId={null}
        channel="ops"
        status="ready"
        threads={threads}
        workspaceId={WORKSPACE_ID}
      />
    );

    expect(screen.getByTestId("network-thread-list-row-thread-1")).toBeInTheDocument();
    expect(screen.getByText("Launch pricing decision")).toBeInTheDocument();
    expect(screen.getByText("Pricing decision pending.")).toBeInTheDocument();
  });

  it("Should mark the active thread row with aria-current and data-selected", () => {
    render(
      <ThreadsList
        activeThreadId="thread-1"
        channel="ops"
        status="ready"
        threads={threads}
        workspaceId={WORKSPACE_ID}
      />
    );

    const row = screen.getByTestId("network-thread-list-row-thread-1");
    expect(row).toHaveAttribute("aria-current", "page");
    expect(row).toHaveAttribute("data-selected", "true");
  });

  it("Should render the loading skeleton when no threads are available yet", () => {
    render(
      <ThreadsList
        activeThreadId={null}
        channel="ops"
        status="loading"
        threads={[]}
        workspaceId={WORKSPACE_ID}
      />
    );
    expect(screen.getByTestId("network-thread-list-skeleton")).toBeInTheDocument();
  });

  it("Should render the empty state when no threads exist", () => {
    render(
      <ThreadsList
        activeThreadId={null}
        channel="ops"
        status="ready"
        threads={[]}
        workspaceId={WORKSPACE_ID}
      />
    );
    expect(screen.getByText("No threads yet.")).toBeInTheDocument();
  });

  it("Should reduce contrast when dim is true", () => {
    render(
      <ThreadsList
        activeThreadId="thread-1"
        channel="ops"
        dim
        status="ready"
        threads={threads}
        workspaceId={WORKSPACE_ID}
      />
    );
    expect(screen.getByTestId("network-thread-list")).toHaveAttribute("data-dim", "true");
  });

  it("Should expose an accessible load-more action when the server has another page", async () => {
    const user = userEvent.setup();
    const onLoadMore = vi.fn();
    render(
      <ThreadsList
        activeThreadId={null}
        channel="ops"
        status="ready"
        paginationStatus="available"
        onLoadMore={onLoadMore}
        threads={threads}
        total={2}
        workspaceId={WORKSPACE_ID}
      />
    );

    await user.click(screen.getByRole("button", { name: "Load more threads" }));
    expect(onLoadMore).toHaveBeenCalledTimes(1);
  });

  it("Should truncate long title and preview without expanding row width", () => {
    const longPreview =
      "Modernize the visual language and component system - Improve information architecture and key user flows - Raise accessibility (WCAG 2.2 AA) and Core Web Vitals across the app - Establish a token-driven design system that engineering can consume directly";
    const longTitle =
      "Kicking off a new thread to coordinate a redesign of the network shell and recents rail with a very long title that must not overflow";
    const longThread: NetworkThreadSummary = {
      ...threads[0]!,
      thread_id: "thread-long",
      title: longTitle,
      last_message_preview: longPreview,
    };

    const { container } = render(
      <div className="max-w-sm">
        <ThreadsList
          activeThreadId={null}
          channel="design"
          status="ready"
          threads={[longThread]}
          workspaceId={WORKSPACE_ID}
        />
      </div>
    );

    const row = screen.getByTestId("network-thread-list-row-thread-long");
    const title = screen.getByTestId("network-thread-list-row-title-thread-long");
    const preview = screen.getByTestId("network-thread-list-row-preview-thread-long");

    expect(title).toHaveClass("min-w-0", "truncate");
    expect(title).toHaveAttribute("title", longTitle);
    expect(preview).toHaveClass("truncate");
    expect(preview).toHaveAttribute("title", longPreview);

    const containerEl = container.firstElementChild as HTMLElement;
    Object.defineProperty(containerEl, "clientWidth", { configurable: true, value: 384 });
    Object.defineProperty(row, "scrollWidth", { configurable: true, value: 384 });
    expect(row.scrollWidth).toBeLessThanOrEqual(containerEl.clientWidth + 1);
  });
});
