// Suite: U0 SignInScreen, useSignInPage, GateGraceBannerView, AccountSignInNoticesSection,
//        and Account nav badge (S15 / U0, U3)
// Test IDs: UT-120, UT-138
// Invariant: Every U0 state, link, error, rate-limit countdown, grace banner variant, and U3
//            sign-in notice + nav badge renders exact spec copy and actions.

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { computerQueryKey, LOCAL_COMPUTER_ID } from "@/lib/computer-scope";
import { computerRegistryKeys } from "@/systems/computers/lib/query-keys";
import {
  AccountApiError,
  type AccountSignInStreamHandlers,
  type AccountStatusPayload,
} from "../../adapters/account-api";
import { accountKeys } from "../../lib/query-keys";
import {
  COMPOZY_PRIVACY_URL,
  COMPOZY_STATUS_URL,
  COMPOZY_SUPPORT_MAILTO,
  COMPOZY_TERMS_URL,
  COMPOZY_WHY_SIGN_IN_URL,
} from "../../lib/account-view";
import { resolveSignInView } from "../../lib/sign-in-gate";
import { useSignInPage } from "../../hooks/use-sign-in-page";
import { AccountSignInNoticesSection } from "../account-sign-in-notices-section";
import { GateGraceBannerView } from "../gate-grace-banner";
import { SignInScreen } from "../sign-in-screen";
import { SettingsNavItemContent } from "@/systems/os/apps/settings/settings-nav-parts";
import { findSettingsSection } from "@/systems/settings/lib/sections";

const mocks = vi.hoisted(() => ({
  fetchAccountStatus: vi.fn(),
  startAccountLogin: vi.fn(),
  cancelAccountLogin: vi.fn(),
  openAccountSignInStream: vi.fn(),
  unlinkRemoteHost: vi.fn(),
  navigate: vi.fn(),
}));

vi.mock("@tanstack/react-router", async importOriginal => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    useRouter: () => ({
      navigate: mocks.navigate,
      state: { location: { pathname: "/sign-in", href: "/sign-in" } },
    }),
  };
});

vi.mock("../../adapters/account-api", async importOriginal => {
  const actual = await importOriginal<typeof import("../../adapters/account-api")>();
  return {
    ...actual,
    fetchAccountStatus: mocks.fetchAccountStatus,
    startAccountLogin: mocks.startAccountLogin,
    cancelAccountLogin: mocks.cancelAccountLogin,
    openAccountSignInStream: mocks.openAccountSignInStream,
  };
});

vi.mock("@/systems/computers/adapters/remote-host-api", async importOriginal => {
  const actual =
    await importOriginal<typeof import("@/systems/computers/adapters/remote-host-api")>();
  return {
    ...actual,
    unlinkRemoteHost: mocks.unlinkRemoteHost,
  };
});

function makeStatus(overrides: Partial<AccountStatusPayload> = {}): AccountStatusPayload {
  return {
    gate: "enforced",
    grace: null,
    state: "signed_out",
    source: null,
    signed_out_reason: null,
    pre_account_install: false,
    installation_linked: false,
    token_name: null,
    signed_in: false,
    email: "",
    name: "",
    provider: "",
    device: null,
    marketing_opt_in: false,
    plan: "free",
    subscription: null,
    remote_computers_used: 0,
    remote_computers_limit: 1,
    cloud_routes_used: 0,
    cloud_routes_limit: 0,
    payment_issue: false,
    recent_sign_ins: [],
    ...overrides,
  };
}

function LiveSignInHarness({ redirectTo }: { redirectTo?: string }) {
  const page = useSignInPage({ redirectTo });
  return (
    <SignInScreen
      buttonsDisabled={page.view.buttonsDisabled}
      isUnlinking={page.isUnlinking}
      marketingOptIn={page.marketingOptIn}
      notice={page.view.notice}
      onBackToApp={page.handleBackToApp}
      onCancelWaiting={page.handleCancelWaiting}
      onMarketingOptInChange={page.setMarketingOptIn}
      onReopenBrowser={page.handleReopenBrowser}
      onRetryConnection={page.handleRetryConnection}
      onSelectProvider={page.handleSelectProvider}
      onUnlinkInstallation={page.handleUnlinkInstallation}
      subtitle={page.view.subtitle}
      title={page.view.title}
      tokenRevoked={page.view.tokenRevoked}
      waiting={page.waiting}
    />
  );
}

