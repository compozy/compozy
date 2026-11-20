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
  removeError: null as Error | null,
}));

vi.mock("../../hooks/use-extension-actions", () => ({
  useRemoveExtension: () => ({
    error: mocks.removeError,
    isPending: false,
    mutateAsync: mocks.remove,
  }),
}));

vi.mock("../../hooks/use-extensions", () => ({
  useExtensionProvenance: () => mocks.provenance,
}));

import {
  ExtensionGatewayConfirmDialog,
  ExtensionProvenanceDialog,
  RemoveExtensionDialog,
  VerifiedMark,
} from "../extension-dialogs";

beforeEach(() => {
  vi.clearAllMocks();
  mocks.remove.mockResolvedValue(undefined);
  mocks.removeError = null;
  mocks.provenance.data = null;
  mocks.provenance.error = null;
  mocks.provenance.isLoading = false;
});

/** A bundled built-in row at the global instance: the only row the permanent option applies to. */
const bundledRow = { ...extensionFixtures[0]!, dev: false, source: "bundled" };

describe("RemoveExtensionDialog permanent removal", () => {
  it("Should offer permanent removal for a bundled row with its consequence, defaulting to non-permanent", () => {
    render(
      <RemoveExtensionDialog
        canRemovePermanently
        extension={bundledRow}
        onOpenChange={vi.fn()}
        open
      />
    );

    const permanent = screen.getByTestId("remove-extension-choice-permanent");
    expect(permanent).toHaveTextContent("Remove permanently");
    expect(permanent).toHaveTextContent("The built-in will not return on restart or update.");
    expect(screen.getByTestId("remove-extension-choice-remove")).toHaveAttribute(
      "aria-checked",
      "true"
    );
    expect(permanent).toHaveAttribute("aria-checked", "false");
    expect(screen.getByTestId("remove-extension-confirm")).toHaveTextContent("Remove extension");
  });

  it("Should send the request without permanent unless the permanent choice is confirmed", async () => {
    const user = userEvent.setup();
    render(
      <RemoveExtensionDialog
        canRemovePermanently
        extension={bundledRow}
        onOpenChange={vi.fn()}
        open
      />
    );

    await user.type(screen.getByLabelText("Type to confirm"), "otel-bridge");
    await user.click(screen.getByTestId("remove-extension-confirm"));

    await waitFor(() =>
      expect(mocks.remove).toHaveBeenCalledWith({
        dev: false,
        name: "otel-bridge",
        profileName: "default",
      })
    );
    expect(mocks.remove.mock.calls[0]![0]).not.toHaveProperty("permanent");
  });

  it("Should send permanent=true without an instance scope when the permanent choice is confirmed", async () => {
    const user = userEvent.setup();
    const onRemoved = vi.fn();
    render(
      <RemoveExtensionDialog
        canRemovePermanently
        extension={bundledRow}
        onOpenChange={vi.fn()}
        onRemoved={onRemoved}
        open
      />
    );

    await user.click(screen.getByTestId("remove-extension-choice-permanent"));
    expect(screen.getByTestId("remove-extension-choice-permanent")).toHaveAttribute(
      "aria-checked",
      "true"
    );
    expect(screen.getByTestId("remove-extension-confirm")).toHaveTextContent("Remove permanently");
    await user.type(screen.getByLabelText("Type to confirm"), "otel-bridge");
    await user.click(screen.getByTestId("remove-extension-confirm"));

    await waitFor(() =>
      expect(mocks.remove).toHaveBeenCalledWith({
        dev: false,
        name: "otel-bridge",
        permanent: true,
      })
    );
    expect(mocks.remove.mock.calls[0]![0]).not.toHaveProperty("profileName");
    expect(onRemoved).toHaveBeenCalledOnce();
  });

  it.each([
    ["marketplace row", { ...extensionFixtures[0]!, source: "marketplace" }, true],
    ["local row", { ...extensionFixtures[0]!, source: "user" }, true],
    ["bundled row for an actor without write permission", bundledRow, false],
    ["bundled dev overlay", { ...bundledRow, dev: true }, true],
    ["bundled row attached to a workspace", { ...bundledRow, workspace_id: "ws_northstar" }, true],
  ])("Should hide the permanent option for a %s", (_label, extension, canRemovePermanently) => {
    render(
      <RemoveExtensionDialog
        canRemovePermanently={canRemovePermanently}
        extension={extension}
        onOpenChange={vi.fn()}
        open
      />
    );

    expect(screen.queryByRole("radiogroup")).not.toBeInTheDocument();
    expect(screen.queryByTestId("remove-extension-choice-permanent")).not.toBeInTheDocument();
  });

  it("Should show a server refusal inline in the dialog", () => {
    mocks.removeError = new Error(
      'otel-bridge has source "bundled" but the request was refused: permanent removal is not available'
    );
    render(
      <RemoveExtensionDialog
        canRemovePermanently
        extension={bundledRow}
        onOpenChange={vi.fn()}
        open
      />
    );

    expect(screen.getByText(/permanent removal is not available/)).toBeInTheDocument();
  });

  it("Should start a reopened dialog from the non-permanent default", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    const { rerender } = render(
      <RemoveExtensionDialog
        canRemovePermanently
        extension={bundledRow}
        onOpenChange={onOpenChange}
        open
      />
    );

    await user.click(screen.getByTestId("remove-extension-choice-permanent"));
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onOpenChange).toHaveBeenCalledWith(false);

    rerender(
      <RemoveExtensionDialog
        canRemovePermanently
        extension={bundledRow}
        onOpenChange={onOpenChange}
        open={false}
      />
    );
    rerender(
      <RemoveExtensionDialog
        canRemovePermanently
        extension={bundledRow}
        onOpenChange={onOpenChange}
        open
      />
    );
    expect(screen.getByTestId("remove-extension-choice-remove")).toHaveAttribute(
      "aria-checked",
      "true"
    );
  });
});

