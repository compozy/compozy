import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  bundleActivationFixtures,
  extensionFixtures,
  extensionProvenanceFixtures,
} from "../../mocks/fixtures";

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
  DeactivateBundleDialog,
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
  it("Should block removal and offer dependency retry while bundle activity is unresolved", async () => {
    const user = userEvent.setup();
    const onRetryDependencies = vi.fn();
    const { rerender } = render(
      <RemoveExtensionDialog
        activeBundles={undefined}
        dependencyError={null}
        dependencyLoading
        extension={extensionFixtures[0]!}
        onOpenChange={vi.fn()}
        onRetryDependencies={onRetryDependencies}
        open
      />
    );

    expect(screen.getByText("Checking active bundles before removal.")).toBeInTheDocument();
    await user.type(screen.getByLabelText("Type to confirm"), "otel-bridge");
    expect(screen.getByTestId("remove-extension-confirm")).toBeDisabled();

    rerender(
      <RemoveExtensionDialog
        activeBundles={undefined}
        dependencyError={new Error("bundle inventory unavailable")}
        dependencyLoading={false}
        extension={extensionFixtures[0]!}
        onOpenChange={vi.fn()}
        onRetryDependencies={onRetryDependencies}
        open
      />
    );
    expect(screen.getByRole("note")).toHaveTextContent("bundle inventory unavailable");
    await user.click(screen.getByRole("button", { name: "Retry bundle activity" }));
    expect(onRetryDependencies).toHaveBeenCalledOnce();
    expect(screen.getByTestId("remove-extension-confirm")).toBeDisabled();
  });

  it("Should show provenance permissions and keep removal blocked by an active bundle", async () => {
    const user = userEvent.setup();
    render(
      <RemoveExtensionDialog
        activeBundles={bundleActivationFixtures}
        dependencyError={null}
        dependencyLoading={false}
        extension={extensionFixtures[1]!}
        onOpenChange={vi.fn()}
        onRetryDependencies={vi.fn()}
        open
      />
    );

    expect(screen.getByRole("note")).toHaveTextContent(
      "Revoked permissions: sessions.read, network.egress"
    );
    expect(screen.getByRole("note")).toHaveTextContent(
      "The daemon returns 409 while an active bundle depends on this extension"
    );
    await user.type(screen.getByLabelText("Type to confirm"), "slack-notify");
    expect(screen.getByTestId("remove-extension-confirm")).toBeDisabled();
    expect(mocks.remove).not.toHaveBeenCalled();
  });

  it("Should require the exact extension name before invoking the real remove mutation", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    const onRemoved = vi.fn();
    render(
      <RemoveExtensionDialog
        activeBundles={[]}
        dependencyError={null}
        dependencyLoading={false}
        extension={extensionFixtures[0]!}
        onOpenChange={onOpenChange}
        onRemoved={onRemoved}
        onRetryDependencies={vi.fn()}
        open
      />
    );

    const confirm = screen.getByTestId("remove-extension-confirm");
    await user.type(screen.getByLabelText("Type to confirm"), "otel");
    expect(confirm).toBeDisabled();
    await user.type(screen.getByLabelText("Type to confirm"), "-bridge");
    expect(confirm).toBeEnabled();
    await user.click(confirm);
    await waitFor(() => expect(mocks.remove).toHaveBeenCalledWith("otel-bridge"));
    expect(onOpenChange).toHaveBeenCalledWith(false);
    expect(onRemoved).toHaveBeenCalledOnce();
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

describe("DeactivateBundleDialog", () => {
  it("Should disclose daemon failure while preserving the real confirmation callback", async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn().mockResolvedValue(undefined);
    render(
      <DeactivateBundleDialog
        activation={bundleActivationFixtures[0]!}
        error="deactivation rejected"
        onConfirm={onConfirm}
        onOpenChange={vi.fn()}
        open
        pending={false}
      />
    );

    expect(screen.getByText("deactivation rejected")).toBeInTheDocument();
    expect(screen.getByRole("note")).toHaveTextContent("production profile (workspace)");
    await user.click(screen.getByRole("button", { name: "Deactivate" }));
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
