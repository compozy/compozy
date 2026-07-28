// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createElement, type ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

const navigateMock = vi.fn();
const promoteNetworkThreadTaskMock = vi.fn();
const listNetworkSubscriptionsMock = vi.fn();

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children }: { children: ReactNode }) => <>{children}</>,
  useNavigate: () => navigateMock,
}));

vi.mock("@/systems/network/adapters/network-api", async () => {
  const actual = await vi.importActual<typeof import("@/systems/network/adapters/network-api")>(
    "@/systems/network/adapters/network-api"
  );
  return {
    ...actual,
    listNetworkSubscriptions: (...args: unknown[]) => listNetworkSubscriptionsMock(...args),
    promoteNetworkThreadTask: (...args: unknown[]) => promoteNetworkThreadTaskMock(...args),
  };
});

import { ThreadOverlayHeader, ThreadTaskLinks } from "../thread-overlay-header";
import type { NetworkThreadDetail } from "../../../types";

function renderWithClient(children: ReactNode) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(createElement(QueryClientProvider, { client }, children));
}

const detail: NetworkThreadDetail = {
  channel: "ops",
  last_activity_at: "2026-04-17T18:16:00Z",
  last_message_preview: "Ship status is ready.",
  message_count: 4,
  open_work_count: 0,
  opened_at: "2026-04-17T18:00:00Z",
  opened_by_peer_id: "peer-self",
  participant_count: 2,
  root_message_id: "msg_root",
  thread_id: "thread_ops",
  title: "Launch follow-up",
};

describe("ThreadOverlayHeader", () => {
  it("promotes the thread to a task and navigates to the new task", async () => {
    const user = userEvent.setup();
    navigateMock.mockReset();
    listNetworkSubscriptionsMock.mockResolvedValue([]);
    promoteNetworkThreadTaskMock.mockResolvedValue({
      origin: {
        channel: "ops",
        digest: "Launch follow-up",
        origin_message_id: "msg_root",
        task_id: "task_thread",
        thread_id: "thread_ops",
      },
      task: {
        created_at: "2026-04-17T18:20:00Z",
        created_by: { kind: "network_peer", ref: "peer-self" },
        id: "task_thread",
        latest_event_seq: 1,
        origin: { kind: "network", ref: "ops/thread_ops" },
        scope: "workspace",
        status: "draft",
        title: "Launch follow-up",
        updated_at: "2026-04-17T18:20:00Z",
      },
    });

    renderWithClient(
      <ThreadOverlayHeader
        channel="ops"
        detail={detail}
        onClose={() => undefined}
        onOpenMain={() => undefined}
        rootMessageId="msg_root"
        selfSessionId="session-self"
        threadId="thread_ops"
        workspaceId="ws_1"
      />
    );

    await user.click(screen.getByTestId("network-thread-promote-task"));

    await waitFor(() => expect(promoteNetworkThreadTaskMock).toHaveBeenCalledTimes(1));
    expect(promoteNetworkThreadTaskMock.mock.calls[0]).toMatchObject([
      "ws_1",
      "ops",
      "thread_ops",
      { origin_message_id: "msg_root", title: "Launch follow-up" },
    ]);
    expect(navigateMock).toHaveBeenCalledWith({
      params: { id: "task_thread" },
      to: "/tasks/$id",
    });
  });

  it("renders linked task pills for promoted thread origins", () => {
    render(
      <ThreadTaskLinks
        links={[
          {
            channel: "ops",
            digest: "Launch follow-up",
            origin_message_id: "msg_root",
            task_id: "task_thread",
            thread_id: "thread_ops",
            workspace_id: "ws_1",
          },
        ]}
      />
    );

    expect(screen.getByTestId("network-thread-task-links")).toHaveTextContent("task_thread");
  });
});
