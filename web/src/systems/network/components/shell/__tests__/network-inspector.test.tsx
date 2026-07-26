// @vitest-environment jsdom

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { NetworkInspector } from "../network-inspector";
import type { NetworkChannelSummary } from "@/systems/network";

const sampleChannel: NetworkChannelSummary = {
  channel: "ops",
  peer_count: 4,
  purpose: "Operational coordination.",
  fanout_policy: "capability_match",
  created_by: "ops-human",
  created_at: "2026-04-17T14:00:00Z",
};

function renderInspector() {
  return render(
    <NetworkInspector
      activeTab="members"
      channel={sampleChannel}
      isMembersLoading={false}
      isWorkLoading={false}
      members={[]}
      onTabChange={() => undefined}
      workCount={0}
      workEntries={[]}
    />
  );
}

describe("NetworkInspector", () => {
  it("Should pin the About block with purpose and fixed KV rows", () => {
    renderInspector();
    expect(screen.getByTestId("network-inspector-purpose")).toHaveTextContent(
      "Operational coordination."
    );
    const about = screen.getByTestId("network-inspector-about");
    expect(about).toHaveTextContent("Fanout");
    expect(about).toHaveTextContent("capability_match");
    expect(about).toHaveTextContent("Created by");
  });

  it("Should offer exactly the Members and Work segments (no Activity tab, no chrome)", () => {
    renderInspector();
    expect(screen.getByTestId("network-inspector-tab-members")).toBeInTheDocument();
    expect(screen.getByTestId("network-inspector-tab-work")).toBeInTheDocument();
    expect(screen.queryByTestId("network-inspector-tab-activity")).toBeNull();
    expect(screen.queryByTestId("network-inspector-close")).toBeNull();
  });
});