describe("RemoveExtensionDialog", () => {
  it("Should show declared permissions without gating removal on unrelated state", async () => {
    const user = userEvent.setup();
    render(<RemoveExtensionDialog extension={extensionFixtures[1]!} onOpenChange={vi.fn()} open />);

    expect(screen.getByRole("note")).toHaveTextContent(
      "Permissions removed: gateway/status, sessions/list"
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
      expect(mocks.remove).toHaveBeenCalledWith({
        dev: false,
        name: "otel-bridge",
        profileName: "default",
      })
    );
    expect(onOpenChange).toHaveBeenCalledWith(false);
    expect(onRemoved).toHaveBeenCalledOnce();
  });

  it("Should unlink a workspace dev overlay without promising global removal", async () => {
    const user = userEvent.setup();
    const extension = {
      ...extensionFixtures[0]!,
      dev: true,
      profile: "engineering",
      workspace_id: "workspace-dev",
    };
    render(<RemoveExtensionDialog extension={extension} onOpenChange={vi.fn()} open />);

    expect(screen.getByText(/published installation stays in place/i)).toBeInTheDocument();
    await user.type(screen.getByLabelText("Type to confirm"), extension.name);
    const confirm = screen.getByRole("button", { name: "Unlink local copy" });
    expect(confirm).toBeEnabled();
    await user.click(confirm);

    await waitFor(() =>
      expect(mocks.remove).toHaveBeenCalledWith({
        dev: true,
        name: extension.name,
        profileName: "engineering",
        workspaceId: "workspace-dev",
      })
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

describe("ExtensionGatewayConfirmDialog", () => {
  it("Should show the exact digest and preserve the update callback", async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn();
    render(
      <ExtensionGatewayConfirmDialog
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
    expect(screen.getByText(/Updating dep-kit-ops/)).toBeInTheDocument();
    expect(screen.getByTestId("extension-gateway-confirm-digest")).toHaveTextContent(
      "sha256:6f1c0a94d3b27e58"
    );
    await user.click(screen.getByTestId("extension-gateway-confirm-accept"));
    expect(onConfirm).toHaveBeenCalledOnce();
  });
});

describe("VerifiedMark", () => {
  it("Should expose a verified checksum signal only when verification is true", () => {
    const { rerender } = render(<VerifiedMark verified />);
    expect(screen.getByLabelText("Checksum verified")).toBeInTheDocument();

    rerender(<VerifiedMark verified={false} />);
    expect(screen.queryByLabelText("Checksum verified")).not.toBeInTheDocument();
  });
});
