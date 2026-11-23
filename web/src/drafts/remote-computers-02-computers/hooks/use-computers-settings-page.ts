import { useState } from "react";

import { toast } from "@compozy/ui";

import {
  ALL_REMOTE_SCOPES,
  type AccountDeviceFixture,
  type AccountStateFixture,
  type AccountUnaddedComputerFixture,
  type ComputerFixture,
  type LoadBalancingWeight,
  type LocalComputerFixture,
  type PendingApprovalFixture,
  type RemoteScope,
  type UpgradeMigrationNoticeFixture,
} from "@/systems/computers/mocks/remote-computers-fixtures";

interface UseComputersSettingsPageInput {
  initialAccount: AccountStateFixture;
  initialLocal: LocalComputerFixture;
  initialComputers: readonly ComputerFixture[];
  initialUnadded: readonly AccountUnaddedComputerFixture[];
  initialDevices: readonly AccountDeviceFixture[];
  initialApprovals: readonly PendingApprovalFixture[];
  initialUpgradeNotice: UpgradeMigrationNoticeFixture | null;
  initialLoadBalancingEnabled: boolean;
  initialDisableLocalOpen: boolean;
  initialRemoveTargetId: string | null;
  initialRevokeDeviceId: string | null;
  initialUninstallWarning: string | null;
}

