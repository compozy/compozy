import { Plus } from "@compozy/ui/icons";

import { Button, toast } from "@compozy/ui";

import {
  ACCOUNT_DEVICES,
  PENDING_APPROVALS,
  type AccountDeviceFixture,
  type AccountStateFixture,
  type AccountUnaddedComputerFixture,
  type ComputerFixture,
  type LocalComputerFixture,
  type PendingApprovalFixture,
  type UpgradeMigrationNoticeFixture,
} from "@/systems/computers/mocks/remote-computers-fixtures";

import { RemoteSettingsShell } from "../remote-computers-shared/remote-settings-shell";
import { ComputersBanners, ComputersListSection } from "./computers-list-section";
import {
  AccountComputersSection,
  DisableLocalAgentsDialog,
  LoadBalancingSection,
  RemoveComputerDialog,
  RevokeDeviceDialog,
} from "./computers-sections";
import { useComputersSettingsPage } from "./hooks/use-computers-settings-page";
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
  initialDisableLocalOpen?: boolean;
  initialRemoveTargetId?: string | null;
  initialRevokeDeviceId?: string | null;
  selectedComputerId?: string | null;
  uninstallWarningMessage?: string | null;
}

function openAddComputer() {
  toast.message("Opens Add computer dialog (Screen 03)");
}

function openUpgrade() {
  toast.message("Opens Upgrade sheet (Screen 01 · U9)");
}

function buildSubheadMeta(
  account: AccountStateFixture,
  computers: readonly ComputerFixture[]
): Array<{ key: string; content: string }> {
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
  initialDisableLocalOpen = false,
  initialRemoveTargetId = null,
  initialRevokeDeviceId = null,
  selectedComputerId = null,
  uninstallWarningMessage: initialUninstallWarning = null,
}: ComputersPageProps) {
  const page = useComputersSettingsPage({
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
  });

  const overlay = (
    <>
      <RemoveComputerDialog
        computer={page.removeTarget}
        isFreePlan={page.account.plan === "free"}
        onClose={() => page.setRemoveTarget(null)}
        onConfirm={page.handleConfirmRemove}
      />
      <RevokeDeviceDialog
        device={page.revokeTarget}
        localRemoteEnabled={page.local.remoteEnabled}
        onClose={() => page.setRevokeTarget(null)}
        onConfirm={page.handleConfirmRevoke}
        requireDeviceApproval={page.local.requireDeviceApproval}
      />
      <DisableLocalAgentsDialog
        local={page.local}
        onClose={() => page.setConfirmDisableLocal(false)}
        onConfirm={page.handleConfirmDisableLocal}
        open={page.confirmDisableLocal}
      />
    </>
  );

  return (
    <RemoteSettingsShell
      activeSection="computers"
      headerActions={
        <Button onClick={openAddComputer} size="sm" type="button" variant="secondary">
          <Plus aria-hidden="true" />
          Add computer
        </Button>
      }
      meta={buildSubheadMeta(page.account, page.computers)}
      overlay={overlay}
      title="Computers"
      width="wide"
    >
      <ComputersBanners
        account={page.account}
        onDismissUninstallWarning={() => page.setUninstallWarning(null)}
        onDismissUpgradeNotice={() => page.setUpgradeNotice(null)}
        onLearnMoreUpgrade={() => toast.message("Opens https://compozy.com/docs/computers")}
        onRetryCloud={page.handleRetryCloud}
        onReviewWebhooks={() => toast.message("Opens Automation triggers to confirm new addresses")}
        uninstallWarning={page.uninstallWarning}
        upgradeNotice={page.upgradeNotice}
      />

      <ThisComputerSection
        account={page.account}
        defaultDevicesOpen={devicesOpen}
        devices={page.devices}
        hasRemoteComputers={page.computers.length > 0}
        local={page.local}
        onApprovePending={id => page.handleDecideApproval(id, true)}
        onDenyPending={id => page.handleDecideApproval(id, false)}
        onInspectHostedRoute={() => toast.message("Opens hosted route diagnosis details")}
        onRequestRevokeDevice={page.setRevokeTarget}
        onRequestToggleRunAgents={page.handleRequestToggleRunAgents}
        onToggleDeviceScope={page.handleToggleDeviceScope}
        onToggleHostedRoute={page.handleToggleHostedRoute}
        onToggleRemoteEnabled={page.handleToggleRemoteEnabled}
        onToggleRequireApproval={page.handleToggleRequireApproval}
        onUpgrade={openUpgrade}
        pendingApprovals={page.approvals}
      />

      <ComputersListSection
        account={page.account}
        computers={page.computers}
        onAddComputer={openAddComputer}
        onOpenDetails={(target, tab) =>
          toast.message(`Opens Computer details for ${target.name} (${tab ?? "status"} tab)`)
        }
        onRemove={page.setRemoveTarget}
        onRemoveRoute={page.handleRemoveRoute}
        onRename={target => toast.message(`Rename ${target.name}`)}
        onToggleEnabled={page.handleToggleComputer}
        onUpgrade={openUpgrade}
        selectedComputerId={selectedComputerId}
      />

      <AccountComputersSection
        account={page.account}
        computers={page.unadded}
        onAddAccountComputer={page.handleAddAccountComputer}
        onUpgrade={openUpgrade}
      />

      <LoadBalancingSection
        computers={page.computers}
        defaultOpen={loadBalancingOpen}
        enabled={page.loadBalancingEnabled}
        local={page.local}
        onChangeComputerWeight={page.handleChangeComputerWeight}
        onChangeLocalWeight={page.handleChangeLocalWeight}
        onToggleEnabled={page.setLoadBalancingEnabled}
      />
    </RemoteSettingsShell>
  );
}
