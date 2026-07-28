import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { OpenWorkEntry } from "../../../hooks/use-work";
import type { NetworkStatus } from "../../../types";
import { NetworkHeadStatus } from "../network-head-status";

const activeStatus: NetworkStatus = {
  enabled: true,
  status: "active",
  local_peers: 3,
};

const workingEntry: OpenWorkEntry = {
  workId: "work_1",
  state: "working",
  messageId: "message_1",
  targetPeerId: "peer_1",
  openedAt: null,
  lastActivityAt: null,
};

describe("NetworkHeadStatus", () => {
  it("Should render the authoritative open count beyond loaded work evidence", () => {
    render(
      <NetworkHeadStatus
        drilledIntoConversation
        openWorkCount={4}
        status={activeStatus}
        workEntries={[workingEntry]}
      />
    );

    expect(screen.getByTestId("network-head-status")).toHaveTextContent("working · 4 work open");
  });

  it.each([
    {
      name: "active network",
      drilledIntoConversation: false,
      status: activeStatus,
      workEntries: [] as OpenWorkEntry[],
      openWorkCount: 0,
      label: "Active · 3 live",
      tone: "success",
    },
    {
      name: "ready listener",
      drilledIntoConversation: false,
      status: { ...activeStatus, status: "ready", local_peers: 0 },
      workEntries: [] as OpenWorkEntry[],
      openWorkCount: 0,
      label: "Ready",
      tone: "info",
    },
    {
      name: "working conversation",
      drilledIntoConversation: true,
      status: activeStatus,
      workEntries: [workingEntry],
      openWorkCount: 1,
      label: "working · 1 work open",
      tone: "info",
    },
    {
      name: "conversation needing input",
      drilledIntoConversation: true,
      status: activeStatus,
      workEntries: [{ ...workingEntry, state: "needs_input" }],
      openWorkCount: 1,
      label: "needs input · 1 work open",
      tone: "warning",
    },
  ])("Should project the $name state", props => {
    render(<NetworkHeadStatus {...props} />);

    const status = screen.getByTestId("network-head-status");
    expect(status).toHaveTextContent(props.label);
    expect(status).toHaveAttribute("data-tone", props.tone);
  });

  it("Should render no status when Network is disabled", () => {
    const { container } = render(
      <NetworkHeadStatus
        drilledIntoConversation={false}
        openWorkCount={0}
        status={{ ...activeStatus, enabled: false, status: "disabled" }}
        workEntries={[]}
      />
    );

    expect(container).toBeEmptyDOMElement();
  });
});
