// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const directDetailMock = vi.hoisted(() => vi.fn());
const directDetailArgsMock = vi.hoisted(() => vi.fn());
const messagesArgsMock = vi.hoisted(() => vi.fn());
const listNetworkPeersMock = vi.hoisted(() => vi.fn());

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, ...rest }: { children: React.ReactNode }) => (
    <a {...(rest as Record<string, unknown>)}>{children}</a>
  ),
  useNavigate: () => () => undefined,
}));

vi.mock("../../../adapters/network-api", async original => {
  const actual = (await original()) as Record<string, unknown>;
  return {
    ...actual,
    listNetworkPeers: (...args: unknown[]) => listNetworkPeersMock(...args),
  };
});

vi.mock("../../../hooks/use-directs", async () => {
  const actual = await vi.importActual<typeof import("../../../hooks/use-directs")>(
    "../../../hooks/use-directs"
  );
  return {
    ...actual,
    useNetworkDirectDetail: (...args: unknown[]) => {
      directDetailArgsMock(...args);
      return directDetailMock();
    },
  };
});

vi.mock("../../../hooks/use-messages", () => ({
  useNetworkMessages: (args: unknown) => {
    messagesArgsMock(args);
    return {
      messages: [],
      isLoading: false,
      isFetching: false,
      hasOlder: false,
      isLoadingOlder: false,
      loadOlder: vi.fn(),
      error: null,
    };
  },
}));

vi.mock("../../../hooks/use-active-session", () => ({
  useActiveNetworkSession: () => ({
    session: {
      channel: "ops",
      peerId: "peer-self",
      sessionId: "sess-self",
      displayName: "Self",
    },
    disabledReason: null,
    isLoading: false,
  }),
}));

import { DirectRoom } from "../direct-room";

describe("DirectRoom headerless layout", () => {
  beforeEach(() => {
    directDetailMock.mockReset();
    directDetailArgsMock.mockClear();
    messagesArgsMock.mockClear();
    listNetworkPeersMock.mockReset();
    listNetworkPeersMock.mockResolvedValue([
      {
        channel: "ops",
        display_name: "Local Peer",
        joined_at: "2026-04-17T18:00:00Z",
        local: true,
        peer_card: {
          peer_id: "peer-remote",
          profiles_supported: [],
          capabilities: [],
          artifacts_supported: [],
          trust_modes_supported: [],
        },
        peer_id: "peer-remote",
        presence_state: "local",
        session_id: "sess-remote",
      },
    ]);
    directDetailMock.mockReturnValue({
      direct: {
        workspace_id: "ws-test",
        channel: "ops",
        direct_id: "direct_test",
        last_activity_at: "2026-04-17T18:00:00Z",
        last_message_preview: "preview",
        message_count: 0,
        open_work_count: 0,
        opened_at: "2026-04-17T17:00:00Z",
        session_a: "sess-self",
        session_b: "sess-remote",
      },
      isLoading: false,
      error: null,
    });
  });

  function renderRoom() {
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    return render(
      <QueryClientProvider client={client}>
        <DirectRoom
          channel="ops"
          directId="direct_test"
          selfSessionId="sess-self"
          workspaceId="ws_test"
        />
      </QueryClientProvider>
    );
  }

  it("Should render no #channel name and no member count", async () => {
    renderRoom();
    expect(screen.queryByText("#ops")).toBeNull();
    expect(screen.queryByText(/members/i)).toBeNull();
    expect(await screen.findByText("@peer-remote")).toBeInTheDocument();
  });

  it("Should scope direct detail, history, and loaded work to the URL workspace", () => {
    renderRoom();
    expect(directDetailArgsMock).toHaveBeenCalledWith("ops", "direct_test", {
      workspaceId: "ws_test",
    });
    expect(messagesArgsMock.mock.calls.length).toBeGreaterThan(0);
    for (const [args] of messagesArgsMock.mock.calls) {
      expect(args).toEqual(expect.objectContaining({ workspaceId: "ws_test" }));
    }
  });

  it("Should render the identity header as a domain-local compact head", async () => {
    renderRoom();
    const header = screen.getByTestId("network-direct-identity-row");
    const title = header.querySelector("h2");
    expect(title).not.toBeNull();
    await screen.findByText("@peer-remote");
    expect(title?.textContent).toContain("@peer-remote");
  });

  it("Should render the role indicator as agent (eyebrow in actions slot)", () => {
    renderRoom();
    expect(screen.getByText("agent")).toBeInTheDocument();
  });

  it("Should render the daemon-derived direct peer presence", async () => {
    renderRoom();
    await screen.findByText("local");
    const badge = screen.getByTestId("network-direct-presence");
    expect(badge).toHaveAttribute("data-state", "local");
    expect(badge).toHaveAttribute("aria-label", "peer presence local");
    expect(badge).toHaveTextContent("local");
  });

  it("Should render an unavailable state without composer when the direct detail fails", () => {
    directDetailMock.mockReturnValue({
      direct: null,
      isLoading: false,
      error: new Error("Direct room not found"),
    });

    renderRoom();

    expect(screen.getByTestId("network-direct-room-error")).toHaveTextContent(
      "Direct room unavailable"
    );
    expect(screen.getByTestId("network-direct-room-error")).toHaveTextContent(
      "Could not load direct room direct_test. Choose an existing direct room from #ops."
    );
    expect(screen.queryByRole("textbox", { name: /message @peer/i })).toBeNull();
  });

  it("Should render loading state without composer while direct detail resolves", () => {
    directDetailMock.mockReturnValue({
      direct: null,
      isLoading: true,
      error: null,
    });

    renderRoom();

    expect(screen.getByTestId("network-timeline-skeleton")).toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: /message @peer/i })).toBeNull();
  });
});
