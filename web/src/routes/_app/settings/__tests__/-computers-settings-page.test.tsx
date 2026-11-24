// Suite: Settings → Computers route page composition (S8 / U1, U2, U7)
// Invariant: ComputersSettingsPage renders loading/error, cloud-unreachable + uninstall banners,
// This computer controls, Computers list, On your account, Add computer dialog, and Details panel.
import { fireEvent, render, screen, within } from "@testing-library/react";
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
  ACCOUNT_UNADDED_FREE,
  COMPUTERS_DOWNGRADED,
  COMPUTERS_PRO,
  LOCAL_COMPUTER,
} from "@/systems/computers/mocks/remote-computers-fixtures";
import { ComputersSettingsPage } from "../-computers-settings-page";

function makeView(overrides: Record<string, unknown> = {}) {
  return {
    account: ACCOUNT_PRESETS.pro,
    isGraceMode: false,
    handleGoToSignIn: vi.fn(),
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
    isUpdatingLocalSettings: false,
    loadBalancingEnabled: false,
    isUpdatingLoadBalancing: false,
    addingAccountId: null,
    liveUpdateState: "idle",
    liveUpdateSteps: undefined,
    handleRequestToggleRunAgents: vi.fn(),
    handleConfirmDisableAgents: vi.fn(),
    handleToggleLoadBalancing: vi.fn(),
    handleChangeLocalWeight: vi.fn(),
    handleChangeComputerWeight: vi.fn(),
    handleToggleRemoteEnabled: vi.fn(),
    handleToggleHostedRoute: vi.fn(),
    handleToggleRequireApproval: vi.fn(),
    handleOpenUpgrade: vi.fn(),
    handleOpenUpgradeFromAdd: vi.fn(),
    fallbackUpgradeSheet: null,
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
    expect(within(screen.getByTestId("computers-list")).getByText("devbox")).toBeInTheDocument();
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

  it("renders empty state when no remote computers are added and disables local execution switch", () => {
    const openAdd = vi.fn();
    state.view = makeView({
      computers: [],
      unaddedComputers: [],
      setAddDialogOpen: openAdd,
    });

    render(<ComputersSettingsPage />);

    expect(screen.getByTestId("computers-empty")).toBeInTheDocument();
    expect(screen.getByRole("switch", { name: "Run agents on this computer" })).toHaveAttribute(
      "aria-disabled",
      "true"
    );
    expect(screen.getByText(/Add a computer first/i)).toBeInTheDocument();
    expect(screen.queryByTestId("computers-load-balancing")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Add computer/i }));
    expect(openAdd).toHaveBeenCalledWith(true);
  });

  it("renders Load balancing section and Stop running agents confirmation dialog", () => {
    const onConfirmDisable = vi.fn();
    state.view = makeView({
      loadBalancingEnabled: true,
      disableAgentsOpen: true,
      handleConfirmDisableAgents: onConfirmDisable,
    });

    render(<ComputersSettingsPage />);

    expect(screen.getByTestId("computers-load-balancing")).toBeInTheDocument();
    expect(screen.getByText("Stop running agents on this computer?")).toBeInTheDocument();
    expect(screen.getByText(/2 sessions are running here/i)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Stop running agents" }));
    expect(onConfirmDisable).toHaveBeenCalledTimes(1);
  });

  it("wires every S12 paywall surface on Free/downgraded and Hosted route switch on Pro", () => {
    const openUpgrade = vi.fn();
    const openUpgradeFromAdd = vi.fn();
    const toggleHostedRoute = vi.fn();

    // Pro shows interactive Hosted route switch.
    state.view = makeView({
      account: ACCOUNT_PRESETS.pro,
      handleToggleHostedRoute: toggleHostedRoute,
    });
    const { rerender } = render(<ComputersSettingsPage />);
    fireEvent.click(screen.getByRole("switch", { name: "Hosted route" }));
    expect(toggleHostedRoute).toHaveBeenCalledTimes(1);

    // Free / downgraded wires Hosted route paywall, Free slot meter, Needs Pro rows, and On your account paywall.
    state.view = makeView({
      account: ACCOUNT_PRESETS.downgraded,
      computers: COMPUTERS_DOWNGRADED,
      unaddedComputers: ACCOUNT_UNADDED_FREE,
      handleOpenUpgrade: openUpgrade,
      handleOpenUpgradeFromAdd: openUpgradeFromAdd,
      addDialogOpen: true,
      addComputer: {
        sshSuggestions: [],
        unaddedComputers: ACCOUNT_UNADDED_FREE,
        controller: {
          method: "ssh",
          setMethod: vi.fn(),
          draft: { host: "testhost2", user: "pedro", port: "22", name: "testhost2" },
          setDraft: vi.fn(),
          sshStage: "plan_limit",
          setupStatus: {
            setup_id: "stp_limit",
            computer_id: null,
            state: "failed",
            step: "admit",
            prompt: null,
            error: {
              code: "plan_limit_reached",
              error: "Free includes 1 remote computer. Upgrade to Pro for unlimited computers.",
            },
          },
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
    });
    rerender(<ComputersSettingsPage />);

    expect(screen.getByText(/Hosted route is part of Pro/i)).toBeInTheDocument();
    expect(screen.getByText("1 active · 2 need Pro")).toBeInTheDocument();
    expect(
      screen.getByText("This computer can only be reached through the hosted route (Pro)")
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Upgrade to Pro" }));
    expect(openUpgradeFromAdd).toHaveBeenCalledTimes(1);

    state.view = makeView({
      account: ACCOUNT_PRESETS.downgraded,
      computers: COMPUTERS_DOWNGRADED,
      unaddedComputers: ACCOUNT_UNADDED_FREE,
      handleOpenUpgrade: openUpgrade,
      handleOpenUpgradeFromAdd: openUpgradeFromAdd,
      addDialogOpen: false,
    });
    rerender(<ComputersSettingsPage />);

    const upgradeButtons = screen.getAllByRole("button", { name: /^Upgrade$/i });
    expect(upgradeButtons.length).toBeGreaterThanOrEqual(4);
    fireEvent.click(upgradeButtons[0]!);
    expect(openUpgrade).toHaveBeenCalledTimes(1);
  });

  it("UT-138: renders Sign in to use this prompt instead of computers list during grace mode", () => {
    const goToSignIn = vi.fn();
    state.view = makeView({
      account: ACCOUNT_PRESETS.signed_out,
      isGraceMode: true,
      handleGoToSignIn: goToSignIn,
    });

    render(<ComputersSettingsPage />);

    expect(screen.getByTestId("computers-grace-sign-in-prompt")).toHaveTextContent(
      "Sign in to use this."
    );
    expect(screen.queryByTestId("computers-list")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Sign in/i }));
    expect(goToSignIn).toHaveBeenCalledTimes(1);
  });
});
