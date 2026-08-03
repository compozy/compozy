import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { extensionFixtures, extensionProvenanceFixtures } from "../../mocks/fixtures";

const mocks = vi.hoisted(() => ({
  provenance: {
    data: null as (typeof extensionProvenanceFixtures)[string] | null,
    error: null as Error | null,
    isLoading: false,
  },
  remove: vi.fn(),
}));

vi.mock("../../hooks/use-extension-actions", () => ({
  useRemoveExtension: () => ({
    error: null,
    isPending: false,
    mutateAsync: mocks.remove,
  }),
}));

vi.mock("../../hooks/use-extensions", () => ({
  useExtensionProvenance: () => mocks.provenance,
}));

import {
  ExtensionNetworkConfirmDialog,
  ExtensionProvenanceDialog,
  RemoveExtensionDialog,
  VerifiedMark,
} from "../extension-dialogs";

beforeEach(() => {
  vi.clearAllMocks();
  mocks.remove.mockResolvedValue(undefined);
  mocks.provenance.data = null;
  mocks.provenance.error = null;
  mocks.provenance.isLoading = false;
});

describe("RemoveExtensionDialog", () => {
  it("Should show declared permissions without gating removal on unrelated state", async () => {
    const user = userEvent.setup();
    render(<RemoveExtensionDialog extension={extensionFixtures[1]!} onOpenChange={vi.fn()} open />);

    expect(screen.getByRole("note")).toHaveTextContent(
      "Revoked permissions: network/send, sessions/list"
    );
    await user.type(screen.getByLabelText("Type to confirm"), "slack-notify");
    expect(screen.getByTestId("remove-extension-confirm")).toBeEnabled();
    expect(mocks.remove).not.toHaveBeenCalled();
  });

  it("Should require the exact extension name before invoking the real remove mutation", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    const onRemoved = vi.fn();
    render(
      <RemoveExtensionDialog
        extension={extensionFixtures[0]!}
        onOpenChange={onOpenChange}
        onRemoved={onRemoved}
        open
      />
    );

    const confirm = screen.getByTestId("remove-extension-confirm");
    await user.type(screen.getByLabelText("Type to confirm"), "otel");
    expect(confirm).toBeDisabled();
    await user.type(screen.getByLabelText("Type to confirm"), "-bridge");
    expect(confirm).toBeEnabled();
    await user.click(confirm);
    await waitFor(() =>
      expect(mocks.remove).toHaveBeenCalledWith({ dev: false, name: "otel-bridge" })
    );
    expect(onOpenChange).toHaveBeenCalledWith(false);
    expect(onRemoved).toHaveBeenCalledOnce();
  });

  it("Should unlink a workspace dev overlay without promising global removal", async () => {
    const user = userEvent.setup();
    const extension = { ...extensionFixtures[0]!, dev: true };
    render(<RemoveExtensionDialog extension={extension} onOpenChange={vi.fn()} open />);

    expect(screen.getByText(/published installation stays in place/i)).toBeInTheDocument();
    await user.type(screen.getByLabelText("Type to confirm"), extension.name);
    const confirm = screen.getByRole("button", { name: "Unlink dev overlay" });
    expect(confirm).toBeEnabled();
    await user.click(confirm);

    await waitFor(() =>
      expect(mocks.remove).toHaveBeenCalledWith({ dev: true, name: extension.name })
    );
  });
});

describe("ExtensionProvenanceDialog", () => {
  it("Should render loading, daemon failure, and the complete provenance payload", () => {
    mocks.provenance.isLoading = true;
    const { rerender } = render(
      <ExtensionProvenanceDialog extension={extensionFixtures[0]!} onOpenChange={vi.fn()} open />
    );
    expect(screen.getByText("Loading provenance")).toBeInTheDocument();

    mocks.provenance.isLoading = false;
    mocks.provenance.error = new Error("provenance unavailable");
    rerender(
      <ExtensionProvenanceDialog extension={extensionFixtures[0]!} onOpenChange={vi.fn()} open />
    );
    expect(screen.getByText("provenance unavailable")).toBeInTheDocument();

    mocks.provenance.error = null;
    rerender(
      <ExtensionProvenanceDialog extension={extensionFixtures[0]!} onOpenChange={vi.fn()} open />
    );
    expect(screen.getByText("No provenance data is available.")).toBeInTheDocument();

    mocks.provenance.data = extensionProvenanceFixtures["otel-bridge"]!;
    rerender(
      <ExtensionProvenanceDialog extension={extensionFixtures[0]!} onOpenChange={vi.fn()} open />
    );
    const content = screen.getByTestId("extension-provenance-content");
    expect(content).toHaveTextContent("marketplace_registry");
    expect(content).toHaveTextContent("sha256:otel-bridge-060");
    expect(content).toHaveTextContent("operator:web");
  });

  it("Should preserve mixed-case provenance source values outside MonoId", () => {
    mocks.provenance.data = {
      ...extensionProvenanceFixtures["otel-bridge"]!,
      installed_by: "Operator:Web",
      installed_from: "Marketplace_Registry",
      registry_tier: "Verified",
    };
    render(
      <ExtensionProvenanceDialog extension={extensionFixtures[0]!} onOpenChange={vi.fn()} open />
    );
    const content = screen.getByTestId("extension-provenance-content");
    expect(within(content).getByText("Marketplace_Registry")).toBeInTheDocument();
    expect(within(content).getByText("Verified")).toBeInTheDocument();
    expect(within(content).getByText("Operator:Web")).toBeInTheDocument();
    expect(within(content).getByText("sha256:otel-bridge-060")).toBeInTheDocument();
  });
});

describe("ExtensionNetworkConfirmDialog", () => {
  it.each([
    ["enable" as const, "Enabling dep-kit-ops"],
    ["update" as const, "Updating dep-kit-ops"],
  ])(
    "Should show the exact digest and preserve the confirmation callback for %s",
    async (action, sentence) => {
      const user = userEvent.setup();
      const onConfirm = vi.fn();
      render(
        <ExtensionNetworkConfirmDialog
          action={action}
          digest="sha256:6f1c0a94d3b27e58"
          error="confirmation rejected"
          extensionName="dep-kit-ops"
          onConfirm={onConfirm}
          onOpenChange={vi.fn()}
          open
          pending={false}
        />
      );

      expect(screen.getByText("confirmation rejected")).toBeInTheDocument();
      expect(screen.getByText(new RegExp(sentence))).toBeInTheDocument();
      expect(screen.getByTestId("extension-network-confirm-digest")).toHaveTextContent(
        "sha256:6f1c0a94d3b27e58"
      );
      await user.click(screen.getByTestId("extension-network-confirm-accept"));
      expect(onConfirm).toHaveBeenCalledOnce();
    }
  );
});

describe("VerifiedMark", () => {
  it("Should expose a verified checksum signal only when verification is true", () => {
    const { rerender } = render(<VerifiedMark verified />);
    expect(screen.getByLabelText("Checksum verified")).toBeInTheDocument();

    rerender(<VerifiedMark verified={false} />);
    expect(screen.queryByLabelText("Checksum verified")).not.toBeInTheDocument();
  });
});
