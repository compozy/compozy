import { use } from "react";
import { Plus } from "@compozy/ui/icons";

import { Button } from "@compozy/ui";

import { AccountRevokeDeviceDialog, ConnectedUpgradeSheet } from "@/systems/account";
import {
  AccountComputersSection,
  AddComputerDialog,
  ComputerDetailsPanel,
  ComputersBanners,
  ComputersListSection,
  ComputersThisComputerSection,
  DisableLocalAgentsDialog,
  LoadBalancingSection,
  RemoveComputerDialog,
  RenameComputerDialog,
  useComputersSettingsPage,
} from "@/systems/computers";
import { OsShellContext } from "@/systems/os";
import { SettingsPageFrame, SettingsPageState, useSettingsTopbar } from "@/systems/settings";

/**
 * Settings → Computers (`_uiux.md` U1, U2, U7, U9):
 * - This computer (Run agents, Remote access, Hosted route Pro paywall/toggle,
 *   Require approval for new devices, Devices disclosure)
 * - Computers list with Free slot meter, status, Needs Pro Upgrade CTA, route, load,
 *   inline routes & permissions disclosures
 * - On your account (linked elsewhere, one-click Add or Pro paywall)
 * - Add computer modal (SSH with plan-limit Upgrade CTA, On your account, Link with a command)
 * - Right-docked Computer details panel
 */
