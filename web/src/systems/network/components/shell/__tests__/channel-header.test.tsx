// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";

vi.mock("../../../hooks/use-channel-members", () => ({
  useChannelMembers: () => ({
    members: [],
    agentCount: 1,
    humanCount: 1,
    isLoading: false,
  }),
}));

import { ChannelHeader } from "../channel-header";
import type { NetworkChannel, NetworkChannelSummary } from "@/systems/network";

const WORKSPACE_ID = "w1";

function renderHeader({
  channel = sampleChannel,
  detail = null,
  threadCount = 3,
}: {
  channel?: NetworkChannelSummary;
  detail?: NetworkChannel | null;
  threadCount?: number | null;
} = {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  return render(
    <ChannelHeader
      workspaceId={WORKSPACE_ID}
      channel={channel}
      detail={detail}
      openWorkCount={2}
      threadCount={threadCount}
    />,
    { wrapper }
  );
}

const sampleChannel: NetworkChannelSummary = {
  channel: "ops",
  workspace_id: "w1",
  created_at: "2026-04-17T14:00:00Z",
  created_by: "ops",
  peer_count: 4,
  purpose: "Operational coordination.",
  fanout_policy: "capability_match",
};

describe("ChannelHeader", () => {
  it("Should render the chead title without a DetailHeader or icon tile", () => {
    renderHeader();
    const wrapper = screen.getByTestId("network-channel-header");
    expect(wrapper.querySelector('[data-slot="detail-header"]')).toBeNull();
    expect(wrapper.querySelector('[data-slot="page-header-icon"]')).toBeNull();
    expect(screen.getByTestId("network-channel-title")).toHaveTextContent("ops");
  });

  it("Should carry no action buttons (inspector toggle and kebab live in the toolbar)", () => {
    renderHeader();
    expect(screen.queryByTestId("network-channel-inspector-toggle")).toBeNull();
    expect(screen.queryByTestId("network-channel-kebab")).toBeNull();
    expect(screen.getByTestId("network-channel-header").querySelector("button")).toBeNull();
  });

  it("Should render the work pill, fanout pill, purpose line, and dot meta", () => {
    renderHeader();
    expect(screen.getByTestId("network-channel-work-pill")).toHaveTextContent("2 work open");
    expect(screen.getByTestId("network-channel-fanout-pill")).toHaveTextContent("capability_match");
    expect(screen.getByTestId("network-channel-purpose")).toHaveTextContent(
      "Operational coordination."
    );
    expect(screen.getByTestId("network-channel-meta-agents")).toHaveTextContent("1 agent");
    expect(screen.getByTestId("network-channel-meta-humans")).toHaveTextContent("1 human");
    expect(screen.getByTestId("network-channel-meta-threads")).toHaveTextContent("3 threads");
  });
});
