import {
  AccountBanners,
  AccountCreateTokenDialog,
  AccountDeleteDialog,
  AccountDeleteSection,
  AccountDevicesSection,
  AccountEmailSection,
  AccountGraceSignInPrompt,
  AccountIdentitySection,
  AccountPendingApprovalsSection,
  AccountPlanSection,
  AccountRevokeDeviceDialog,
  AccountSignInNoticesSection,
  AccountSignOutDialog,
  AccountTokenCreatedDialog,
  AccountTokensSection,
  ConnectedUpgradeSheet,
  accountDeleteProInfoFromView,
  useAccountSettingsPage,
} from "@/systems/account";
import { SettingsPageFrame, SettingsPageState, useSettingsTopbar } from "@/systems/settings";

/**
 * Settings → Account (`_uiux.md` U3, U9):
 * - New sign-in notices (Revoke this device + Dismiss)
 * - Profile & sign-out (or sign-in flow when signed out / link-only / account token / grace)
 * - Payment-issue & downgraded banners with Manage billing
 * - Plan and usage with Upgrade sheet (U9) and Stripe Customer Portal (Manage billing)
 * - Pending device approvals (Approve / Deny / Delivering / Recent 24h)
 * - Devices across the account with admin trust promotion and Q3 revoke dialog
 * - Email marketing preference switch
 * - Account tokens (list, create with one-time secret reveal, revoke)
 * - Delete account (with Pro cancellation & 14-day refund block)
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

  if (view.isGraceMode) {
    return (
      <SettingsPageFrame
        description="Manage your CompozyOS account, plan usage, pending approvals, and trusted devices."
        slug="account"
        width="form"
      >
        <AccountGraceSignInPrompt onSignIn={view.handleGoToSignIn} />
      </SettingsPageFrame>
    );
  }

  const isFullSession = view.account.signedIn && view.account.source === "session";
  const deleteProInfo = accountDeleteProInfoFromView(view.account);

  return (
    <>
      <SettingsPageFrame
        description="Manage your CompozyOS account, plan usage, pending approvals, and trusted devices."
        slug="account"
        width="form"
      >
        <AccountBanners
          account={view.account}
          downgradedNeedsProCount={view.downgradedNeedsProCount}
          isOpeningBillingPortal={view.isOpeningBillingPortal}
          onManageBilling={isFullSession ? view.handleManageBilling : undefined}
          onRetryCloud={view.handleRetryCloud}
        />

        {isFullSession && view.recentSignIns.length > 0 ? (
          <AccountSignInNoticesSection
            currentDeviceIsAdmin={view.currentDeviceIsAdmin}
            isDismissing={view.isDismissingNotices}
            notices={view.recentSignIns}
            onDismiss={view.handleDismissSignInNotices}
            onRevokeNotice={view.handleRevokeSignInNotice}
          />
        ) : null}

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
              isOpeningBillingPortal={view.isOpeningBillingPortal}
              onManageBilling={view.handleManageBilling}
              onUpgrade={view.handleOpenUpgrade}
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

            <AccountEmailSection
              disabled={view.isUpdatingEmail}
              marketingOptIn={view.account.marketingOptIn}
              onToggleMarketingOptIn={view.handleToggleMarketingOptIn}
            />

            <AccountTokensSection
              onCreateToken={() => view.setCreateTokenOpen(true)}
              onRevokeToken={view.handleRevokeToken}
              revokingTokenId={view.revokingTokenId}
              tokens={view.tokens}
            />

            <AccountDeleteSection
              account={view.account}
              currentDeviceIsAdmin={view.currentDeviceIsAdmin}
              onManageBilling={view.handleManageBilling}
              onRequestDelete={() => view.setDeleteOpen(true)}
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

      <AccountCreateTokenDialog
        error={view.createTokenError}
        isPending={view.isCreatingToken}
        onClose={() => view.setCreateTokenOpen(false)}
        onCreate={view.handleCreateToken}
        open={view.createTokenOpen}
      />

      <AccountTokenCreatedDialog
        createdToken={view.createdToken}
        onClose={() => view.setCreatedToken(null)}
      />

      <AccountSignOutDialog
        error={view.signOutError}
        installationLinked={view.signOutRequiresUnlink}
        isPending={view.isSigningOut}
        onClose={() => view.setSignOutOpen(false)}
        onConfirm={view.handleConfirmSignOut}
        open={view.signOutOpen}
      />

      <AccountDeleteDialog
        email={view.account.email}
        error={view.deleteError}
        isPending={view.isDeletingAccount}
        linkedComputersCount={view.account.remoteComputersUsed}
        onClose={() => view.setDeleteOpen(false)}
        onConfirm={view.handleConfirmDeleteAccount}
        open={view.deleteOpen}
        subscription={deleteProInfo}
      />

      {view.fallbackUpgradeSheet ? (
        <ConnectedUpgradeSheet sheet={view.fallbackUpgradeSheet} />
      ) : null}
    </>
  );
}
