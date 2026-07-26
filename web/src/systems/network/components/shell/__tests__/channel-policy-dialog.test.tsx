// @vitest-environment jsdom

import { dialogShellClass, UIProvider } from "@agh/ui";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { ChannelPolicyDialog } from "../channel-policy-dialog";
import type { ChannelMember } from "../../../hooks/use-channel-members";
import type { NetworkChannel, NetworkChannelSummary } from "../../../types";

const channel: NetworkChannelSummary = {
  channel: "ops",
  coordinator_peer_id: "peer-release",
  created_at: "2026-04-17T14:00:00Z",
  created_by: "ops",
  fanout_policy: "coordinator",
  peer_count: 4,
  purpose: "Coordinate launch.",
  workspace_id: "ws_1",
};

const detail = {
  ...channel,
  kind_counts: [],
  peers: [],
} as NetworkChannel;

const members: ChannelMember[] = [
  {
    displayName: "Release",
    local: true,
    peerId: "peer-release",
    sessionId: "session-release",
    presenceState: "local",
    role: "agent",
  },
  {
    displayName: "Support",
    local: true,
    peerId: "peer-support",
    sessionId: "session-support",
    presenceState: "local",
    role: "agent",
  },
];

type DialogProps = React.ComponentProps<typeof ChannelPolicyDialog>;

function renderDialog(overrides: Partial<DialogProps> = {}) {
  const props: DialogProps = {
    channel,
    detail,
    isSubmitting: false,
    members,
    onOpenChange: vi.fn(),
    onSubmit: vi.fn().mockResolvedValue(undefined),
    open: true,
    ...overrides,
  };
  render(
    <UIProvider reducedMotion="always">
      <ChannelPolicyDialog {...props} />
    </UIProvider>
  );
  return props;
}

describe("ChannelPolicyDialog", () => {
  it("Should host the editor on the shared sm modal token with a ruled entity header", () => {
    renderDialog();
    const host = screen.getByTestId("channel-policy-dialog");
    for (const token of dialogShellClass("sm").split(" ")) {
      expect(host.className).toContain(token);
    }
    expect(host.className).not.toMatch(/max-w-120/);
    expect(host.querySelector('[data-slot="dialog-header"]')).toHaveAttribute(
      "data-variant",
      "ruled"
    );
    expect(host.querySelectorAll('[data-slot="entity-dialog-header-close"]')).toHaveLength(1);
  });

  it("Should lock the channel name and membership behind ImmutableIdentity", () => {
    renderDialog();
    const identity = screen.getByTestId("channel-policy-identity");
    expect(within(identity).getByText("ops")).toBeInTheDocument();
    expect(within(identity).getByText("Release, Support")).toBeInTheDocument();
    // The update contract omits both, so neither may exist as a form control.
    expect(screen.queryByLabelText("Channel name")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Members")).not.toBeInTheDocument();
  });

  it("Should submit purpose, policy, and coordinator peer", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    const onOpenChange = vi.fn();
    renderDialog({ onOpenChange, onSubmit });

    await user.clear(screen.getByTestId("channel-policy-purpose"));
    await user.type(screen.getByTestId("channel-policy-purpose"), "Coordinate final rollout.");
    await user.click(screen.getByTestId("channel-policy-submit"));

    expect(onSubmit).toHaveBeenCalledWith({
      coordinator_peer_id: "peer-release",
      fanout_policy: "coordinator",
      purpose: "Coordinate final rollout.",
    });
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("Should reveal the coordinator picker only for the coordinator policy", async () => {
    const user = userEvent.setup();
    renderDialog();

    expect(screen.getByTestId("channel-policy-coordinator")).toBeInTheDocument();
    await user.click(screen.getByTestId("channel-policy-fanout-all_members"));
    expect(screen.queryByTestId("channel-policy-coordinator")).not.toBeInTheDocument();
  });

  it("Should reset the coordinator peer when selecting a non-coordinator fanout policy", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    const onOpenChange = vi.fn();
    renderDialog({ onOpenChange, onSubmit });

    await user.click(screen.getByTestId("channel-policy-fanout-all_members"));
    await user.click(screen.getByTestId("channel-policy-submit"));

    expect(onSubmit).toHaveBeenCalledWith({
      coordinator_peer_id: "",
      fanout_policy: "all_members",
      purpose: "Coordinate launch.",
    });
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("Should select a coordinator peer from the live member catalog", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    renderDialog({ onSubmit });

    await user.click(screen.getByTestId("channel-policy-coordinator"));
    await user.click(await screen.findByTestId("channel-policy-coordinator-peer-support"));
    await user.click(screen.getByTestId("channel-policy-submit"));

    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({ coordinator_peer_id: "peer-support" })
    );
  });

  it("Should block the save while coordinator policy has no peer", async () => {
    const user = userEvent.setup();
    renderDialog({
      channel: { ...channel, coordinator_peer_id: "" },
      detail: { ...detail, coordinator_peer_id: "" } as NetworkChannel,
    });

    expect(screen.getByTestId("channel-policy-submit")).toBeDisabled();
    await user.click(screen.getByTestId("channel-policy-coordinator"));
    await user.click(await screen.findByTestId("channel-policy-coordinator-peer-release"));
    expect(screen.getByTestId("channel-policy-submit")).toBeEnabled();
  });

  it("Should keep the dialog open and report submit errors", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn().mockRejectedValue(new Error("save failed"));
    const onOpenChange = vi.fn();
    renderDialog({ onOpenChange, onSubmit });

    await user.click(screen.getByTestId("channel-policy-submit"));

    expect(await screen.findByRole("alert")).toHaveTextContent("save failed");
    expect(onOpenChange).not.toHaveBeenCalledWith(false);
  });
});
