// Suite: Settings → Account route page composition (S8 / U3)
// Invariant: AccountSettingsPage renders profile, plan display without checkout, pending
// approvals, devices with admin promotion & Q3 revoke dialog, sign-out dialog, and signed-out /
// link-only / account-token states cleanly.
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
  ACCOUNT_LINK_ONLY,
  ACCOUNT_PRO,
  ACCOUNT_SIGNED_OUT,
  ACCOUNT_TOKEN_ONLY,
  DEVICES_PRO,
  PENDING_APPROVALS,
} from "@/systems/computers/mocks/remote-computers-fixtures";
import { AccountSettingsPage } from "../-account-settings-page";

function makeView(overrides: Record<string, unknown> = {}) {
  return {
    account: ACCOUNT_PRO,
    devices: DEVICES_PRO,
    currentDevice: DEVICES_PRO[0],
    currentDeviceIsAdmin: true,
    approvals: PENDING_APPROVALS,
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
    signOutError: null,
    isSigningOut: false,
    setRevokeTarget: vi.fn(),
    setSignOutOpen: vi.fn(),
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
    ...overrides,
  };
}

describe("AccountSettingsPage", () => {
  beforeEach(() => {
    state.view = makeView();
  });

  it("renders Profile, Plan without checkout, Pending approvals, and Devices for a signed-in session", () => {
    render(<AccountSettingsPage />);

    expect(screen.getByTestId("account-identity")).toBeInTheDocument();
    expect(screen.getByTestId("account-plan")).toBeInTheDocument();
    expect(screen.getByTestId("account-pending-approvals")).toBeInTheDocument();
    expect(screen.getByTestId("account-devices")).toBeInTheDocument();
    // Checkout/Upgrade button belongs to S12 and must not appear in S8 Plan section.
    expect(screen.queryByRole("button", { name: /Upgrade/i })).not.toBeInTheDocument();
  });

  it("renders signed-out login buttons and handles Google/GitHub sign-in", () => {
    const startSignIn = vi.fn();
    state.view = makeView({
      account: ACCOUNT_SIGNED_OUT,
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
      account: ACCOUNT_LINK_ONLY,
      linkMismatchNotice: true,
      handleUnlinkLocal: unlink,
    });

    const { rerender } = render(<AccountSettingsPage />);
    expect(screen.getByTestId("account-link-mismatch-notice")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Unlink" }));
    expect(unlink).toHaveBeenCalledTimes(1);

    state.view = makeView({ account: ACCOUNT_TOKEN_ONLY });
    rerender(<AccountSettingsPage />);
    expect(screen.getByText(/Local use only/i)).toBeInTheDocument();
  });

  it("renders Q3 revoke dialog approval failure recovery when local approval fails after revoke", () => {
    const retryApproval = vi.fn();
    state.view = makeView({
      revokeTarget: DEVICES_PRO[1],
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
});
