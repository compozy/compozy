// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createElement, type ReactNode } from "react";

const sendNetworkMessageMock = vi.fn();
const navigateMock = vi.fn();
const toastErrorMock = vi.fn();

vi.mock("../../../adapters/network-api", async () => {
  const actual = await vi.importActual<typeof import("../../../adapters/network-api")>(
    "../../../adapters/network-api"
  );
  return {
    ...actual,
    sendNetworkMessage: (...args: unknown[]) => sendNetworkMessageMock(...args),
  };
});

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children }: { children: ReactNode }) => <>{children}</>,
  useNavigate: () => navigateMock,
}));

vi.mock("@agh/ui", async () => {
  const actual = await vi.importActual<typeof import("@agh/ui")>("@agh/ui");
  return {
    ...actual,
    toast: {
      error: (...args: unknown[]) => toastErrorMock(...args),
    },
  };
});

import { ChannelThreadComposer } from "../channel-thread-composer";

const WORKSPACE_ID = "ws_alpha";

function renderComposer() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client }, children);
  return render(
    <ChannelThreadComposer
      workspaceId={WORKSPACE_ID}
      channel="ops"
      displayName="Codex"
      peerFrom="peer-self"
      sessionId="sess-1"
    />,
    { wrapper }
  );
}

describe("ChannelThreadComposer", () => {
  beforeEach(() => {
    sendNetworkMessageMock.mockReset();
    navigateMock.mockReset();
    toastErrorMock.mockReset();
  });
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("Should generate a thread_id, send the root say, and redirect on success", async () => {
    sendNetworkMessageMock.mockResolvedValue({
      message: { id: "client-id", kind: "say", channel: "ops", session_id: "sess-1" },
    });
    const user = userEvent.setup();
    renderComposer();

    await user.type(
      screen.getByTestId("network-composer-textarea-channel-thread"),
      "Open the launch thread with @peer.remote"
    );
    await user.click(screen.getByTestId("network-composer-send-channel-thread"));

    await waitFor(() => expect(sendNetworkMessageMock).toHaveBeenCalledTimes(1));
    expect(sendNetworkMessageMock.mock.calls[0]?.[0]).toBe(WORKSPACE_ID);
    const sent = sendNetworkMessageMock.mock.calls[0]?.[1] as Record<string, unknown>;
    expect(sent.surface).toBe("thread");
    expect(typeof sent.thread_id).toBe("string");
    expect((sent.thread_id as string).startsWith("thread_")).toBe(true);
    expect(sent.kind).toBe("say");
    expect(sent.mentions).toEqual(["peer.remote"]);
    expect(sent).not.toHaveProperty("interaction_id");

    await waitFor(() => expect(navigateMock).toHaveBeenCalledTimes(1));
    expect(navigateMock.mock.calls[0]?.[0]).toMatchObject({
      to: "/network/$workspaceId/$channel/threads/$threadId",
      params: { workspaceId: WORKSPACE_ID, channel: "ops" },
    });
  });

  it("Should retry once silently on collision, then surface a toast on the second failure", async () => {
    sendNetworkMessageMock.mockRejectedValue(new Error("collision"));
    const user = userEvent.setup();
    renderComposer();

    await user.type(
      screen.getByTestId("network-composer-textarea-channel-thread"),
      "Try this thread"
    );
    await user.click(screen.getByTestId("network-composer-send-channel-thread"));

    await waitFor(() => expect(sendNetworkMessageMock).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(toastErrorMock).toHaveBeenCalledTimes(1));
    expect(toastErrorMock).toHaveBeenCalledWith("Couldn't open this thread. Try again.");
    expect(navigateMock).not.toHaveBeenCalled();
  });
});
