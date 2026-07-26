// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createElement, type ReactElement, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const listNetworkSubscriptionsMock = vi.fn();
const upsertNetworkSubscriptionMock = vi.fn();
const deleteNetworkSubscriptionMock = vi.fn();

vi.mock("@/systems/network/adapters/network-api", async () => {
  const actual = await vi.importActual<typeof import("@/systems/network/adapters/network-api")>(
    "@/systems/network/adapters/network-api"
  );
  return {
    ...actual,
    deleteNetworkSubscription: (...args: unknown[]) => deleteNetworkSubscriptionMock(...args),
    listNetworkSubscriptions: (...args: unknown[]) => listNetworkSubscriptionsMock(...args),
    upsertNetworkSubscription: (...args: unknown[]) => upsertNetworkSubscriptionMock(...args),
  };
});

vi.mock("@agh/ui", async () => {
  const actual = await vi.importActual<typeof import("@agh/ui")>("@agh/ui");
  const React = await vi.importActual<typeof import("react")>("react");
  return {
    ...actual,
    DropdownMenu: ({ children }: { children: ReactNode }) =>
      React.createElement("div", null, children),
    DropdownMenuContent: ({ children }: { children: ReactNode }) =>
      React.createElement("div", null, children),
    DropdownMenuItem: ({
      children,
      onSelect,
      ...props
    }: {
      children: ReactNode;
      onSelect?: (event: { preventDefault: () => void }) => void;
      [key: string]: unknown;
    }) =>
      React.createElement(
        "button",
        {
          ...props,
          onClick: () => onSelect?.({ preventDefault: vi.fn() }),
          type: "button",
        },
        children
      ),
    DropdownMenuTrigger: ({ children, render }: { children: ReactNode; render?: ReactElement }) =>
      React.isValidElement(render)
        ? React.cloneElement(render, {}, children)
        : React.createElement("button", { type: "button" }, children),
  };
});

import { ThreadSubscriptionControl } from "../thread-subscription-control";

function renderWithClient(children: ReactNode) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(createElement(QueryClientProvider, { client }, children));
}

describe("ThreadSubscriptionControl", () => {
  beforeEach(() => {
    listNetworkSubscriptionsMock.mockReset();
    upsertNetworkSubscriptionMock.mockReset();
    deleteNetworkSubscriptionMock.mockReset();
  });

  it("Should show a surviving mode and upsert it for the current session", async () => {
    const user = userEvent.setup();
    listNetworkSubscriptionsMock.mockResolvedValue([
      {
        channel: "ops",
        mode: "mute",
        session_id: "session-self",
        thread_id: "thread_ops",
      },
    ]);
    upsertNetworkSubscriptionMock.mockResolvedValue({
      channel: "ops",
      mode: "full",
      session_id: "session-self",
      thread_id: "thread_ops",
    });

    renderWithClient(
      <ThreadSubscriptionControl
        channel="ops"
        sessionId="session-self"
        threadId="thread_ops"
        workspaceId="ws_1"
      />
    );

    await waitFor(() =>
      expect(screen.getByTestId("network-thread-subscription-trigger")).toHaveTextContent("Muted")
    );
    expect(screen.queryByTestId("network-thread-subscription-digest")).not.toBeInTheDocument();
    await user.click(screen.getByTestId("network-thread-subscription-trigger"));
    await user.click(screen.getByTestId("network-thread-subscription-full"));

    await waitFor(() => expect(upsertNetworkSubscriptionMock).toHaveBeenCalledTimes(1));
    expect(upsertNetworkSubscriptionMock).toHaveBeenCalledWith("ws_1", "ops", {
      mode: "full",
      session_id: "session-self",
      thread_id: "thread_ops",
    });
  });

  it("Should delete the thread-scoped row for default routing", async () => {
    const user = userEvent.setup();
    listNetworkSubscriptionsMock.mockResolvedValue([]);
    deleteNetworkSubscriptionMock.mockResolvedValue(undefined);

    renderWithClient(
      <ThreadSubscriptionControl
        channel="ops"
        sessionId="session-self"
        threadId="thread_ops"
        workspaceId="ws_1"
      />
    );

    await waitFor(() =>
      expect(screen.getByTestId("network-thread-subscription-trigger")).toHaveTextContent("Default")
    );
    await user.click(screen.getByTestId("network-thread-subscription-trigger"));
    await user.click(screen.getByTestId("network-thread-subscription-default"));

    await waitFor(() => expect(deleteNetworkSubscriptionMock).toHaveBeenCalledTimes(1));
    expect(deleteNetworkSubscriptionMock).toHaveBeenCalledWith("ws_1", "ops", "session-self", {
      thread_id: "thread_ops",
    });
  });

  it("Should show an unavailable disabled state when subscriptions cannot load", async () => {
    listNetworkSubscriptionsMock.mockRejectedValue(new Error("subscription lookup failed"));

    renderWithClient(
      <ThreadSubscriptionControl
        channel="ops"
        sessionId="session-self"
        threadId="thread_ops"
        workspaceId="ws_1"
      />
    );

    await waitFor(() =>
      expect(screen.getByTestId("network-thread-subscription-trigger")).toHaveTextContent(
        "Unavailable"
      )
    );
    expect(screen.getByTestId("network-thread-subscription-trigger")).toBeDisabled();
    expect(screen.getByTestId("network-thread-subscription-trigger")).not.toHaveTextContent(
      "Default"
    );
    expect(upsertNetworkSubscriptionMock).not.toHaveBeenCalled();
    expect(deleteNetworkSubscriptionMock).not.toHaveBeenCalled();
  });
});
