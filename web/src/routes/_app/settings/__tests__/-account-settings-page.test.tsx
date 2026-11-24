// Suite: Settings → Account route page composition (S8 + S12 / U3, U9)
// Invariant: AccountSettingsPage renders profile, plan & billing controls (Upgrade on Free,
//            Manage billing on Stripe Pro, absent Manage billing on complimentary Pro, past_due
//            payment issue banner, downgraded needs_pro usage), pending approvals, devices with
//            admin promotion & Q3 revoke dialog, sign-out dialog, and signed-out / link-only /
//            account-token states cleanly.
import { fireEvent, render, screen } from "@testing-library/react";
import type { PropsWithChildren } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const state = vi.hoisted(() => ({
  view: {} as Record<string, unknown>,
}));

vi.mock("@/systems/account", async () => {
  const actual = await vi.importActual<typeof import("@/systems/account")>("@/systems/account");
  return {
    ...actual,
    useAccountSettingsPage: () => state.view,
  };
});

vi.mock("@/systems/settings", async () => {
  const actual = await vi.importActual<typeof import("@/systems/settings")>("@/systems/settings");
  return {
    ...actual,
    SettingsPageFrame: ({ children }: PropsWithChildren) => <main>{children}</main>,
    useSettingsTopbar: vi.fn(),
  };
});

import {
  ACCOUNT_DEVICES,
  ACCOUNT_PRESETS,
  PENDING_APPROVALS,
} from "@/systems/computers/mocks/remote-computers-fixtures";
import { AccountSettingsPage } from "../-account-settings-page";

function makeView(overrides: Record<string, unknown> = {}) {
  return {
    account: ACCOUNT_PRESETS.pro,
    isGraceMode: false,
    devices: ACCOUNT_DEVICES,
    currentDevice: ACCOUNT_DEVICES[0],
    currentDeviceIsAdmin: true,
    approvals: PENDING_APPROVALS,
    recentSignIns: [],
    tokens: [],
    downgradedNeedsProCount: 0,
    localRemoteEnabled: true,
    requireDeviceApproval: false,
    isLoading: false,
    error: null,
    loginAttempt: null,
    loginError: null,
    linkMismatchNotice: false,
    isStartingLogin: false,
    revokeTarget: null,
    revokeApprovalFailure: null,
    isRevoking: false,
    signOutOpen: false,
    signOutRequiresUnlink: true,
    signOutError: null,
    isSigningOut: false,
    isOpeningBillingPortal: false,
    isUpdatingEmail: false,
    createTokenOpen: false,
    createTokenError: null,
    isCreatingToken: false,
    createdToken: null,
    revokingTokenId: null,
    isDismissingNotices: false,
    deleteOpen: false,
    deleteError: null,
    isDeletingAccount: false,
    fallbackUpgradeSheet: null,
    setRevokeTarget: vi.fn(),
    setSignOutOpen: vi.fn(),
    setCreateTokenOpen: vi.fn(),
    setCreatedToken: vi.fn(),
    setDeleteOpen: vi.fn(),
    handleToggleAdmin: vi.fn(),
    handleConfirmRevoke: vi.fn(),
    handleRetryEnableApproval: vi.fn(),
    handleCloseRevoke: vi.fn(),
    handleApprove: vi.fn(),
    handleDeny: vi.fn(),
    handleConfirmSignOut: vi.fn(),
    handleStartSignIn: vi.fn(),
    handleCancelSignIn: vi.fn(),
    handleUnlinkLocal: vi.fn(),
    handleRetryCloud: vi.fn(),
    handleOpenUpgrade: vi.fn(),
    handleManageBilling: vi.fn(),
    handleToggleMarketingOptIn: vi.fn(),
    handleCreateToken: vi.fn(),
    handleRevokeToken: vi.fn(),
    handleRevokeSignInNotice: vi.fn(),
    handleDismissSignInNotices: vi.fn(),
    handleConfirmDeleteAccount: vi.fn(),
    handleGoToSignIn: vi.fn(),
    ...overrides,
  };
}

