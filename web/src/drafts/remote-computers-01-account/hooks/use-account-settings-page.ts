import { useEffect, useState, type RefObject } from "react";

import { toast } from "@compozy/ui";

import type { UpgradeSheetPhase } from "@/systems/account/components/account-upgrade-sheet";
import {
  ACCOUNT_PRESETS,
  type AccountDeviceFixture,
  type AccountStateFixture,
  type AccountTokenFixture,
  type PendingApprovalFixture,
} from "@/systems/computers/mocks/remote-computers-fixtures";
import type { CreatedAccountTokenFixture } from "@/systems/computers/mocks/remote-computers-fixtures-lane-a";

interface UseAccountSettingsPageInput {
  initialAccount: AccountStateFixture;
  initialDevices: readonly AccountDeviceFixture[];
  initialApprovals: readonly PendingApprovalFixture[];
  initialTokens: readonly AccountTokenFixture[];
  localRemoteEnabled: boolean;
  initialRequireApproval: boolean;
  scrollToLower: boolean;
  initialUpgradeOpen: boolean;
  initialUpgradePhase: UpgradeSheetPhase;
  initialRevokeDeviceId: string | null;
  initialCreatedToken: CreatedAccountTokenFixture | null;
  initialSignOutOpen: boolean;
  initialDeleteOpen: boolean;
  initialLinkMismatch: boolean;
  lowerSectionsRef: RefObject<HTMLDivElement | null>;
}

export function useAccountSettingsPage({
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
  lowerSectionsRef,
}: UseAccountSettingsPageInput) {
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
  }, [lowerSectionsRef, scrollToLower]);

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
    toast.message(
      account.installationLinked
        ? "Signed out and unlinked this computer. Returning to Sign in."
        : "Signed out. Returning to Sign in."
    );
  };

  const handleConfirmDeleteAccount = () => {
    setDeleteOpen(false);
    toast.message("Account deletion started. Returning to Sign in.");
  };

  const handleRetryCloud = () => {
    setAccount(current => ({ ...current, cloudUnreachable: false }));
    toast.success("Reconnected to Compozy Cloud.");
  };

  const handleUnlink = () => {
    setLinkMismatch(false);
    toast.message("Unlinked this computer from remote access. Returning to Sign in.");
  };

  const handleToggleMarketingOptIn = (next: boolean) => {
    setAccount(current => ({ ...current, marketingOptIn: next }));
    toast.success(
      next ? "Subscribed to product news and offers." : "Unsubscribed from product news and offers."
    );
  };

  return {
    account,
    devices,
    approvals,
    tokens,
    requireApproval,
    upgradeOpen,
    setUpgradeOpen,
    upgradePhase,
    createTokenOpen,
    setCreateTokenOpen,
    createdToken,
    setCreatedToken,
    signOutOpen,
    setSignOutOpen,
    deleteOpen,
    setDeleteOpen,
    linkMismatch,
    revokeTarget,
    setRevokeTarget,
    isSessionAccount,
    currentDevice,
    currentDeviceIsAdmin,
    openUpgradeSheet,
    handleCompleteUpgrade,
    handleToggleAdmin,
    handleConfirmRevoke,
    handleDecideApproval,
    handleCreateToken,
    handleRevokeToken,
    handleConfirmSignOut,
    handleConfirmDeleteAccount,
    handleRetryCloud,
    handleUnlink,
    handleToggleMarketingOptIn,
  };
}
