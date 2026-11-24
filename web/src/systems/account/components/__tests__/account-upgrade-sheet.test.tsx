// Suite: Upgrade sheet U9 live checkout & account-stream completion (S12 / US-002)
// Invariant: Annual is preselected with tax line and benefits, Continue to checkout posts the
//            chosen interval and opens Stripe Checkout, account.changed{plan:"pro"} completes to
//            "You're on Pro" even with remote access off, 15m waiting shows "Still waiting? Check
//            again" with truthful manual refresh, and cancelling returns to plan selection with
//            "Checkout was cancelled. Nothing was charged.".
// Boundary IN: AccountUpgradeSheetProvider, UpgradeSheet, useUpgradeSheet.
// Boundary OUT: /api/account/checkout, /api/account, /api/account/stream, window.open.

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import * as accountApi from "../../adapters/account-api";
import {
  AccountUpgradeSheetProvider,
  useOptionalUpgradeSheetActions,
} from "../../contexts/upgrade-sheet-context";
import * as billingWindow from "../../lib/billing-window";
import { accountKeys } from "../../lib/query-keys";
import { accountStatusStoryFixture } from "../../mocks/handlers";

function createQueryClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
}

function makeAccountStatus(overrides: Partial<accountApi.AccountStatusPayload> = {}) {
  return {
    ...accountStatusStoryFixture,
    plan: "free",
    subscription: null,
    remote_computers_used: 1,
    remote_computers_limit: 1,
    cloud_routes_used: 0,
    cloud_routes_limit: 0,
    payment_issue: false,
    ...overrides,
  };
}

function TriggerHarness({
  interval,
  onUpgraded,
}: {
  interval?: "year" | "month";
  onUpgraded?: () => void;
}) {
  const actions = useOptionalUpgradeSheetActions();
  return (
    <button
      onClick={() => actions?.openUpgradeSheet({ interval, onUpgraded })}
      type="button"
    >
      Open upgrade
    </button>
  );
}

function renderSheet(
  queryClient: QueryClient,
  children: ReactNode,
  waitingTimeoutMs?: number
) {
  return render(
    <QueryClientProvider client={queryClient}>
      <AccountUpgradeSheetProvider waitingTimeoutMs={waitingTimeoutMs}>
        {children}
      </AccountUpgradeSheetProvider>
    </QueryClientProvider>
  );
}

