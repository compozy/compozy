import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { dialogShellClass, OverlayContainerContext, UIProvider } from "@agh/ui";
import { agentFixtures } from "@/systems/agent/mocks";

import { createNetworkChannelDraft } from "../../lib/network-formatters";
import { NetworkCreateChannelDialog } from "../network-create-channel-dialog";

type DialogProps = React.ComponentProps<typeof NetworkCreateChannelDialog>;

function makeProps(overrides: Partial<DialogProps> = {}): DialogProps {
  return {
    agents: agentFixtures,
    canSubmit: true,
    draft: createNetworkChannelDraft(),
    isSubmitting: false,
    onAgentSelectionChange: () => undefined,
    onChannelNameChange: () => undefined,
    onFanoutPolicyChange: () => undefined,
    onOpenChange: () => undefined,
    onPurposeChange: () => undefined,
    onSubmit: () => undefined,
    open: true,
    workspaceName: "polybot",
    ...overrides,
  };
}

function renderDialog(
  overrides: Partial<DialogProps> = {},
  wrap?: (ui: React.ReactElement) => React.ReactElement
) {
  const dialog = <NetworkCreateChannelDialog {...makeProps(overrides)} />;
  return render(<UIProvider reducedMotion="always">{wrap ? wrap(dialog) : dialog}</UIProvider>);
}

