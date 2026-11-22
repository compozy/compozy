import { useEffect, useRef, useState } from "react";

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
  ACCOUNT_PRESETS,
  type AccountDeviceFixture,
  type AccountStateFixture,
  type AccountTokenFixture,
  type PendingApprovalFixture,
} from "@/systems/computers/mocks/remote-computers-fixtures";
import {
  ACCOUNT_TOKENS,
  type CreatedAccountTokenFixture,
} from "@/systems/computers/mocks/remote-computers-fixtures-lane-a";

import { RemoteSettingsShell } from "../remote-computers-shared";
import { AccountBanners, AccountIdentitySection, AccountPlanSection } from "./account-sections";

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
  const lowerSectionsRef = useRef<HTMLDivElement | null>(null);
  const [account, setAccount] = useState(initialAccount);
  const [devices, setDevices] = useState<AccountDeviceFixture[]>([...initialDevices]);
  const [approvals, setApprovals] = useState<PendingApprovalFixture[]>([...initialApprovals]);
  const [tokens, setTokens] = useState<AccountTokenFixture[]>([...initialTokens]);
  const [requireApproval, setRequireApproval] = useState(initialRequireApproval);
  const [upgradeOpen, setUpgradeOpen] = useState(initialUpgradeOpen);
  const [upgradePhase, setUpgradePhase] = useState<UpgradeSheetPhase>(initialUpgradePhase);
  const [createTokenOpen, setCreateTokenOpen] = useState(false);
  const [createdToken, setCreatedToken] = useState<CreatedAccountTokenFixture | null>(
    initialCreatedToken
  );
  const [signOutOpen, setSignOutOpen] = useState(initialSignOutOpen);
  const [deleteOpen, setDeleteOpen] = useState(initialDeleteOpen);
  const [linkMismatch, setLinkMismatch] = useState(initialLinkMismatch);
  const [revokeTarget, setRevokeTarget] = useState<AccountDeviceFixture | null>(() =>
    initialRevokeDeviceId
      ? (initialDevices.find(device => device.id === initialRevokeDeviceId) ?? null)
      : null
  );

  useEffect(() => {
    if (!scrollToLower) return;
    lowerSectionsRef.current?.scrollIntoView({ block: "end", behavior: "instant" });
  }, [scrollToLower]);

  const isSessionAccount = account.source === "session";
  const currentDevice = devices.find(device => device.thisDevice) ?? devices[0];
  const currentDeviceIsAdmin = Boolean(currentDevice?.admin);

  const openUpgradeSheet = (phase: UpgradeSheetPhase = "select") => {
    setUpgradePhase(phase);
    setUpgradeOpen(true);
  };

  const handleCompleteUpgrade = () => {
    setAccount({
      ...ACCOUNT_PRESETS.pro,
      marketingOptIn: account.marketingOptIn,
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
    if (enableApproval && localRemoteEnabled && !requireApproval) {
      setRequireApproval(true);
    }
    setRevokeTarget(null);
    toast.success(
      enableApproval && localRemoteEnabled && !requireApproval
        ? `Revoked ${target.name} and turned on approval for new devices on this computer.`
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

  const handleCreateToken = (name: string) => {
    const id = `atk_${Math.random().toString(36).slice(2, 8)}`;
    const nextRow: AccountTokenFixture = {
      id,
      name,
      createdLabel: "Just now",
      lastUsedLabel: null,
    };
    setTokens(current => [nextRow, ...current]);
    setCreateTokenOpen(false);
    setCreatedToken({
      id,
      name,
      token: `cpzat_2Qe8kL4mN9pR7xT1vB6cD3fG5hJ0sA8wY2uXv9`,
    });
  };

  const handleRevokeToken = (target: AccountTokenFixture) => {
    setTokens(current => current.filter(item => item.id !== target.id));
    toast.success(`Revoked account token "${target.name}".`);
  };

  const handleConfirmSignOut = () => {
    setSignOutOpen(false);
    toast(
      account.installationLinked
        ? "Signed out and unlinked this computer. Returning to Sign in."
        : "Signed out. Returning to Sign in."
    );
  };

  const handleConfirmDeleteAccount = () => {
    setDeleteOpen(false);
    toast("Account deletion started. Returning to Sign in.");
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
        localRemoteEnabled={localRemoteEnabled}
        onClose={() => setRevokeTarget(null)}
        onConfirm={handleConfirmRevoke}
        requireDeviceApproval={requireApproval}
      />
      <AccountCreateTokenDialog
        onClose={() => setCreateTokenOpen(false)}
        onCreate={handleCreateToken}
        open={createTokenOpen}
      />
      <AccountTokenCreatedDialog
        createdToken={createdToken}
        onClose={() => setCreatedToken(null)}
      />
      <AccountSignOutDialog
        installationLinked={account.installationLinked}
        onClose={() => setSignOutOpen(false)}
        onConfirm={handleConfirmSignOut}
        open={signOutOpen}
      />
      <AccountDeleteDialog
        email={account.email}
        linkedComputersCount={Math.max(1, account.remoteComputersUsed)}
        onClose={() => setDeleteOpen(false)}
        onConfirm={handleConfirmDeleteAccount}
        open={deleteOpen}
      />
    </>
  );

  return (
    <RemoteSettingsShell
      activeSection="account"
      meta={buildSubheadMeta(account, devices, approvals)}
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

      <AccountIdentitySection
        account={account}
        currentDevice={currentDevice}
        linkMismatchNotice={linkMismatch}
        onRequestSignIn={() => toast("Opening Compozy sign-in in your browser…")}
        onRequestSignOut={() => setSignOutOpen(true)}
        onRequestUnlink={() => {
          setLinkMismatch(false);
          toast("Unlinked this computer from remote access. Returning to Sign in.");
        }}
      />

      {isSessionAccount ? (
        <>
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

          <div className="flex flex-col gap-6" ref={lowerSectionsRef}>
            <AccountEmailSection
              marketingOptIn={account.marketingOptIn}
              onToggleMarketingOptIn={next => {
                setAccount(current => ({ ...current, marketingOptIn: next }));
                toast.success(
                  next
                    ? "Subscribed to product news and offers."
                    : "Unsubscribed from product news and offers."
                );
              }}
            />

            <AccountTokensSection
              onCreateToken={() => setCreateTokenOpen(true)}
              onRevokeToken={handleRevokeToken}
              tokens={tokens}
            />

            <AccountDeleteSection
              account={account}
              currentDeviceIsAdmin={currentDeviceIsAdmin}
              onManageBilling={openBillingPortal}
              onRequestDelete={() => setDeleteOpen(true)}
            />
          </div>
        </>
      ) : null}
    </RemoteSettingsShell>
  );
}
