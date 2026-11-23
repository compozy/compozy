import { toast } from "@compozy/ui";

import {
  AccountDeleteDialog,
  AccountDeleteSection,
  AccountSignOutDialog,
} from "@/systems/account/components/account-danger-section";
import {
  AccountDevicesSection,
  AccountRevokeDeviceDialog,
} from "@/systems/account/components/account-devices-section";
import {
  AccountCreateTokenDialog,
  AccountEmailSection,
  AccountTokenCreatedDialog,
  AccountTokensSection,
} from "@/systems/account/components/account-email-tokens-section";
import { AccountPendingApprovalsSection } from "@/systems/account/components/account-pending-approvals";
import {
  UpgradeSheet,
  type UpgradeInterval,
  type UpgradeSheetPhase,
} from "@/systems/account/components/account-upgrade-sheet";
import {
  ACCOUNT_DEVICES,
  type AccountDeviceFixture,
  type AccountStateFixture,
  type AccountTokenFixture,
  type PendingApprovalFixture,
} from "@/systems/computers/mocks/remote-computers-fixtures";
import {
  ACCOUNT_TOKENS,
  type CreatedAccountTokenFixture,
} from "@/systems/computers/mocks/remote-computers-fixtures-lane-a";

import { RemoteSettingsShell } from "../remote-computers-shared/remote-settings-shell";
import { AccountBanners, AccountIdentitySection, AccountPlanSection } from "./account-sections";
import { useAccountSettingsPage } from "./hooks/use-account-settings-page";

export interface AccountPageProps {
  account: AccountStateFixture;
  devices?: readonly AccountDeviceFixture[];
  pendingApprovals?: readonly PendingApprovalFixture[];
  tokens?: readonly AccountTokenFixture[];
  downgradedNeedsProCount?: number;
  localRemoteEnabled?: boolean;
  localRequireDeviceApproval?: boolean;
  scrollToLower?: boolean;
  initialUpgradeOpen?: boolean;
  initialUpgradePhase?: UpgradeSheetPhase;
  initialUpgradeInterval?: UpgradeInterval;
  initialRevokeDeviceId?: string | null;
  initialCreatedToken?: CreatedAccountTokenFixture | null;
  initialSignOutOpen?: boolean;
  initialDeleteOpen?: boolean;
  initialLinkMismatch?: boolean;
}

function openBillingPortal() {
  toast.message("Opens Stripe Customer Portal in your browser.");
}

function buildSubheadMeta(
  account: AccountStateFixture,
  devices: readonly AccountDeviceFixture[],
  approvals: readonly PendingApprovalFixture[]
): Array<{ key: string; content: string }> {
  if (account.source === "host_link") {
    return [{ key: "host-link", content: "Linked for remote access" }];
  }
  if (account.source === "account_token") {
    return [
      { key: "token", content: `Account token "${account.tokenName ?? "ci"}"` },
      { key: "scope", content: "Local use only" },
    ];
  }

  const entries: Array<{ key: string; content: string }> = [
    {
      key: "plan",
      content: account.plan === "pro" ? "Compozy Pro" : "Free plan",
    },
    {
      key: "devices",
      content: `${devices.length} ${devices.length === 1 ? "device" : "devices"}`,
    },
  ];

  const waiting = approvals.filter(item => item.state === "pending").length;
  if (waiting > 0) {
    entries.push({
      key: "approvals",
      content: `${waiting} pending ${waiting === 1 ? "approval" : "approvals"}`,
    });
  }
  return entries;
}

