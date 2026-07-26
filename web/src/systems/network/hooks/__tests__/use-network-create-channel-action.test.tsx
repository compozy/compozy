// @vitest-environment jsdom

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { act } from "react";
import { describe, beforeEach, expect, it, vi } from "vitest";

import { UIProvider } from "@agh/ui";
import { agentFixtures } from "@/systems/agent/mocks";

import { useNetworkCreateChannelAction } from "../use-network-create-channel-action";

type DialogProps = {
  agents: typeof agentFixtures;
  canSubmit: boolean;
  draft: ReturnType<typeof import("../../lib/network-formatters").createNetworkChannelDraft>;
  isSubmitting: boolean;
  onChannelNameChange: (value: string) => void;
  onOpenChange: (open: boolean) => void;
  onPurposeChange: (value: string) => void;
  onAgentSelectionChange: (agentNames: string[]) => void;
  onSubmit: () => void | Promise<void>;
  open: boolean;
  workspaceName?: string | null;
};

const navigateMock = vi.fn();
const mutateAsyncMock = vi.fn();
let latestDialogProps: DialogProps | null = null;

vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => navigateMock,
}));

vi.mock("@/systems/agent", () => ({
  useAgents: () => ({
    data: agentFixtures,
    isLoading: false,
  }),
}));

vi.mock("@/systems/workspace", () => ({
  useActiveWorkspace: () => ({
    activeWorkspace: { id: "ws_123", name: "polybot" },
    activeWorkspaceId: "ws_123",
  }),
}));

vi.mock("../use-network-actions", () => ({
  useCreateNetworkChannel: () => ({
    mutateAsync: mutateAsyncMock,
    isPending: false,
  }),
}));

vi.mock("../../components/network-create-channel-dialog", () => ({
  NetworkCreateChannelDialog: (props: DialogProps) => {
    latestDialogProps = props;
    return (
      <div
        data-can-submit={String(props.canSubmit)}
        data-open={String(props.open)}
        data-testid="network-create-channel-dialog-props"
      />
    );
  },
}));

function Harness({ enabled }: { enabled: boolean }) {
  const { action, dialog } = useNetworkCreateChannelAction({ enabled });
  return (
    <>
      {action}
      {dialog}
    </>
  );
}

describe("useNetworkCreateChannelAction", () => {
  beforeEach(() => {
    latestDialogProps = null;
    mutateAsyncMock.mockReset();
    navigateMock.mockReset();
  });

  it("Should block submission when enabled flips false while the dialog stays open", async () => {
    const user = userEvent.setup();
    const view = render(
      <UIProvider reducedMotion="always">
        <Harness enabled />
      </UIProvider>
    );

    await user.click(screen.getByTestId("network-open-create-dialog"));
    expect(latestDialogProps?.open).toBe(true);

    act(() => {
      latestDialogProps?.onChannelNameChange("deployments");
      latestDialogProps?.onPurposeChange("Coordinate deploy verification");
      latestDialogProps?.onAgentSelectionChange([agentFixtures[0]!.name]);
    });

    expect(screen.getByTestId("network-create-channel-dialog-props")).toHaveAttribute(
      "data-can-submit",
      "true"
    );

    view.rerender(
      <UIProvider reducedMotion="always">
        <Harness enabled={false} />
      </UIProvider>
    );

    expect(screen.getByTestId("network-create-channel-dialog-props")).toHaveAttribute(
      "data-open",
      "true"
    );
    expect(screen.getByTestId("network-create-channel-dialog-props")).toHaveAttribute(
      "data-can-submit",
      "false"
    );

    await act(async () => {
      await latestDialogProps?.onSubmit();
    });

    expect(mutateAsyncMock).not.toHaveBeenCalled();
  });

  it("Should submit successfully with async control flow and navigate to the new channel", async () => {
    const user = userEvent.setup();
    mutateAsyncMock.mockResolvedValue({
      channel: {
        channel: "deployments",
      },
    });

    render(
      <UIProvider reducedMotion="always">
        <Harness enabled />
      </UIProvider>
    );

    await user.click(screen.getByTestId("network-open-create-dialog"));

    act(() => {
      latestDialogProps?.onChannelNameChange("  deployments  ");
      latestDialogProps?.onPurposeChange("  Coordinate deploy verification  ");
      latestDialogProps?.onAgentSelectionChange([agentFixtures[0]!.name]);
    });

    await act(async () => {
      await latestDialogProps?.onSubmit();
    });

    expect(mutateAsyncMock).toHaveBeenCalledWith({
      agent_names: [agentFixtures[0]!.name],
      channel: "deployments",
      fanout_policy: "capability_match",
      purpose: "Coordinate deploy verification",
      workspace_id: "ws_123",
    });
    // `coordinator_peer_id` is rejected by the daemon under any other policy and
    // has no resolvable value before the member sessions exist.
    expect(mutateAsyncMock.mock.calls.at(-1)?.[0]).not.toHaveProperty("coordinator_peer_id");
    expect(navigateMock).toHaveBeenCalledWith({
      params: { workspaceId: "ws_123", channel: "deployments" },
      to: "/network/$workspaceId/$channel/threads",
    });
    expect(screen.getByTestId("network-create-channel-dialog-props")).toHaveAttribute(
      "data-open",
      "false"
    );
  });
});
