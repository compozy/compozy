// Suite: Account deletion Pro cancellation & 14-day refund block (S12 / U3, US-028 AC-4/EC-1)
// Invariant: Active Pro never blocks account deletion; AccountDeleteProBlock and
//            AccountDeleteDialog render the remaining period and refund line derived from
//            refund_eligible_until (or complimentary Pro copy) and label confirm "Delete account
//            and cancel Pro".
// Boundary IN: AccountDeleteProBlock, AccountDeleteDialog, AccountDeleteSection,
//              describeDeleteAccountProImpact.
// Boundary OUT: Cloud deletion job and Stripe cancellation (covered by Go UT-121 / UT-136).

import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ACCOUNT_PRESETS } from "@/systems/computers/mocks/remote-computers-fixtures";

import { COMPOZY_REFUNDS_URL } from "../../lib/account-view";
import {
  AccountDeleteDialog,
  AccountDeleteProBlock,
  AccountDeleteSection,
} from "../account-danger-section";

describe("AccountDeleteProBlock & AccountDeleteDialog (S12 / US-028 AC-4)", () => {
  it("shows full-refund line and Refund policy link inside the 14-day window", () => {
    render(
      <AccountDeleteDialog
        email="pedro@example.com"
        linkedComputersCount={2}
        now={new Date("2026-11-25T12:00:00Z")}
        onClose={vi.fn()}
        onConfirm={vi.fn()}
        open
        subscription={{
          source: "stripe",
          interval: "year",
          currentPeriodEnd: "2027-03-01T00:00:00Z",
          // 14 days after Nov 20, 2026
          refundEligibleUntil: "2026-12-04T00:00:00Z",
        }}
      />
    );

    const proBlock = screen.getByTestId("account-delete-pro-block");
    expect(proBlock).toHaveTextContent("Deleting your account also cancels Pro.");
    expect(proBlock).toHaveTextContent("Pro (annual) is paid until Mar 1, 2027 and ends now.");
    expect(proBlock).toHaveTextContent("Your last payment on Nov 20 is refunded in full.");
    expect(screen.getByRole("link", { name: /Refund policy/i })).toHaveAttribute(
      "href",
      COMPOZY_REFUNDS_URL
    );
    expect(
      screen.getByRole("button", { name: "Delete account and cancel Pro" })
    ).toBeInTheDocument();
  });

  it("shows no-refund line when refund_eligible_until is null or elapsed", () => {
    const { rerender } = render(
      <AccountDeleteProBlock
        now={new Date("2026-12-10T00:00:00Z")}
        subscription={{
          source: "stripe",
          interval: "month",
          currentPeriodEnd: "2027-01-01T00:00:00Z",
          refundEligibleUntil: null,
        }}
      />
    );

    expect(screen.getByTestId("account-delete-pro-block")).toHaveTextContent(
      "Pro (monthly) is paid until Jan 1, 2027 and ends now."
    );
    expect(screen.getByTestId("account-delete-pro-block")).toHaveTextContent(
      "Refunds cover 14 days after a payment, so no refund applies."
    );

    rerender(
      <AccountDeleteProBlock
        now={new Date("2026-12-10T00:00:00Z")}
        subscription={{
          source: "stripe",
          interval: "year",
          currentPeriodEnd: "2027-03-01T00:00:00Z",
          refundEligibleUntil: "2026-12-04T00:00:00Z",
        }}
      />
    );
    expect(screen.getByTestId("account-delete-pro-block")).toHaveTextContent(
      "Refunds cover 14 days after a payment, so no refund applies."
    );
  });

  it("renders complimentary Pro copy without refund link", () => {
    render(
      <AccountDeleteProBlock
        subscription={{
          source: "complimentary",
          interval: null,
          currentPeriodEnd: "2027-01-01T00:00:00Z",
          refundEligibleUntil: null,
        }}
      />
    );

    const proBlock = screen.getByTestId("account-delete-pro-block");
    expect(proBlock).toHaveTextContent("Deleting your account also cancels Pro.");
    expect(proBlock).toHaveTextContent("Your complimentary Pro ends now.");
    expect(screen.queryByRole("link", { name: /Refund policy/i })).not.toBeInTheDocument();
  });

  it("keeps Delete account available on Pro for admin devices and hides it on non-admin devices", () => {
    const onDelete = vi.fn();
    const { rerender } = render(
      <AccountDeleteSection
        account={ACCOUNT_PRESETS.pro}
        currentDeviceIsAdmin
        onRequestDelete={onDelete}
      />
    );

    expect(screen.getByRole("button", { name: "Delete account" })).toBeInTheDocument();
    expect(screen.queryByText(/Cancel Pro before deleting/i)).not.toBeInTheDocument();

    rerender(
      <AccountDeleteSection
        account={ACCOUNT_PRESETS.pro}
        currentDeviceIsAdmin={false}
        onRequestDelete={onDelete}
      />
    );
    expect(screen.queryByRole("button", { name: "Delete account" })).not.toBeInTheDocument();
    expect(
      screen.getByText("Delete your account from a device trusted for admin.")
    ).toBeInTheDocument();
  });
});