export function useComputersSettingsPage({
  initialAccount,
  initialLocal,
  initialComputers,
  initialUnadded,
  initialDevices,
  initialApprovals,
  initialUpgradeNotice,
  initialLoadBalancingEnabled,
  initialDisableLocalOpen,
  initialRemoveTargetId,
  initialRevokeDeviceId,
  initialUninstallWarning,
}: UseComputersSettingsPageInput) {
  const [account, setAccount] = useState(initialAccount);
  const [local, setLocal] = useState(initialLocal);
  const [computers, setComputers] = useState<ComputerFixture[]>([...initialComputers]);
  const [unadded, setUnadded] = useState<AccountUnaddedComputerFixture[]>([...initialUnadded]);
  const [devices, setDevices] = useState<AccountDeviceFixture[]>([...initialDevices]);
  const [approvals, setApprovals] = useState<PendingApprovalFixture[]>([...initialApprovals]);
  const [upgradeNotice, setUpgradeNotice] = useState(initialUpgradeNotice);
  const [uninstallWarning, setUninstallWarning] = useState(initialUninstallWarning);
  const [loadBalancingEnabled, setLoadBalancingEnabled] = useState(initialLoadBalancingEnabled);
  const [confirmDisableLocal, setConfirmDisableLocal] = useState(initialDisableLocalOpen);

  const [removeTarget, setRemoveTarget] = useState<ComputerFixture | null>(() =>
    initialRemoveTargetId
      ? (initialComputers.find(item => item.id === initialRemoveTargetId) ?? null)
      : null
  );
  const [revokeTarget, setRevokeTarget] = useState<AccountDeviceFixture | null>(() =>
    initialRevokeDeviceId
      ? (initialDevices.find(item => item.id === initialRevokeDeviceId) ?? null)
      : null
  );

  const handleToggleComputer = (target: ComputerFixture, nextEnabled: boolean) => {
    setComputers(current =>
      current.map(item =>
        item.id === target.id
          ? { ...item, enabled: nextEnabled, status: nextEnabled ? "connected" : "disabled" }
          : item
      )
    );
    toast.message(nextEnabled ? `Enabled ${target.name}` : `Turned off ${target.name}`);
  };

  const handleRemoveRoute = (target: ComputerFixture, routeId: string) => {
    setComputers(current =>
      current.map(item =>
        item.id === target.id
          ? { ...item, routes: item.routes.filter(route => route.id !== routeId) }
          : item
      )
    );
    toast.message(`Removed route from ${target.name}`);
  };

  const handleConfirmRemove = (
    target: ComputerFixture,
    options: { unlink: boolean; uninstall: boolean }
  ) => {
    setComputers(current => current.filter(item => item.id !== target.id));
    setRemoveTarget(null);
    if (options.unlink && account.plan === "free") {
      setAccount(current => ({
        ...current,
        remoteComputersUsed: Math.max(0, current.remoteComputersUsed - 1),
      }));
    }
    if (options.uninstall && target.status === "offline" && target.sshTarget) {
      setUninstallWarning(
        `Couldn't uninstall from ${target.sshTarget}; run compozy service uninstall there.`
      );
    } else {
      toast.success(`Removed ${target.name}. Agents already running there keep running.`);
    }
  };

  const handleAddAccountComputer = (item: AccountUnaddedComputerFixture) => {
    setUnadded(current => current.filter(entry => entry.id !== item.id));
    const addedRoute = {
      id: `rt_${item.id}`,
      kind: "direct" as const,
      address: `${item.name}.local:47823`,
      label: "via LAN",
      inUse: item.online,
      learned: true,
      lastError: null,
    };
    setComputers(current => [
      {
        id: item.id,
        name: item.name,
        platform: item.platform,
        iconKind: item.iconKind,
        version: "0.8.0",
        protocol: { min: 1, max: 1 },
        status: item.online ? "connected" : "offline",
        enabled: true,
        planState: "ok",
        originKind: "account",
        routeInUse: item.online ? addedRoute : null,
        routes: [addedRoute],
        scopes: ALL_REMOTE_SCOPES,
        load: null,
        loadBalancingWeight: 50,
        diagnosis: null,
        providers: [],
        projects: [],
        addedAt: "2026-11-22T20:44:00Z",
        lastConnectedAt: "2026-11-22T20:44:00Z",
        lastConnectedLabel: item.online ? "just now" : "never",
      },
      ...current,
    ]);
    toast.success(`Added ${item.name}`);
  };

  const handleConfirmRevoke = (device: AccountDeviceFixture, enableApproval: boolean) => {
    setDevices(current => current.filter(item => item.id !== device.id));
    if (enableApproval && !local.requireDeviceApproval) {
      setLocal(current => ({ ...current, requireDeviceApproval: true }));
    }
    setRevokeTarget(null);
    toast.success(
      enableApproval && !local.requireDeviceApproval
        ? `Revoked ${device.name} and enabled approval for new devices.`
        : `Revoked ${device.name}.`
    );
  };

  const handleToggleDeviceScope = (deviceId: string, scope: RemoteScope, next: boolean) => {
    setDevices(current =>
      current.map(device => {
        if (device.id !== deviceId) return device;
        const set = new Set(device.scopesOnLocal);
        if (next) set.add(scope);
        else set.delete(scope);
        return { ...device, scopesOnLocal: [...set] };
      })
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

  const handleRequestToggleRunAgents = (next: boolean) => {
    if (!next && local.activeLocalSessions > 0) {
      setConfirmDisableLocal(true);
      return;
    }
    setLocal(current => ({ ...current, localRunAgents: next }));
  };

  const handleRetryCloud = () => {
    setAccount(current => ({ ...current, cloudUnreachable: false }));
    toast.success("Reconnected to Compozy Cloud.");
  };

  const handleConfirmDisableLocal = () => {
    setLocal(current => ({ ...current, localRunAgents: false }));
    setConfirmDisableLocal(false);
  };

  const handleToggleHostedRoute = (next: boolean) => {
    setLocal(current => ({
      ...current,
      hostedRouteEnabled: next,
      hostedRouteState: next ? "starting" : "off",
    }));
  };

  const handleToggleRemoteEnabled = (next: boolean) => {
    setLocal(current => ({ ...current, remoteEnabled: next }));
  };

  const handleToggleRequireApproval = (next: boolean) => {
    setLocal(current => ({ ...current, requireDeviceApproval: next }));
  };

  const handleChangeComputerWeight = (id: string, weight: LoadBalancingWeight) => {
    setComputers(current =>
      current.map(item => (item.id === id ? { ...item, loadBalancingWeight: weight } : item))
    );
  };

  const handleChangeLocalWeight = (weight: LoadBalancingWeight) => {
    setLocal(current => ({ ...current, loadBalancingWeight: weight }));
  };

  return {
    account,
    local,
    computers,
    unadded,
    devices,
    approvals,
    upgradeNotice,
    setUpgradeNotice,
    uninstallWarning,
    setUninstallWarning,
    loadBalancingEnabled,
    setLoadBalancingEnabled,
    confirmDisableLocal,
    setConfirmDisableLocal,
    removeTarget,
    setRemoveTarget,
    revokeTarget,
    setRevokeTarget,
    handleToggleComputer,
    handleRemoveRoute,
    handleConfirmRemove,
    handleAddAccountComputer,
    handleConfirmRevoke,
    handleToggleDeviceScope,
    handleDecideApproval,
    handleRequestToggleRunAgents,
    handleRetryCloud,
    handleConfirmDisableLocal,
    handleToggleHostedRoute,
    handleToggleRemoteEnabled,
    handleToggleRequireApproval,
    handleChangeComputerWeight,
    handleChangeLocalWeight,
  };
}