describe("U0 SignInScreen states and legal/help links (UT-120, UT-138)", () => {
  let streamHandlers: AccountSignInStreamHandlers | null = null;

  beforeEach(() => {
    vi.spyOn(window, "open").mockReturnValue(null);
    streamHandlers = null;
    mocks.fetchAccountStatus.mockReset();
    mocks.startAccountLogin.mockReset();
    mocks.cancelAccountLogin.mockReset();
    mocks.cancelAccountLogin.mockResolvedValue(undefined);
    mocks.openAccountSignInStream.mockReset();
    mocks.openAccountSignInStream.mockImplementation((handlers: AccountSignInStreamHandlers) => {
      streamHandlers = handlers;
      return () => {
        streamHandlers = null;
      };
    });
    mocks.unlinkRemoteHost.mockReset();
    mocks.unlinkRemoteHost.mockResolvedValue(undefined);
    mocks.navigate.mockReset();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("UT-120 & UT-138: renders first-run copy, unchecked consent, Terms, Privacy, and Why sign in? links", () => {
    const view = resolveSignInView(makeStatus());
    render(<SignInScreen subtitle={view.subtitle} title={view.title} />);

    expect(screen.getByRole("heading", { name: "Sign in to CompozyOS" })).toBeInTheDocument();
    expect(screen.getByText("Run coding agents on all your computers.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Continue with Google/i })).toBeEnabled();
    expect(screen.getByRole("button", { name: /Continue with GitHub/i })).toBeEnabled();
    expect(
      screen.getByRole("checkbox", { name: /Email me product news and offers/i })
    ).not.toBeChecked();

    expect(screen.getByRole("link", { name: "Terms" })).toHaveAttribute("href", COMPOZY_TERMS_URL);
    expect(screen.getByRole("link", { name: "Privacy Policy" })).toHaveAttribute(
      "href",
      COMPOZY_PRIVACY_URL
    );
    expect(screen.getByRole("link", { name: /Why sign in\?/i })).toHaveAttribute(
      "href",
      COMPOZY_WHY_SIGN_IN_URL
    );
  });

  it("UT-120: renders pre_account_install copy and every reauth_required reason including account_suspended and token_revoked", () => {
    const preAccount = resolveSignInView(makeStatus({ pre_account_install: true }));
    const { rerender } = render(
      <SignInScreen subtitle={preAccount.subtitle} title={preAccount.title} />
    );
    expect(
      screen.getByRole("heading", { name: /Sign in to keep using CompozyOS/i })
    ).toBeInTheDocument();
    expect(
      screen.getByText("Your sessions, projects and settings are still here.")
    ).toBeInTheDocument();

    const reasons: Array<[string, RegExp]> = [
      ["device_revoked", /This computer was removed from your account\. Sign in again\./],
      ["account_deleted", /Your account was deleted\./],
      ["session_invalid", /Your session ended\. Sign in again\./],
      ["computer_unlinked", /This computer was unlinked from your account\./],
    ];
    for (const [reason, expected] of reasons) {
      const view = resolveSignInView(
        makeStatus({ state: "reauth_required", signed_out_reason: reason })
      );
      rerender(<SignInScreen notice={view.notice} title={view.title} />);
      expect(screen.getByRole("heading", { name: "Sign in again" })).toBeInTheDocument();
      expect(screen.getByTestId("sign-in-notice")).toHaveTextContent(expected);
    }

    // token_revoked hides provider buttons and renders CLI hint.
    const tokenView = resolveSignInView(
      makeStatus({ state: "reauth_required", signed_out_reason: "token_revoked" })
    );
    rerender(
      <SignInScreen
        notice={tokenView.notice}
        title={tokenView.title}
        tokenRevoked={tokenView.tokenRevoked}
      />
    );
    expect(screen.getByText(/This account token was revoked\./)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Continue with Google/i })).not.toBeInTheDocument();
    expect(screen.getByText(/compozy account login/)).toBeInTheDocument();

    // UT-138: account_suspended shows mailto:support@compozy.com link and keeps provider buttons.
    const suspendedView = resolveSignInView(
      makeStatus({ state: "reauth_required", signed_out_reason: "account_suspended" })
    );
    rerender(
      <SignInScreen
        buttonsDisabled={suspendedView.buttonsDisabled}
        notice={suspendedView.notice}
        title={suspendedView.title}
      />
    );
    expect(screen.getByRole("heading", { name: "This account is suspended" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "support@compozy.com" })).toHaveAttribute(
      "href",
      COMPOZY_SUPPORT_MAILTO
    );
    expect(screen.getByRole("button", { name: /Continue with Google/i })).toBeEnabled();
    expect(screen.getByRole("button", { name: /Continue with GitHub/i })).toBeEnabled();
  });

  it("UT-120: handles waiting, Cancel, expired, cloud_unreachable (with Status link), and installation_linked_to_other_account", async () => {
    mocks.fetchAccountStatus.mockResolvedValue(makeStatus());
    mocks.startAccountLogin.mockResolvedValue({
      attempt_id: "att_1",
      user_code: "WDJB-MJHT",
      verification_uri: "https://cloud.compozy.com/activate",
      verification_uri_complete:
        "https://cloud.compozy.com/activate?user_code=WDJB-MJHT&provider=google",
      expires_at: "2026-11-24T19:00:00Z",
    });

    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={queryClient}>
        <LiveSignInHarness redirectTo="/sessions" />
      </QueryClientProvider>
    );

    fireEvent.click(await screen.findByRole("button", { name: /Continue with Google/i }));
    expect(await screen.findByText("WDJB-MJHT")).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent("Waiting for sign-in in your browser");

    // Cancel returns to buttons with cancelled notice.
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(mocks.cancelAccountLogin).toHaveBeenCalledWith("att_1");
    expect(screen.getByText("Sign-in was cancelled.")).toBeInTheDocument();

    // SSE expired event renders timed-out copy.
    await act(async () => {
      streamHandlers?.onProgress?.({
        attempt_id: "att_2",
        state: "expired",
        code: "account_login_expired",
      });
    });
    expect(screen.getByText("Sign-in timed out. Try again.")).toBeInTheDocument();

    // SSE installation_linked_to_other_account renders message and Unlink button.
    await act(async () => {
      streamHandlers?.onProgress?.({
        attempt_id: "att_3",
        state: "failed",
        code: "installation_linked_to_other_account",
      });
    });
    const unlinkButton = screen.getByRole("button", { name: "Unlink" });
    fireEvent.click(unlinkButton);
    await waitFor(() => expect(mocks.unlinkRemoteHost).toHaveBeenCalledTimes(1));

    // Cloud unreachable disables provider buttons and shows Status link.
    mocks.startAccountLogin.mockRejectedValueOnce(
      new AccountApiError("Can't reach Compozy Cloud.", 503, "account_cloud_unreachable")
    );
    fireEvent.click(screen.getByRole("button", { name: /Continue with GitHub/i }));
    expect(
      await screen.findByText("Can't reach CompozyOS. Check your connection.")
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Status/i })).toHaveAttribute(
      "href",
      COMPOZY_STATUS_URL
    );
    expect(screen.getByRole("button", { name: /Continue with Google/i })).toBeDisabled();
  });

  it("UT-138: rate_limited disables provider buttons until Retry-After elapses", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-11-24T12:00:00Z"));
    mocks.fetchAccountStatus.mockResolvedValue(makeStatus());
    mocks.startAccountLogin.mockRejectedValueOnce(
      new AccountApiError("Too many sign-in attempts.", 429, "rate_limited", 120)
    );

    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={queryClient}>
        <LiveSignInHarness />
      </QueryClientProvider>
    );

    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    fireEvent.click(screen.getByRole("button", { name: /Continue with Google/i }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });

    expect(
      screen.getByText("Too many sign-in attempts. Try again in 2 minutes.")
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Continue with Google/i })).toBeDisabled();
    expect(screen.getByRole("button", { name: /Continue with GitHub/i })).toBeDisabled();

    // Advance past the 120-second Retry-After window.
    await act(async () => {
      vi.advanceTimersByTime(121_000);
    });

    expect(screen.getByRole("button", { name: /Continue with Google/i })).toBeEnabled();
    expect(screen.getByRole("button", { name: /Continue with GitHub/i })).toBeEnabled();
  });

  it("UT-138: GateGraceBannerView renders offline variant with Try again and Status and no dismiss", () => {
    const onTryAgain = vi.fn();
    render(
      <GateGraceBannerView
        grace={{ source: "offline", until: "2026-12-03T14:00:00Z", message: "" }}
        onTryAgain={onTryAgain}
      />
    );

    expect(
      screen.getByText(
        /Can't reach CompozyOS to sign in\. CompozyOS keeps working until Dec 3, 14:00\./
      )
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Try again/i }));
    expect(onTryAgain).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("link", { name: /Status/i })).toHaveAttribute(
      "href",
      COMPOZY_STATUS_URL
    );
    expect(screen.queryByRole("button", { name: /Dismiss/i })).not.toBeInTheDocument();
  });

  it("UT-138: Account nav item renders warning badge with recent sign-in count and notices dismiss cleanly", () => {
    const accountSection = findSettingsSection("account")!;
    const { rerender } = render(
      <div>
        <SettingsNavItemContent
          badge={{ count: 2, tone: "warning" }}
          isActive={false}
          section={accountSection}
        />
      </div>
    );
    expect(screen.getByTestId("settings-section-badge-account")).toHaveTextContent("2");

    rerender(
      <div>
        <SettingsNavItemContent isActive={false} section={accountSection} />
      </div>
    );
    expect(screen.queryByTestId("settings-section-badge-account")).not.toBeInTheDocument();

    const onDismiss = vi.fn();
    const onRevoke = vi.fn();
    render(
      <AccountSignInNoticesSection
        currentDeviceIsAdmin
        notices={[
          {
            deviceId: "dev_a",
            name: "iPhone",
            kind: "browser",
            location: "Lisbon, PT",
            at: "2026-11-23T10:00:00Z",
            whenLabel: "1d ago",
          },
          {
            deviceId: "dev_b",
            name: "Linux CLI",
            kind: "cli",
            location: "Berlin, DE",
            at: "2026-11-24T08:00:00Z",
            whenLabel: "2h ago",
          },
        ]}
        onDismiss={onDismiss}
        onRevokeNotice={onRevoke}
      />
    );

    expect(screen.getByText("New sign-in on iPhone")).toBeInTheDocument();
    expect(screen.getByText("New sign-in on Linux CLI")).toBeInTheDocument();
    fireEvent.click(screen.getAllByRole("button", { name: "Revoke this device" })[0]!);
    expect(onRevoke).toHaveBeenCalledWith(expect.objectContaining({ deviceId: "dev_a" }));
    fireEvent.click(screen.getByRole("button", { name: "Dismiss" }));
    expect(onDismiss).toHaveBeenCalledTimes(1);
  });

  // Regression test for AST-010: completing sign-in on /sign-in must cancel and remove all prior
  // account- and computer-scoped queries and seed only the newly confirmed account status.
  it("AST-010: evicts previous account devices, approvals, tokens, and remote queries on sign-in completion", async () => {
    const signedOut = makeStatus({ state: "signed_out", signed_in: false });
    const accountB = makeStatus({
      state: "signed_in",
      signed_in: true,
      source: "session",
      email: "b@example.test",
      name: "Account B",
      device: { id: "dev_b", name: "MacBook B", admin: true },
    });
    mocks.fetchAccountStatus.mockResolvedValueOnce(signedOut).mockResolvedValueOnce(accountB);

    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    queryClient.setQueryData(accountKeys.devices(), { devices: [{ id: "dev_a" }] });
    queryClient.setQueryData(accountKeys.approvals(), { approvals: [{ id: "apr_a" }] });
    queryClient.setQueryData(accountKeys.tokens(), { tokens: [{ id: "atk_a" }] });
    queryClient.setQueryData(computerRegistryKeys.list(), { computers: [{ id: "C1" }] });
    queryClient.setQueryData(computerQueryKey("C1", "sessions"), ["remote-c1"]);
    queryClient.setQueryData(computerQueryKey(LOCAL_COMPUTER_ID, "sessions"), ["local-sess"]);

    render(
      <QueryClientProvider client={queryClient}>
        <LiveSignInHarness redirectTo="/settings/account" />
      </QueryClientProvider>
    );

    await waitFor(() => expect(streamHandlers).not.toBeNull());
    await act(async () => {
      streamHandlers?.onState?.({ state: "signed_in", signed_out_reason: null });
    });

    await waitFor(() => {
      expect(queryClient.getQueryData(accountKeys.status())).toEqual(accountB);
    });
    expect(queryClient.getQueryData(accountKeys.devices())).toBeUndefined();
    expect(queryClient.getQueryData(accountKeys.approvals())).toBeUndefined();
    expect(queryClient.getQueryData(accountKeys.tokens())).toBeUndefined();
    expect(queryClient.getQueryData(computerRegistryKeys.list())).toBeUndefined();
    expect(queryClient.getQueryData(computerQueryKey("C1", "sessions"))).toBeUndefined();
    expect(queryClient.getQueryData(computerQueryKey(LOCAL_COMPUTER_ID, "sessions"))).toEqual([
      "local-sess",
    ]);
  });
});