describe("AccountSettingsPage", () => {
  beforeEach(() => {
    state.view = makeView();
  });

  it("renders Profile, Plan with Manage billing on Pro, Pending approvals, and Devices for a signed-in session", () => {
    const manageBilling = vi.fn();
    state.view = makeView({ handleManageBilling: manageBilling });

    render(<AccountSettingsPage />);

    expect(screen.getByTestId("account-identity")).toBeInTheDocument();
    expect(screen.getByTestId("account-plan")).toBeInTheDocument();
    expect(screen.getByTestId("account-pending-approvals")).toBeInTheDocument();
    expect(screen.getByTestId("account-devices")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Manage billing/i }));
    expect(manageBilling).toHaveBeenCalledTimes(1);
  });

  it("renders Upgrade on Free plan and hides Manage billing on complimentary Pro", () => {
    const openUpgrade = vi.fn();
    state.view = makeView({
      account: ACCOUNT_PRESETS.free,
      handleOpenUpgrade: openUpgrade,
    });

    const { rerender } = render(<AccountSettingsPage />);
    fireEvent.click(screen.getByRole("button", { name: /Upgrade/i }));
    expect(openUpgrade).toHaveBeenCalledTimes(1);

    state.view = makeView({
      account: {
        ...ACCOUNT_PRESETS.pro,
        subscriptionSource: "complimentary",
        renewsAt: null,
        endsAt: "2027-06-01T00:00:00Z",
      },
    });
    rerender(<AccountSettingsPage />);
    expect(screen.getByText(/complimentary until Jun 1, 2027/i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Manage billing/i })).not.toBeInTheDocument();
  });

  it("renders payment issue banner with Manage billing and downgraded plan-limited banner", () => {
    const manageBilling = vi.fn();
    state.view = makeView({
      account: ACCOUNT_PRESETS.past_due,
      handleManageBilling: manageBilling,
    });

    const { rerender } = render(<AccountSettingsPage />);
    const banner = screen.getByTestId("account-payment-issue");
    expect(banner).toHaveTextContent("Payment failed. Update your card.");
    // Single Manage billing button lives in the banner while past_due.
    const manageButtons = screen.getAllByRole("button", { name: /Manage billing/i });
    expect(manageButtons).toHaveLength(1);
    fireEvent.click(manageButtons[0]!);
    expect(manageBilling).toHaveBeenCalledTimes(1);

    state.view = makeView({
      account: { ...ACCOUNT_PRESETS.free, hostedRouteGraceDaysLeft: null },
      downgradedNeedsProCount: 2,
    });
    rerender(<AccountSettingsPage />);
    expect(screen.getByTestId("account-downgraded-banner")).toHaveTextContent(
      "Pro ended. 1 computer stays active."
    );
    expect(screen.getByText("1 active · 2 need Pro")).toBeInTheDocument();
  });

  it("renders signed-out login buttons and handles Google/GitHub sign-in", () => {
    const startSignIn = vi.fn();
    state.view = makeView({
      account: ACCOUNT_PRESETS.signed_out,
      handleStartSignIn: startSignIn,
    });

    render(<AccountSettingsPage />);

    expect(screen.queryByTestId("account-devices")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Continue with Google/i }));
    expect(startSignIn).toHaveBeenCalledWith("google");
    fireEvent.click(screen.getByRole("button", { name: /Continue with GitHub/i }));
    expect(startSignIn).toHaveBeenCalledWith("github");
  });

  it("renders link-only mismatch notice with Unlink and account-token local-only state", () => {
    const unlink = vi.fn();
    state.view = makeView({
      account: ACCOUNT_PRESETS.link_only_installation,
      linkMismatchNotice: true,
      handleUnlinkLocal: unlink,
    });

    const { rerender } = render(<AccountSettingsPage />);
    expect(screen.getByTestId("account-link-mismatch-notice")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Unlink" }));
    expect(unlink).toHaveBeenCalledTimes(1);

    state.view = makeView({ account: ACCOUNT_PRESETS.token_installation });
    rerender(<AccountSettingsPage />);
    expect(screen.getByText(/Local use only/i)).toBeInTheDocument();
  });

  it("renders Q3 revoke dialog approval failure recovery when local approval fails after revoke", () => {
    const retryApproval = vi.fn();
    state.view = makeView({
      revokeTarget: ACCOUNT_DEVICES[1],
      revokeApprovalFailure: "disk full",
      handleRetryEnableApproval: retryApproval,
    });

    render(<AccountSettingsPage />);

    expect(
      screen.getByText(/Device removed\. Couldn't turn on approval: disk full/i)
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(retryApproval).toHaveBeenCalledTimes(1);
  });

  it("UT-138: renders Email, Account tokens (create + one-time reveal + revoke), Sign-in notices, and Delete account Pro dialog", () => {
    const toggleEmail = vi.fn();
    const dismissNotices = vi.fn();
    const revokeNotice = vi.fn();
    const revokeToken = vi.fn();
    const openDelete = vi.fn();
    state.view = makeView({
      account: {
        ...ACCOUNT_PRESETS.pro,
        subscriptionSource: "stripe",
        subscriptionInterval: "year",
        renewsAt: "2027-03-01T00:00:00Z",
        refundEligibleUntil: "2099-12-04T00:00:00Z",
      },
      recentSignIns: [
        {
          deviceId: "dev_5k1p9z",
          name: "iPhone",
          kind: "browser",
          location: "Lisbon, PT",
          at: "2026-11-23T10:00:00Z",
          whenLabel: "1d ago",
        },
        {
          deviceId: "dev_9x1",
          name: "Work Laptop",
          kind: "desktop",
          location: "Porto, PT",
          at: "2026-11-24T09:00:00Z",
          whenLabel: "2h ago",
        },
      ],
      tokens: [{ id: "atk_1", name: "ci", createdLabel: "", lastUsedLabel: "2h ago" }],
      createdToken: { id: "atk_2", name: "deploy", token: "cpzat_secret_once" },
      deleteOpen: true,
      handleToggleMarketingOptIn: toggleEmail,
      handleDismissSignInNotices: dismissNotices,
      handleRevokeSignInNotice: revokeNotice,
      handleRevokeToken: revokeToken,
      setDeleteOpen: openDelete,
    });

    render(<AccountSettingsPage />);

    expect(screen.getByTestId("account-sign-in-notices")).toBeInTheDocument();
    expect(screen.getByText("New sign-in on iPhone")).toBeInTheDocument();
    expect(screen.getByText("New sign-in on Work Laptop")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Dismiss" }));
    expect(dismissNotices).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole("switch", { name: "Product news and offers" }));
    expect(toggleEmail).toHaveBeenCalledTimes(1);

    expect(screen.getByTestId("account-tokens")).toBeInTheDocument();
    expect(screen.getByTestId("account-token-created-dialog")).toHaveTextContent(
      "cpzat_secret_once"
    );

    const proBlock = screen.getByTestId("account-delete-pro-block");
    expect(proBlock).toHaveTextContent("Deleting your account also cancels Pro.");
    expect(
      screen.getByRole("button", { name: "Delete account and cancel Pro" })
    ).toBeInTheDocument();
  });

  it("UT-138: renders Sign in to use this prompt during grace mode instead of account content", () => {
    const goToSignIn = vi.fn();
    state.view = makeView({
      account: ACCOUNT_PRESETS.signed_out,
      isGraceMode: true,
      handleGoToSignIn: goToSignIn,
    });

    render(<AccountSettingsPage />);

    expect(screen.getByText("Sign in to use this.")).toBeInTheDocument();
    expect(screen.queryByTestId("account-devices")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Sign in/i }));
    expect(goToSignIn).toHaveBeenCalledTimes(1);
  });
});
