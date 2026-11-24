import {
  AccountBanners,
  AccountDevicesSection,
  AccountIdentitySection,
  AccountPendingApprovalsSection,
  AccountPlanSection,
  AccountRevokeDeviceDialog,
  AccountSignOutDialog,
  useAccountSettingsPage,
} from "@/systems/account";
import { SettingsPageFrame, SettingsPageState, useSettingsTopbar } from "@/systems/settings";

/**
 * Settings → Account (`_uiux.md` U3):
 * - Profile & sign-out (or sign-in flow when signed out / link-only / account token)
 * - Plan and usage display (checkout & Upgrade sheet land in S12)
 * - Pending device approvals (Approve / Deny / Delivering / Recent 24h)
 * - Devices across the account with admin trust promotion and Q3 revoke dialog
 */
export function AccountSettingsPage() {
  const view = useAccountSettingsPage();
  useSettingsTopbar();

  if (view.isLoading) {
    return <SettingsPageState slug="account" state="loading" />;
  }

  if (view.error || !view.account) {
    return (
      <SettingsPageState
        error={view.error}
        onRetry={view.handleRetryCloud}
        slug="account"
        state="error"
      />
    );
  }

  const isFullSession = view.account.signedIn && view.account.source === "session";

  return (
    <>
      <SettingsPageFrame
        description="Manage your CompozyOS account, plan usage, pending approvals, and trusted devices."
        slug="account"
        width="form"
      >
        <AccountBanners account={view.account} onRetryCloud={view.handleRetryCloud} />

        <AccountIdentitySection
          account={view.account}
          currentDevice={view.currentDevice}
          isStartingLogin={view.isStartingLogin}
          linkMismatchNotice={view.linkMismatchNotice}
          loginAttempt={view.loginAttempt}
          loginError={view.loginError}
          onCancelSignIn={view.handleCancelSignIn}
          onRequestSignOut={() => view.setSignOutOpen(true)}
          onRequestUnlink={view.handleUnlinkLocal}
          onStartSignIn={view.handleStartSignIn}
        />

        {isFullSession ? (
          <>
            <AccountPlanSection
              account={view.account}
              downgradedNeedsProCount={view.downgradedNeedsProCount}
            />

            <AccountPendingApprovalsSection
              approvals={view.approvals}
              onApprove={view.handleApprove}
              onDeny={view.handleDeny}
            />

            <AccountDevicesSection
              currentDeviceIsAdmin={view.currentDeviceIsAdmin}
              devices={view.devices}
              onRequestRevoke={view.setRevokeTarget}
              onToggleAdmin={view.handleToggleAdmin}
            />
          </>
        ) : null}
      </SettingsPageFrame>

      <AccountRevokeDeviceDialog
        approvalFailure={view.revokeApprovalFailure}
        device={view.revokeTarget}
        isPending={view.isRevoking}
        localRemoteEnabled={view.localRemoteEnabled}
        onClose={view.handleCloseRevoke}
        onConfirm={view.handleConfirmRevoke}
        onRetryEnableApproval={view.handleRetryEnableApproval}
        requireDeviceApproval={view.requireDeviceApproval}
      />

      <AccountSignOutDialog
        error={view.signOutError}
        installationLinked={view.account.installationLinked}
        isPending={view.isSigningOut}
        onClose={() => view.setSignOutOpen(false)}
        onConfirm={view.handleConfirmSignOut}
        open={view.signOutOpen}
      />
    </>
  );
}