describe("UpgradeSheet & AccountUpgradeSheetProvider (S12 / U9)", () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("preselects Pro annual with tax line, starts checkout for selected interval, and completes via account stream", async () => {
    const queryClient = createQueryClient();
    const onUpgraded = vi.fn();
    let streamHandlers: accountApi.AccountStreamHandlers | null = null;
    const navigate = vi.fn();
    const closeTab = vi.fn();

    vi.spyOn(billingWindow, "openBillingBrowserWindow").mockReturnValue({
      navigate,
      close: closeTab,
    });
    const checkoutSpy = vi
      .spyOn(accountApi, "startAccountCheckout")
      .mockResolvedValue({ url: "https://checkout.stripe.com/c/pay/cs_test_123" });
    vi.spyOn(accountApi, "fetchAccountStatus").mockResolvedValue(makeAccountStatus());
    vi.spyOn(accountApi, "openAccountStream").mockImplementation(handlers => {
      streamHandlers = handlers;
      return () => {
        streamHandlers = null;
      };
    });

    renderSheet(queryClient, <TriggerHarness onUpgraded={onUpgraded} />);

    fireEvent.click(screen.getByRole("button", { name: "Open upgrade" }));

    const sheet = screen.getByTestId("account-upgrade-sheet");
    expect(sheet).toHaveAttribute("data-phase", "select");
    expect(screen.getByRole("radio", { name: /Pro annual/ })).toHaveAttribute(
      "aria-checked",
      "true"
    );
    expect(
      screen.getByText("Prices in USD. Plus applicable taxes, calculated at checkout.")
    ).toBeInTheDocument();
    expect(screen.getByText("Unlimited computers")).toBeInTheDocument();
    expect(
      screen.getByText("Hosted route: reach your computers from anywhere")
    ).toBeInTheDocument();
    expect(screen.getByText("Phone access")).toBeInTheDocument();
    expect(screen.getByText("Hosted webhooks")).toBeInTheDocument();

    // Switch to monthly and launch checkout.
    fireEvent.click(screen.getByRole("radio", { name: /Pro monthly/ }));
    expect(screen.getByRole("radio", { name: /Pro monthly/ })).toHaveAttribute(
      "aria-checked",
      "true"
    );

    fireEvent.click(screen.getByRole("button", { name: /Continue to checkout/i }));

    await waitFor(() => {
      expect(checkoutSpy).toHaveBeenCalledWith("month");
      expect(navigate).toHaveBeenCalledWith("https://checkout.stripe.com/c/pay/cs_test_123");
      expect(sheet).toHaveAttribute("data-phase", "waiting");
    });
    expect(screen.getByText("Waiting for checkout…")).toBeInTheDocument();
    expect(streamHandlers).not.toBeNull();

    // Account stream delivers plan: "pro" (works even with remote access off).
    act(() => {
      streamHandlers?.onAccountChanged({
        kind: "plan.changed",
        plan: "pro",
        payment_issue: false,
      });
    });

    await waitFor(() => {
      expect(sheet).toHaveAttribute("data-phase", "done");
    });
    expect(screen.getAllByText("You're on Pro").length).toBeGreaterThan(0);

    fireEvent.click(screen.getByRole("button", { name: "Done" }));
    expect(onUpgraded).toHaveBeenCalledTimes(1);
    expect(screen.queryByTestId("account-upgrade-sheet")).not.toBeInTheDocument();
  });

  it("shows Still waiting? Check again after timeout and only completes when /api/account reports pro", async () => {
    vi.useFakeTimers();
    const queryClient = createQueryClient();
    const navigate = vi.fn();

    vi.spyOn(billingWindow, "openBillingBrowserWindow").mockReturnValue({
      navigate,
      close: vi.fn(),
    });
    vi.spyOn(accountApi, "startAccountCheckout").mockResolvedValue({
      url: "https://checkout.stripe.com/c/pay/cs_test_timeout",
    });
    vi.spyOn(accountApi, "openAccountStream").mockReturnValue(() => undefined);
    const fetchStatusSpy = vi
      .spyOn(accountApi, "fetchAccountStatus")
      .mockResolvedValueOnce(makeAccountStatus({ plan: "free" }))
      .mockResolvedValueOnce(makeAccountStatus({ plan: "free" }))
      .mockResolvedValueOnce(makeAccountStatus({ plan: "pro", remote_computers_limit: null }));

    renderSheet(queryClient, <TriggerHarness />, 15 * 60 * 1000);

    fireEvent.click(screen.getByRole("button", { name: "Open upgrade" }));
    fireEvent.click(screen.getByRole("button", { name: /Continue to checkout/i }));

    await act(async () => {
      await Promise.resolve();
    });
    const sheet = screen.getByTestId("account-upgrade-sheet");
    expect(sheet).toHaveAttribute("data-phase", "waiting");

    // Advance 15 minutes to trigger waiting_timeout.
    act(() => {
      vi.advanceTimersByTime(15 * 60 * 1000);
    });
    expect(sheet).toHaveAttribute("data-phase", "waiting_timeout");
    expect(screen.getByText("Still waiting? Check again")).toBeInTheDocument();

    // First Check again still sees Free → stays waiting_timeout.
    const checkButtons = screen.getAllByRole("button", { name: /Check again/i });
    fireEvent.click(checkButtons[0]!);
    await act(async () => {
      await Promise.resolve();
    });
    expect(sheet).toHaveAttribute("data-phase", "waiting_timeout");

    // Second Check again sees Pro → completes to done.
    fireEvent.click(screen.getAllByRole("button", { name: /Check again/i })[0]!);
    await act(async () => {
      await Promise.resolve();
    });
    expect(fetchStatusSpy).toHaveBeenCalled();
    expect(sheet).toHaveAttribute("data-phase", "done");
    expect(queryClient.getQueryData<accountApi.AccountStatusPayload>(accountKeys.status())?.plan).toBe(
      "pro"
    );
  });

  it("shows cancelled message when returning from waiting and surfaces checkout API errors", async () => {
    const queryClient = createQueryClient();
    const closeTab = vi.fn();

    vi.spyOn(billingWindow, "openBillingBrowserWindow").mockReturnValue({
      navigate: vi.fn(),
      close: closeTab,
    });
    vi.spyOn(accountApi, "openAccountStream").mockReturnValue(() => undefined);
    vi.spyOn(accountApi, "fetchAccountStatus").mockResolvedValue(makeAccountStatus());
    const checkoutSpy = vi
      .spyOn(accountApi, "startAccountCheckout")
      .mockResolvedValueOnce({ url: "https://checkout.stripe.com/c/pay/cs_cancel" })
      .mockRejectedValueOnce(
        new accountApi.AccountApiError("Cloud unreachable", 503, "account_cloud_unreachable")
      );

    renderSheet(queryClient, <TriggerHarness />);

    fireEvent.click(screen.getByRole("button", { name: "Open upgrade" }));
    fireEvent.click(screen.getByRole("button", { name: /Continue to checkout/i }));

    const sheet = screen.getByTestId("account-upgrade-sheet");
    await waitFor(() => {
      expect(sheet).toHaveAttribute("data-phase", "waiting");
    });

    // Cancelling during wait returns to plan choice with cancelled notice.
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(sheet).toHaveAttribute("data-phase", "select");
    expect(screen.getByTestId("upgrade-sheet-cancelled")).toHaveTextContent(
      "Checkout was cancelled. Nothing was charged."
    );

    // Retrying checkout when cloud is unreachable closes reserved tab and shows error inline.
    fireEvent.click(screen.getByRole("button", { name: /Continue to checkout/i }));
    await waitFor(() => {
      expect(checkoutSpy).toHaveBeenCalledTimes(2);
      expect(closeTab).toHaveBeenCalledTimes(1);
      expect(screen.getByTestId("upgrade-sheet-error")).toHaveTextContent(
        "Can't reach Compozy Cloud. Local work is unaffected."
      );
    });
  });
});