export function ComputersSettingsPage() {
  const shell = use(OsShellContext);
  const view = useComputersSettingsPage({
    openTerminalWindow: spec => shell?.coordinator.userOpen(spec),
  });

  useSettingsTopbar({
    actions: (
      <Button
        data-testid="computers-add-button"
        onClick={() => view.setAddDialogOpen(true)}
        size="sm"
        type="button"
      >
        <Plus aria-hidden="true" />
        Add computer
      </Button>
    ),
  });

  if (view.isLoading) {
    return <SettingsPageState slug="computers" state="loading" />;
  }

  if (view.error) {
    return (
      <SettingsPageState
        error={view.error}
        onRetry={view.handleRetryCloud}
        slug="computers"
        state="error"
      />
    );
  }

  const connectedCount = view.computers.filter(
    item => item.status === "connected" || item.status === "needs_attention"
  ).length;
  const needsProCount = view.computers.filter(item => item.status === "needs_pro").length;

  return (
    <>
      <SettingsPageFrame
        description="Run agents on other machines, share this computer with your devices, and see how each connection is routed."
        meta={[
          {
            key: "connected",
            content: (
              <span data-testid="computers-connected-count">
                <span className="font-medium text-muted-foreground">{connectedCount}</span>{" "}
                {connectedCount === 1 ? "computer connected" : "computers connected"}
              </span>
            ),
          },
          ...(needsProCount > 0
            ? [
                {
                  key: "needs-pro",
                  content: (
                    <span data-testid="computers-needs-pro-count">{needsProCount} need Pro</span>
                  ),
                },
              ]
            : []),
          ...(view.account.signedIn &&
          view.account.plan === "free" &&
          view.account.remoteComputersLimit !== null
            ? [
                {
                  key: "free-slot",
                  content: (
                    <span data-testid="computers-free-slot">
                      {view.account.remoteComputersUsed} of {view.account.remoteComputersLimit}{" "}
                      remote computers used
                    </span>
                  ),
                },
              ]
            : []),
          ...(view.local.remoteEnabled
            ? [
                {
                  key: "remote-enabled",
                  content: <span data-testid="computers-remote-on">Remote access on</span>,
                },
              ]
            : []),
        ]}
        slug="computers"
        width="wide"
      >
        <ComputersBanners
          cloudUnreachable={view.cloudUnreachable}
          onDismissUninstallWarning={() => view.setUninstallWarning(null)}
          onRetryCloud={view.handleRetryCloud}
          uninstallWarning={view.uninstallWarning}
        />

        <ComputersThisComputerSection
          account={view.account}
          devices={view.localDevices}
          hasRemoteComputers={view.computers.length > 0}
          local={view.local}
          onApprovePending={approvalId => view.handleApproveLocalPending(approvalId)}
          onDenyPending={approvalId => view.handleDenyLocalPending(approvalId)}
          onRequestRevokeDevice={view.setRevokeTarget}
          onRequestToggleRunAgents={view.handleRequestToggleRunAgents}
          onToggleDeviceScope={view.handleToggleLocalDeviceScope}
          onToggleHostedRoute={view.handleToggleHostedRoute}
          onToggleRemoteEnabled={view.handleToggleRemoteEnabled}
          onToggleRequireApproval={view.handleToggleRequireApproval}
          onUpgrade={view.handleOpenUpgrade}
          pendingApprovals={view.localPendingApprovals}
        />

        <ComputersListSection
          account={view.account}
          computers={view.computers}
          onAddComputer={() => view.setAddDialogOpen(true)}
          onOpenDetails={(computer, tab) => view.openDetails(computer, tab)}
          onRemove={view.setRemoveTarget}
          onRemoveRoute={view.handleRemoveRoute}
          onRename={view.setRenameTarget}
          onToggleEnabled={view.handleToggleEnabled}
          onUpgrade={view.handleOpenUpgrade}
          selectedComputerId={view.selectedComputer?.id ?? null}
        />

        <AccountComputersSection
          account={view.account}
          addingComputerId={view.addingAccountId}
          computers={view.unaddedComputers}
          onAddAccountComputer={view.handleAddAccountComputer}
          onUpgrade={view.handleOpenUpgrade}
        />

        <LoadBalancingSection
          computers={view.computers}
          enabled={view.loadBalancingEnabled}
          isPending={view.isUpdatingLoadBalancing}
          local={view.local}
          onChangeComputerWeight={view.handleChangeComputerWeight}
          onChangeLocalWeight={view.handleChangeLocalWeight}
          onToggleEnabled={view.handleToggleLoadBalancing}
        />
      </SettingsPageFrame>

      {view.selectedComputer ? (
        <ComputerDetailsPanel
          computer={view.selectedComputer}
          currentDeviceName={view.currentDeviceName}
          editablePermissions={view.currentDeviceIsAdmin}
          initialTab={view.detailsTab}
          liveUpdateState={view.liveUpdateState}
          liveUpdateSteps={view.liveUpdateSteps}
          onCheckVersion={view.handleCheckVersion}
          onOpenChange={open => {
            if (!open) view.closeDetails();
          }}
          onRemoveRoute={routeId => {
            if (view.selectedComputer) {
              view.handleRemoveRoute(view.selectedComputer, routeId);
            }
          }}
          onRetryConnection={view.handleRetryConnection}
          onRetryProviders={view.selectedProviders.refetch}
          onSignInProvider={(provider, command) => {
            void view.selectedProviders.signInProvider(provider, command);
          }}
          onStartUpdate={() => {
            if (view.selectedComputer) {
              void view.handleStartUpdate(view.selectedComputer);
            }
          }}
          onToggleScope={(scope, next) => {
            if (view.selectedComputer) {
              view.handleToggleComputerScope(view.selectedComputer, scope, next);
            }
          }}
          open
          placement="pane"
          providersError={view.selectedProviders.error}
          providersLoading={view.selectedProviders.isLoading}
          showRouteReorder={false}
          signingInProviderId={view.selectedProviders.signingInProviderId}
        />
      ) : null}

      <AddComputerDialog
        accountEmail={view.account.email || "your account"}
        isFreePlan={view.account.plan === "free"}
        live={view.addComputer.controller}
        onClose={() => view.setAddDialogOpen(false)}
        onOpenComputer={name => {
          view.setAddDialogOpen(false);
          const match = view.computers.find(c => c.name === name);
          if (match) view.openDetails(match, "status");
        }}
        onUpgrade={view.handleOpenUpgradeFromAdd}
        open={view.addDialogOpen}
        sshSuggestions={view.addComputer.sshSuggestions}
        unaddedComputers={view.addComputer.unaddedComputers}
      />

      <RemoveComputerDialog
        computer={view.removeTarget}
        isFreePlan={view.account.plan === "free"}
        isPending={view.isRemoving}
        onClose={() => view.setRemoveTarget(null)}
        onConfirm={view.handleConfirmRemove}
      />

      <RenameComputerDialog
        computer={view.renameTarget}
        isPending={view.isRenaming}
        onClose={() => view.setRenameTarget(null)}
        onConfirm={view.handleConfirmRename}
      />

      <AccountRevokeDeviceDialog
        approvalFailure={view.revokeApprovalFailure}
        device={view.revokeTarget}
        isPending={view.isRevoking}
        localRemoteEnabled={view.local.remoteEnabled}
        onClose={view.closeRevokeDialog}
        onConfirm={view.handleConfirmRevokeDevice}
        onRetryEnableApproval={view.handleRetryEnableApproval}
        requireDeviceApproval={view.local.requireDeviceApproval}
      />

      <DisableLocalAgentsDialog
        isPending={view.isUpdatingLocalSettings}
        local={view.local}
        onClose={() => view.setDisableAgentsOpen(false)}
        onConfirm={view.handleConfirmDisableAgents}
        open={view.disableAgentsOpen}
      />

      {view.fallbackUpgradeSheet ? (
        <ConnectedUpgradeSheet sheet={view.fallbackUpgradeSheet} />
      ) : null}
    </>
  );
}
