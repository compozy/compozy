// Suite: Settings → Computers route page composition (S8 / U1, U2, U7)
// Invariant: ComputersSettingsPage renders loading/error, cloud-unreachable + uninstall banners,
// This computer controls, Computers list, On your account, Add computer dialog, and Details panel.
import { fireEvent, render, screen } from "@testing-library/react";
import type { PropsWithChildren } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const state = vi.hoisted(() => ({
  view: {} as Record<string, unknown>,
  topbarSpy: vi.fn(),
}));

vi.mock("@/systems/computers", async () => {
  const actual = await vi.importActual<typeof import("@/systems/computers")>("@/systems/computers");
  return {
    ...actual,
    useComputersSettingsPage: () => state.view,
  };
});

vi.mock("@/systems/settings", async () => {
  const actual = await vi.importActual<typeof import("@/systems/settings")>("@/systems/settings");
  return {
    ...actual,
    SettingsPageFrame: ({ children }: PropsWithChildren) => <main>{children}</main>,
    useSettingsTopbar: (opts: unknown) => state.topbarSpy(opts),
  };
});

import {
  ACCOUNT_DEVICES,
  ACCOUNT_PRESETS,
  ACCOUNT_UNADDED_COMPUTERS,
  COMPUTERS_PRO,
  LOCAL_COMPUTER,
} from "@/systems/computers/mocks/remote-computers-fixtures";
import { ComputersSettingsPage } from "../-computers-settings-page";

function makeView(overrides: Record<string, unknown> = {}) {
  return {
    account: ACCOUNT_PRESETS.pro,
    local: LOCAL_COMPUTER,
    computers: COMPUTERS_PRO,
    unaddedComputers: ACCOUNT_UNADDED_COMPUTERS,
    localDevices: ACCOUNT_DEVICES,
    localPendingApprovals: [],
    currentDeviceIsAdmin: true,
    currentDeviceName: "MacBook Pro",
    cloudUnreachable: false,
    uninstallWarning: null,
    setUninstallWarning: vi.fn(),
    isLoading: false,
    error: null,
    addDialogOpen: false,
    setAddDialogOpen: vi.fn(),
    addComputer: {
      sshSuggestions: [],
      unaddedComputers: ACCOUNT_UNADDED_COMPUTERS,
      controller: {
        method: "ssh",
        setMethod: vi.fn(),
        draft: { host: "", user: "", port: "22", name: "" },
        setDraft: vi.fn(),
        sshStage: "picker",
        setupStatus: null,
        hostKeyPrompt: null,
        secretCountdownSeconds: 180,
        secretError: null,
        isAgentInitiated: false,
        linkedCommitReached: false,
        addedIds: [],
        detectedCommandComputer: null,
        onStartSsh: vi.fn(),
        onTrustHost: vi.fn(),
        onDeclineHost: vi.fn(),
        onSubmitSecret: vi.fn(),
        onConfirmCancel: vi.fn(),
        onRetrySetup: vi.fn(),
        onAddAccountComputer: vi.fn(),
        onBackToPicker: vi.fn(),
        onRemoveLinked: vi.fn(),
      },
    },
    selectedComputer: null,
    detailsTab: "status",
    openDetails: vi.fn(),
    closeDetails: vi.fn(),
    removeTarget: null,
    setRemoveTarget: vi.fn(),
    isRemoving: false,
    renameTarget: null,
    setRenameTarget: vi.fn(),
    isRenaming: false,
    revokeTarget: null,
    setRevokeTarget: vi.fn(),
    revokeApprovalFailure: null,
    isRevoking: false,
    closeRevokeDialog: vi.fn(),
    disableAgentsOpen: false,
    setDisableAgentsOpen: vi.fn(),
    addingAccountId: null,
    liveUpdateState: "idle",
    liveUpdateSteps: undefined,
    handleRequestToggleRunAgents: vi.fn(),
    handleConfirmDisableAgents: vi.fn(),
    handleToggleRemoteEnabled: vi.fn(),
    handleToggleRequireApproval: vi.fn(),
    handleToggleLocalDeviceScope: vi.fn(),
    handleApproveLocalPending: vi.fn(),
    handleDenyLocalPending: vi.fn(),
    handleConfirmRevokeDevice: vi.fn(),
    handleRetryEnableApproval: vi.fn(),
    handleToggleEnabled: vi.fn(),
    handleConfirmRename: vi.fn(),
    handleConfirmRemove: vi.fn(),
    handleRemoveRoute: vi.fn(),
    handleAddAccountComputer: vi.fn(),
    handleRetryConnection: vi.fn(),
    handleToggleComputerScope: vi.fn(),
    handleStartUpdate: vi.fn(),
    handleCheckVersion: vi.fn(),
    handleRetryCloud: vi.fn(),
    ...overrides,
  };
}

describe("ComputersSettingsPage", () => {
  beforeEach(() => {
    state.topbarSpy.mockReset();
    state.view = makeView();
  });

  it("renders This computer, Computers list, and On your account sections", () => {
    render(<ComputersSettingsPage />);

    expect(screen.getByTestId("computers-this-computer")).toBeInTheDocument();
    expect(screen.getByTestId("computers-list")).toBeInTheDocument();
    expect(screen.getByTestId("computers-on-account")).toBeInTheDocument();
    expect(screen.getByText("devbox")).toBeInTheDocument();
    expect(state.topbarSpy).toHaveBeenCalled();
  });

  it("renders cloud unreachable and uninstall partial-failure banners with actions", () => {
    const retry = vi.fn();
    const dismiss = vi.fn();
    state.view = makeView({
      cloudUnreachable: true,
      uninstallWarning: "Run compozy uninstall on devbox.",
      handleRetryCloud: retry,
      setUninstallWarning: dismiss,
    });

    render(<ComputersSettingsPage />);

    expect(screen.getByTestId("computers-cloud-unreachable")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Status/i })).toHaveAttribute(
      "href",
      "https://status.compozy.com"
    );
    fireEvent.click(screen.getByRole("button", { name: /Try again/i }));
    expect(retry).toHaveBeenCalledTimes(1);

    expect(screen.getByTestId("computers-uninstall-warning")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Dismiss" }));
    expect(dismiss).toHaveBeenCalledWith(null);
  });

  it("renders empty state when no remote computers are added", () => {
    const openAdd = vi.fn();
    state.view = makeView({
      computers: [],
      unaddedComputers: [],
      setAddDialogOpen: openAdd,
    });

    render(<ComputersSettingsPage />);

    expect(screen.getByTestId("computers-empty")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Add computer/i }));
    expect(openAdd).toHaveBeenCalledWith(true);
  });
});