describe("NetworkCreateChannelDialog", () => {
  it("Should host the editor on the shared sm modal token with a ruled entity header", () => {
    renderDialog();
    const host = screen.getByTestId("network-create-channel-dialog");
    for (const token of dialogShellClass("sm").split(" ")) {
      expect(host.className).toContain(token);
    }
    expect(host.className).not.toMatch(/max-w-120/);
    expect(host.querySelector('[data-slot="dialog-header"]')).toHaveAttribute(
      "data-variant",
      "ruled"
    );
    expect(screen.getByText("Network · Channel")).toBeInTheDocument();
  });

  it("Should expose exactly one close route", () => {
    renderDialog();
    const host = screen.getByTestId("network-create-channel-dialog");
    expect(host.querySelectorAll('[data-slot="entity-dialog-header-close"]')).toHaveLength(1);
    expect(within(host).getAllByRole("button", { name: "Close" })).toHaveLength(1);
  });

  it("Should render the submit button disabled and not fire onSubmit when canSubmit=false", () => {
    const onSubmit = vi.fn();
    renderDialog({ canSubmit: false, onSubmit });

    const submit = screen.getByTestId("network-create-channel-submit");
    expect(submit).toBeDisabled();
    fireEvent.click(submit);
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("Should surface the channel-name input wired to onChannelNameChange", () => {
    const onChannelNameChange = vi.fn();
    renderDialog({ onChannelNameChange });

    expect(
      screen.getByText("Use lowercase letters, numbers, underscores, or hyphens; e.g. coord_core.")
    ).toBeInTheDocument();
    expect(screen.getByTestId("network-channel-name-input")).toHaveAttribute(
      "placeholder",
      "e.g. website_copy"
    );
    fireEvent.change(screen.getByTestId("network-channel-name-input"), {
      target: { value: "deployments" },
    });

    expect(onChannelNameChange).toHaveBeenCalledWith("deployments");
  });

  it("Should surface the purpose input wired to onPurposeChange", () => {
    const onPurposeChange = vi.fn();
    renderDialog({ onPurposeChange });

    fireEvent.change(screen.getByTestId("network-channel-purpose-input"), {
      target: { value: "Coordinate deploy verification" },
    });

    expect(screen.getByTestId("network-channel-purpose-input")).toBeRequired();
    expect(screen.getByTestId("network-channel-purpose-input")).toHaveAttribute(
      "aria-required",
      "true"
    );
    expect(onPurposeChange).toHaveBeenCalledWith("Coordinate deploy verification");
  });

  it("Should offer the fanout policy as neutral radio cards defaulting to capability match", () => {
    renderDialog();
    const group = screen.getByTestId("network-create-channel-fanout-group");
    expect(group).toHaveAttribute("role", "radiogroup");
    expect(within(group).getAllByRole("radio")).toHaveLength(3);
    expect(screen.getByTestId("network-create-channel-fanout-capability_match")).toHaveAttribute(
      "aria-checked",
      "true"
    );
    // Selected state stays neutral glaze + rim; accent is reserved for true CTAs.
    expect(
      screen.getByTestId("network-create-channel-fanout-capability_match").className
    ).not.toMatch(/accent/);
  });

  it("Should report the selected fanout policy through onFanoutPolicyChange", async () => {
    const user = userEvent.setup();
    const onFanoutPolicyChange = vi.fn();
    renderDialog({ onFanoutPolicyChange });

    await user.click(screen.getByTestId("network-create-channel-fanout-all_members"));
    expect(onFanoutPolicyChange).toHaveBeenCalledWith("all_members");
  });

  it("Should keep coordinator unselectable at create time and explain why", async () => {
    const user = userEvent.setup();
    const onFanoutPolicyChange = vi.fn();
    renderDialog({ onFanoutPolicyChange });

    const coordinator = screen.getByTestId("network-create-channel-fanout-coordinator");
    expect(coordinator).toBeDisabled();
    await user.click(coordinator);
    expect(onFanoutPolicyChange).not.toHaveBeenCalled();
    expect(screen.getByTestId("network-create-channel-fanout-hint")).toHaveTextContent(
      /coordinator needs a live member peer/i
    );
  });

  it("Should toggle an agent when its row is clicked from the multi-select popover", async () => {
    const user = userEvent.setup();
    const onAgentSelectionChange = vi.fn();
    renderDialog({
      draft: { ...createNetworkChannelDraft(), channelName: "x" },
      onAgentSelectionChange,
    });

    const firstAgent = agentFixtures[0]!;
    await user.click(screen.getByTestId("network-create-channel-agent-trigger"));
    await user.click(screen.getByTestId(`network-agent-option-${firstAgent.name}`));
    expect(onAgentSelectionChange).toHaveBeenCalledWith([firstAgent.name]);
  });

  it("Should surface the Empty agents state and a workspace warning when the active workspace is missing", () => {
    renderDialog({ agents: [], canSubmit: false, workspaceName: null });

    expect(screen.getByText("No agents available")).toBeInTheDocument();
    expect(
      screen.getByText("Select an active workspace before creating a channel.")
    ).toBeInTheDocument();
  });

  it("Should fire onSubmit once when the form is submitted with canSubmit=true", () => {
    const onSubmit = vi.fn();
    renderDialog({
      draft: {
        ...createNetworkChannelDraft(),
        channelName: "deploy",
        purpose: "Coordinate deploy verification",
        selectedAgentNames: [agentFixtures[0]!.name],
      },
      onSubmit,
    });

    fireEvent.click(screen.getByTestId("network-create-channel-submit"));
    expect(onSubmit).toHaveBeenCalledTimes(1);
  });

  it("Should call onOpenChange when the Cancel button is pressed", () => {
    const onOpenChange = vi.fn();
    renderDialog({ onOpenChange });

    fireEvent.click(screen.getByTestId("network-create-channel-cancel"));
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("Should portal into the owning window overlay container", () => {
    const overlayContainer = document.createElement("div");
    overlayContainer.dataset.testid = "network-window-overlay-host";
    document.body.append(overlayContainer);

    try {
      renderDialog({}, dialog => (
        <OverlayContainerContext.Provider value={overlayContainer}>
          {dialog}
        </OverlayContainerContext.Provider>
      ));

      expect(overlayContainer).toContainElement(
        screen.getByTestId("network-create-channel-dialog")
      );
    } finally {
      overlayContainer.remove();
    }
  });
});
