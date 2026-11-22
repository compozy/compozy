import { useState } from "react";

import { toast } from "@compozy/ui";

import {
  AccountDevicesSection,
  AccountRevokeDeviceDialog,
} from "@/systems/account/components/account-devices-section";
import { AccountPendingApprovalsSection } from "@/systems/account/components/account-pending-approvals";
import {
  UpgradeSheet,
  type UpgradeInterval,
  type UpgradeSheetPhase,
} from "@/systems/account/components/account-upgrade-sheet";
import {
  ACCOUNT_DEVICES,
  ACCOUNT_PRESETS,
  type AccountDeviceFixture,
  type AccountStateFixture,
  type PendingApprovalFixture,
} from "@/systems/computers/mocks/remote-computers-fixtures";

import { RemoteSettingsShell } from "../remote-computers-shared";
import {
  AccountBanners,
  AccountIdentitySection,
  AccountPlanSection,
  AccountSignedOutSection,
  AccountSigningInSection,
} from "./account-sections";

export interface AccountPageProps {
  account: AccountStateFixture;
  signingIn?: boolean;
  devices?: readonly AccountDeviceFixture[];
  pendingApprovals?: readonly PendingApprovalFixture[];
  downgradedNeedsProCount?: number;
  localRequireDeviceApproval?: boolean;
  initialUpgradeOpen?: boolean;
  initialUpgradePhase?: UpgradeSheetPhase;
  initialUpgradeInterval?: UpgradeInterval;
  initialRevokeDeviceId?: string | null;
}