export function AccountSettingsPage({
  account: initialAccount,
  devices: initialDevices = ACCOUNT_DEVICES,
  pendingApprovals: initialApprovals = [],
  tokens: initialTokens = ACCOUNT_TOKENS,
  downgradedNeedsProCount = 0,
  localRemoteEnabled = true,
  localRequireDeviceApproval: initialRequireApproval = true,
  scrollToLower = false,
  initialUpgradeOpen = false,
  initialUpgradePhase = "select",
  initialUpgradeInterval = "year",
  initialRevokeDeviceId = null,
  initialCreatedToken = null,
  initialSignOutOpen = false,
  initialDeleteOpen = false,
  initialLinkMismatch = false,
}: AccountPageProps) {
  const page = useAccountSettingsPage({
    initialAccount,
    initialDevices,
    initialApprovals,
    initialTokens,
    localRemoteEnabled,
    initialRequireApproval,
    scrollToLower,
    initialUpgradeOpen,
    initialUpgradePhase,
    initialRevokeDeviceId,
    initialCreatedToken,
    initialSignOutOpen,
    initialDeleteOpen,
    initialLinkMismatch,
  });

  const overlay = (
    <>
      <UpgradeSheet
        initialInterval={initialUpgradeInterval}
        initialPhase={page.upgradePhase}
        key={`${page.upgradeOpen}-${page.upgradePhase}`}
        onCheckAgain={() => toast.message("Checked plan status with Compozy Cloud.")}
        onClose={() => page.setUpgradeOpen(false)}
        onCompleteUpgrade={page.handleCompleteUpgrade}
        onContinueToCheckout={chosen =>
          toast.message(
            `Opened Stripe Checkout (${chosen === "year" ? "Pro annual" : "Pro monthly"}).`
          )
        }
        onReopenCheckout={() => toast.message("Reopened Stripe Checkout in your browser.")}
        open={page.upgradeOpen}
      />
      <AccountRevokeDeviceDialog
        device={page.revokeTarget}
        localRemoteEnabled={localRemoteEnabled}
        onClose={() => page.setRevokeTarget(null)}
        onConfirm={page.handleConfirmRevoke}
        requireDeviceApproval={page.requireApproval}
      />
      <AccountCreateTokenDialog
        onClose={() => page.setCreateTokenOpen(false)}
        onCreate={page.handleCreateToken}
        open={page.createTokenOpen}
      />
      <AccountTokenCreatedDialog
        createdToken={page.createdToken}
        onClose={() => page.setCreatedToken(null)}
      />
      <AccountSignOutDialog
        installationLinked={page.account.installationLinked}
        onClose={() => page.setSignOutOpen(false)}
        onConfirm={page.handleConfirmSignOut}
        open={page.signOutOpen}
      />
      <AccountDeleteDialog
        email={page.account.email}
        linkedComputersCount={Math.max(1, page.account.remoteComputersUsed)}
        onClose={() => page.setDeleteOpen(false)}
        onConfirm={page.handleConfirmDeleteAccount}
        open={page.deleteOpen}
      />
    </>
  );

  return (
    <RemoteSettingsShell
      activeSection="account"
      meta={buildSubheadMeta(page.account, page.devices, page.approvals)}
      overlay={overlay}
      title="Account"
      width="form"
    >
      {/* Account stacks several dense sections, so it spaces them wider than the scaffold's 24 px. */}
      <div className="flex flex-col gap-10" data-slot="account-sections">
        <AccountBanners
          account={page.account}
          onManageBilling={openBillingPortal}
          onRetryCloud={page.handleRetryCloud}
        />

        <AccountIdentitySection
          account={page.account}
          currentDevice={page.currentDevice}
          linkMismatchNotice={page.linkMismatch}
          onRequestSignIn={() => toast.message("Opening CompozyOS sign-in in your browser…")}
          onRequestSignOut={() => page.setSignOutOpen(true)}
          onRequestUnlink={page.handleUnlink}
        />

        {page.isSessionAccount ? (
          <>
            <AccountPlanSection
              account={page.account}
              downgradedNeedsProCount={downgradedNeedsProCount}
              onContactSupport={() => toast.message("Opens support to raise the hosted-route cap.")}
              onManageBilling={openBillingPortal}
              onUpgrade={() => page.openUpgradeSheet("select")}
            />

            <AccountPendingApprovalsSection
              approvals={page.approvals}
              onApprove={id => page.handleDecideApproval(id, true)}
              onDeny={id => page.handleDecideApproval(id, false)}
            />

            <AccountDevicesSection
              currentDeviceIsAdmin={page.currentDeviceIsAdmin}
              devices={page.devices}
              onRequestRevoke={page.setRevokeTarget}
              onToggleAdmin={page.handleToggleAdmin}
            />

            <div className="flex flex-col gap-10" ref={page.lowerSectionsRef}>
              <AccountEmailSection
                marketingOptIn={page.account.marketingOptIn}
                onToggleMarketingOptIn={page.handleToggleMarketingOptIn}
              />

              <AccountTokensSection
                onCreateToken={() => page.setCreateTokenOpen(true)}
                onRevokeToken={page.handleRevokeToken}
                tokens={page.tokens}
              />

              <AccountDeleteSection
                account={page.account}
                currentDeviceIsAdmin={page.currentDeviceIsAdmin}
                onManageBilling={openBillingPortal}
                onRequestDelete={() => page.setDeleteOpen(true)}
              />
            </div>
          </>
        ) : null}
      </div>
    </RemoteSettingsShell>
  );
}
