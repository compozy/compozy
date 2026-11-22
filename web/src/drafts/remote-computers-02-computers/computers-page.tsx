import { useState } from "react";
import { Plus, UserCheck } from "@compozy/ui/icons";

import { Button, Empty, toast } from "@compozy/ui";

import {
  ACCOUNT_DEVICES,
  ALL_REMOTE_SCOPES,
  PENDING_APPROVALS,
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
import { SettingsGroup } from "@/systems/settings/components/settings-group";

import { RemoteSettingsShell } from "../remote-computers-shared";
import { ComputersBanners, ComputersListSection } from "./computers-list-section";
import {
  AccountComputersSection,
  DisableLocalAgentsDialog,
  LoadBalancingSection,
  RemoveComputerDialog,
  RevokeDeviceDialog,
} from "./computers-sections";
import { ThisComputerSection } from "./this-computer-section";

export interface ComputersPageProps {
  account: AccountStateFixture;
  local: LocalComputerFixture;
  computers: readonly ComputerFixture[];
  unadded: readonly AccountUnaddedComputerFixture[];
  devices?: readonly AccountDeviceFixture[];
  pendingApprovals?: readonly PendingApprovalFixture[];
  upgradeNotice?: UpgradeMigrationNoticeFixture | null;
  loadBalancingEnabled?: boolean;
  loadBalancingOpen?: boolean;
  devicesOpen?: boolean;
  initialRemoveTargetId?: string | null;
  initialRevokeDeviceId?: string | null;
  selectedComputerId?: string | null;
  uninstallWarningMessage?: string | null;
}

function buildSubheadMeta(
  account: AccountStateFixture,
  computers: readonly ComputerFixture[]
): Array<{ key: string; content: string }> {
  if (!account.signedIn) {
    return [{ key: "local", content: "Local computer only" }];
  }
  const connected = computers.filter(
    item => item.status === "connected" || item.status === "needs_attention"
  ).length;
  const offline = computers.filter(item => item.status === "offline").length;
  const needsPro = computers.filter(item => item.status === "needs_pro").length;
  const entries: Array<{ key: string; content: string }> = [];

  if (computers.length > 0) {
    entries.push({ key: "connected", content: `${connected} connected` });
  }
  if (offline > 0) {
    entries.push({ key: "offline", content: `${offline} offline` });
  }
  if (needsPro > 0) {
    entries.push({ key: "needs-pro", content: `${needsPro} need Pro` });
  }
  if (account.plan === "free" && account.remoteComputersLimit !== null) {
    entries.push({
      key: "slot",
      content: `${account.remoteComputersUsed} of ${account.remoteComputersLimit} remote computers used`,
    });
  } else if (account.plan === "pro") {
    entries.push({
      key: "hosted",
      content: `${account.cloudRoutesUsed} of ${account.cloudRoutesLimit} hosted routes`,
    });
  }
  return entries;
}

export function ComputersSettingsPage({
  account: initialAccount,
  local: initialLocal,
  computers: initialComputers,
  unadded: initialUnadded,
  devices: initialDevices = ACCOUNT_DEVICES,
  pendingApprovals: initialApprovals = PENDING_APPROVALS,
  upgradeNotice: initialUpgradeNotice = null,
  loadBalancingEnabled: initialLoadBalancingEnabled = true,
  loadBalancingOpen = false,
  devicesOpen = false,
  initialRemoveTargetId = null,
  initialRevokeDeviceId = null,
  selectedComputerId = null,
  uninstallWarningMessage: initialUninstallWarning = null,
}: ComputersPageProps) {
  const [account, setAccount] = useState(initialAccount);
  const [local, setLocal] = useState(initialLocal);
  const [computers, setComputers] = useState<ComputerFixture[]>([...initialComputers]);
  const [unadded, setUnadded] = useState<AccountUnaddedComputerFixture[]>([...initialUnadded]);
  const [devices, setDevices] = useState<AccountDeviceFixture[]>([...initialDevices]);
  const [approvals, setApprovals] = useState<PendingApprovalFixture[]>([...initialApprovals]);
  const [upgradeNotice, setUpgradeNotice] = useState(initialUpgradeNotice);
  const [uninstallWarning, setUninstallWarning] = useState(initialUninstallWarning);
  const [loadBalancingEnabled, setLoadBalancingEnabled] = useState(initialLoadBalancingEnabled);
  const [confirmDisableLocal, setConfirmDisableLocal] = useState(false);

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
    toast(nextEnabled ? `Enabled ${target.name}` : `Turned off ${target.name}`);
  };

  const handleRemoveRoute = (target: ComputerFixture, routeId: string) => {
    setComputers(current =>
      current.map(item =>
        item.id === target.id
          ? { ...item, routes: item.routes.filter(route => route.id !== routeId) }
          : item
      )
    );
    toast(`Removed route from ${target.name}`);
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

  const openAddComputer = () => toast("Opens Add computer dialog (Screen 03)");
  const openUpgrade = () => toast("Opens Upgrade sheet (Screen 01 · U9)");

  const overlay = (
    <>
      <RemoveComputerDialog
        computer={removeTarget}
        isFreePlan={account.plan === "free"}
        onClose={() => setRemoveTarget(null)}
        onConfirm={handleConfirmRemove}
      />
      <RevokeDeviceDialog
        device={revokeTarget}
        onClose={() => setRevokeTarget(null)}
        onConfirm={handleConfirmRevoke}
        requireDeviceApproval={local.requireDeviceApproval}
      />
      <DisableLocalAgentsDialog
        local={local}
        onClose={() => setConfirmDisableLocal(false)}
        onConfirm={() => {
          setLocal(current => ({ ...current, localRunAgents: false }));
          setConfirmDisableLocal(false);
        }}
        open={confirmDisableLocal}
      />
    </>
  );

  return (
    <RemoteSettingsShell
      activeSection="computers"
      headerActions={
        account.signedIn ? (
          <Button onClick={openAddComputer} size="sm" type="button" variant="secondary">
            <Plus aria-hidden="true" />
            Add computer
          </Button>
        ) : undefined
      }
      meta={buildSubheadMeta(account, computers)}
      overlay={overlay}
      title="Computers"
      width="wide"
    >
      <ComputersBanners
        account={account}
        onDismissUninstallWarning={() => setUninstallWarning(null)}
        onDismissUpgradeNotice={() => setUpgradeNotice(null)}
        onLearnMoreUpgrade={() => toast("Opens https://compozy.com/docs/computers")}
        onRetryCloud={() => {
          setAccount(current => ({ ...current, cloudUnreachable: false }));
          toast.success("Reconnected to Compozy Cloud.");
        }}
        onReviewWebhooks={() => toast("Opens Automation triggers to confirm new addresses")}
        uninstallWarning={uninstallWarning}
        upgradeNotice={upgradeNotice}
      />

      <ThisComputerSection
        account={account}
        defaultDevicesOpen={devicesOpen}
        devices={devices}
        hasRemoteComputers={computers.length > 0}
        local={local}
        onApprovePending={id => handleDecideApproval(id, true)}
        onDenyPending={id => handleDecideApproval(id, false)}
        onInspectHostedRoute={() => toast("Opens hosted route diagnosis details")}
        onRequestRevokeDevice={setRevokeTarget}
        onRequestToggleRunAgents={handleRequestToggleRunAgents}
        onToggleDeviceScope={handleToggleDeviceScope}
        onToggleHostedRoute={next =>
          setLocal(current => ({
            ...current,
            hostedRouteEnabled: next,
            hostedRouteState: next ? "starting" : "off",
          }))
        }
        onToggleRemoteEnabled={next => setLocal(current => ({ ...current, remoteEnabled: next }))}
        onToggleRequireApproval={next =>
          setLocal(current => ({ ...current, requireDeviceApproval: next }))
        }
        onUpgrade={openUpgrade}
        pendingApprovals={approvals}
      />

      {!account.signedIn ? (
        <SettingsGroup bare title="Computers">
          <Empty
            action={
              <Button
                onClick={() => toast("Opens Compozy sign-in (Settings → Account)")}
                size="sm"
                type="button"
              >
                Sign in
              </Button>
            }
            description="Sign in to your Compozy account to add remote computers over SSH or connect machines already linked to your account."
            framed
            icon={UserCheck}
            title="Sign in to use other computers"
          />
        </SettingsGroup>
      ) : (
        <>
          <ComputersListSection
            account={account}
            computers={computers}
            onAddComputer={openAddComputer}
            onOpenDetails={(target, tab) =>
              toast(`Opens Computer details for ${target.name} (${tab ?? "status"} tab)`)
            }
            onRemove={setRemoveTarget}
            onRemoveRoute={handleRemoveRoute}
            onRename={target => toast(`Rename ${target.name}`)}
            onToggleEnabled={handleToggleComputer}
            onUpgrade={openUpgrade}
            selectedComputerId={selectedComputerId}
          />

          <AccountComputersSection
            account={account}
            computers={unadded}
            onAddAccountComputer={handleAddAccountComputer}
            onUpgrade={openUpgrade}
          />

          <LoadBalancingSection
            computers={computers}
            defaultOpen={loadBalancingOpen}
            enabled={loadBalancingEnabled}
            local={local}
            onChangeComputerWeight={(id, weight: LoadBalancingWeight) =>
              setComputers(current =>
                current.map(item =>
                  item.id === id ? { ...item, loadBalancingWeight: weight } : item
                )
              )
            }
            onChangeLocalWeight={weight =>
              setLocal(current => ({ ...current, loadBalancingWeight: weight }))
            }
            onToggleEnabled={setLoadBalancingEnabled}
          />
        </>
      )}
    </RemoteSettingsShell>
  );
}