function buildSubheadMeta(
  account: AccountStateFixture,
  signingIn: boolean,
  devices: readonly AccountDeviceFixture[],
  approvals: readonly PendingApprovalFixture[]
): Array<{ key: string; content: string }> {
  if (signingIn) {
    return [{ key: "signing-in", content: "Waiting for browser sign-in" }];
  }
  if (!account.signedIn) {
    return [{ key: "local", content: "Signed out" }];
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
  signingIn: initialSigningIn = false,
  devices: initialDevices = ACCOUNT_DEVICES,
  pendingApprovals: initialApprovals = [],
  downgradedNeedsProCount = 0,
  localRequireDeviceApproval: initialRequireApproval = true,
  initialUpgradeOpen = false,
  initialUpgradePhase = "select",
  initialUpgradeInterval = "year",
  initialRevokeDeviceId = null,
}: AccountPageProps) {
  const [account, setAccount] = useState(initialAccount);
  const [signingIn, setSigningIn] = useState(initialSigningIn);
  const [devices, setDevices] = useState<AccountDeviceFixture[]>([...initialDevices]);
  const [approvals, setApprovals] = useState<PendingApprovalFixture[]>([...initialApprovals]);
  const [requireApproval, setRequireApproval] = useState(initialRequireApproval);
  const [upgradeOpen, setUpgradeOpen] = useState(initialUpgradeOpen);
  const [upgradePhase, setUpgradePhase] = useState<UpgradeSheetPhase>(initialUpgradePhase);
  const [revokeTarget, setRevokeTarget] = useState<AccountDeviceFixture | null>(() =>
    initialRevokeDeviceId
      ? (initialDevices.find(device => device.id === initialRevokeDeviceId) ?? null)
      : null
  );

  const currentDevice = devices.find(device => device.thisDevice) ?? devices[0];
  const currentDeviceIsAdmin = Boolean(currentDevice?.admin);

  const openUpgradeSheet = (phase: UpgradeSheetPhase = "select") => {
    setUpgradePhase(phase);
    setUpgradeOpen(true);
  };

  const handleCompleteUpgrade = () => {
    setAccount({
      ...ACCOUNT_PRESETS.pro,
      remoteComputersUsed: Math.max(account.remoteComputersUsed, 1),
    });
    toast.success("You're on Compozy Pro.");
  };

  const handleToggleAdmin = (target: AccountDeviceFixture, nextAdmin: boolean) => {
    setDevices(current =>
      current.map(device => (device.id === target.id ? { ...device, admin: nextAdmin } : device))
    );
    toast.success(
      nextAdmin
        ? `${target.name} is now trusted for admin.`
        : `Removed admin trust from ${target.name}.`
    );
  };

  const handleConfirmRevoke = (target: AccountDeviceFixture, enableApproval: boolean) => {
    setDevices(current => current.filter(device => device.id !== target.id));
    if (enableApproval && !requireApproval) {
      setRequireApproval(true);
    }
    setRevokeTarget(null);
    toast.success(
      enableApproval && !requireApproval
        ? `Revoked ${target.name} and turned on approval for new devices.`
        : `Revoked ${target.name}.`
    );
  };

  const handleDecideApproval = (approvalId: string, approve: boolean) => {
    setApprovals(current =>
      current.map(item =>
        item.id === approvalId
          ? {
              ...item,
              state: approve ? "approved" : "denied",
              appliedResult: approve ? "approved" : "denied",
            }
          : item
      )
    );
    toast.success(approve ? "Device approved." : "Device denied.");
  };

  const handleSignOut = () => {
    setAccount(ACCOUNT_PRESETS.signed_out);
    setSigningIn(false);
    setApprovals([]);
    toast("Signed out. Remote connections from this computer are closed.");
  };

  const openBillingPortal = () => toast("Opens Stripe Customer Portal in your browser.");

  const overlay = (
    <>
      <UpgradeSheet
        initialInterval={initialUpgradeInterval}
        initialPhase={upgradePhase}
        key={`${upgradeOpen}-${upgradePhase}`}
        onCheckAgain={() => toast("Checked plan status with Compozy Cloud.")}
        onClose={() => setUpgradeOpen(false)}
        onCompleteUpgrade={handleCompleteUpgrade}
        onContinueToCheckout={chosen =>
          toast(`Opened Stripe Checkout (${chosen === "year" ? "Pro annual" : "Pro monthly"}).`)
        }
        onReopenCheckout={() => toast("Reopened Stripe Checkout in your browser.")}
        open={upgradeOpen}
      />
      <AccountRevokeDeviceDialog
        device={revokeTarget}
        onClose={() => setRevokeTarget(null)}
        onConfirm={handleConfirmRevoke}
        requireDeviceApproval={requireApproval}
      />
    </>
  );

  return (
    <RemoteSettingsShell
      activeSection="account"
      meta={buildSubheadMeta(account, signingIn, devices, approvals)}
      overlay={overlay}
      title="Account"
      width="form"
    >
      <AccountBanners
        account={account}
        onManageBilling={openBillingPortal}
        onRetryCloud={() => {
          setAccount(current => ({ ...current, cloudUnreachable: false }));
          toast.success("Reconnected to Compozy Cloud.");
        }}
      />

      {!account.signedIn ? (
        signingIn ? (
          <AccountSigningInSection
            onCancel={() => setSigningIn(false)}
            onOpenBrowser={() =>
              toast("Opened https://cloud.compozy.com/activate in your browser.")
            }
          />
        ) : (
          <AccountSignedOutSection
            onSignIn={() => {
              setSigningIn(true);
              toast("Opened Compozy sign-in in your browser.");
            }}
            onSignInWithCode={() => setSigningIn(true)}
          />
        )
      ) : (
        <>
          <AccountIdentitySection
            account={account}
            currentDevice={currentDevice}
            onSignOut={handleSignOut}
          />

          <AccountPlanSection
            account={account}
            downgradedNeedsProCount={downgradedNeedsProCount}
            onContactSupport={() => toast("Opens support to raise the hosted-route cap.")}
            onManageBilling={openBillingPortal}
            onUpgrade={() => openUpgradeSheet("select")}
          />

          <AccountPendingApprovalsSection
            approvals={approvals}
            onApprove={id => handleDecideApproval(id, true)}
            onDeny={id => handleDecideApproval(id, false)}
          />

          <AccountDevicesSection
            currentDeviceIsAdmin={currentDeviceIsAdmin}
            devices={devices}
            onRequestRevoke={setRevokeTarget}
            onToggleAdmin={handleToggleAdmin}
          />
        </>
      )}
    </RemoteSettingsShell>
  );
}
