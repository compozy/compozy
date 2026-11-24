import {
  CloudOff,
  ExternalLink,
  Info,
  KeyRound,
  RefreshCw,
  ShieldCheck,
  TriangleAlert,
} from "@compozy/ui/icons";

import { Alert, AlertDescription, AlertTitle, Button, Pill, Spinner } from "@compozy/ui";

import type {
  AccountDeviceFixture,
  AccountProvider,
  AccountStateFixture,
} from "@/systems/computers/mocks/remote-computers-types";
import { SettingRow } from "@/systems/settings/components/setting-row";
import { SettingsGroup } from "@/systems/settings/components/settings-group";

import type { AccountLoginResponse } from "../adapters/account-api";
import { PlanUsageMeter } from "./plan-usage-meter";

const COMPOZY_STATUS_URL = "https://status.compozy.com";

function formatDateLabel(iso: string | null): string | null {
  if (!iso) return null;
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return null;
  return date.toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    timeZone: "UTC",
  });
}

function providerLabel(provider: AccountProvider | null): string {
  return provider === "github" ? "Signed in with GitHub" : "Signed in with Google";
}

export interface AccountBannersProps {
  account: AccountStateFixture;
  onRetryCloud: () => void;
  onManageBilling?: () => void;
}

function CloudUnreachableActions({ onRetryCloud }: { onRetryCloud: () => void }) {
  return (
    <div className="flex shrink-0 items-center gap-1.5">
      <Button onClick={onRetryCloud} size="xs" type="button" variant="secondary">
        <RefreshCw aria-hidden="true" />
        Try again
      </Button>
      <a
        className="inline-flex h-6 items-center gap-1 rounded-sm px-2 text-eyebrow font-medium text-fg hover:bg-surface-2"
        href={COMPOZY_STATUS_URL}
        rel="noreferrer"
        target="_blank"
      >
        Status
        <ExternalLink aria-hidden="true" className="size-3" />
      </a>
    </div>
  );
}

/**
 * Top-of-page notices for Settings → Account (`_uiux.md` U3):
 * - Cloud unreachable: warning + Try again + Status link
 * - Payment failed (`past_due`): destructive alert (billing action wired in S12)
 * - Downgraded grace window: info alert
 */
export function AccountBanners({ account, onRetryCloud, onManageBilling }: AccountBannersProps) {
  return (
    <>
      {account.cloudUnreachable ? (
        <Alert className="gap-y-1 py-2.5" data-testid="account-cloud-unreachable" variant="warning">
          <CloudOff aria-hidden="true" />
          <AlertTitle className="flex flex-wrap items-center justify-between gap-2">
            <span>Can&apos;t reach Compozy Cloud.</span>
            <CloudUnreachableActions onRetryCloud={onRetryCloud} />
          </AlertTitle>
          <AlertDescription>
            Local work is unaffected. Cached account details stay visible until Compozy Cloud
            answers again.
          </AlertDescription>
        </Alert>
      ) : null}

      {account.signedIn && account.paymentIssue ? (
        <Alert className="gap-y-1 py-2.5" data-testid="account-payment-issue" variant="destructive">
          <TriangleAlert aria-hidden="true" />
          <AlertTitle className="flex flex-wrap items-center justify-between gap-2">
            <span>Payment failed. Update your card.</span>
            {onManageBilling ? (
              <Button onClick={onManageBilling} size="xs" type="button" variant="secondary">
                <ExternalLink aria-hidden="true" />
                Manage billing
              </Button>
            ) : null}
          </AlertTitle>
          <AlertDescription>Pro stays active while Stripe retries your payment.</AlertDescription>
        </Alert>
      ) : null}

      {account.signedIn && account.hostedRouteGraceDaysLeft !== null ? (
        <Alert className="gap-y-1 py-2.5" variant="info">
          <Info aria-hidden="true" />
          <AlertTitle>Pro ended. 1 computer stays active.</AlertTitle>
          <AlertDescription>
            Your most recently used computer stays connected over LAN and SSH. Hosted route
            addresses stay reserved for {account.hostedRouteGraceDaysLeft} more days so you can
            re-subscribe without re-adding computers.
          </AlertDescription>
        </Alert>
      ) : null}
    </>
  );
}

export interface AccountIdentitySectionProps {
  account: AccountStateFixture;
  currentDevice: AccountDeviceFixture | undefined;
  linkMismatchNotice?: boolean;
  loginAttempt?: AccountLoginResponse | null;
  loginError?: string | null;
  isStartingLogin?: boolean;
  onStartSignIn?: (provider: AccountProvider) => void;
  onCancelSignIn?: () => void;
  onRequestUnlink?: () => void;
  onRequestSignOut: () => void;
}

function SignedOutIdentityCard({
  loginAttempt,
  loginError,
  isStartingLogin,
  onStartSignIn,
  onCancelSignIn,
}: Pick<
  AccountIdentitySectionProps,
  "loginAttempt" | "loginError" | "isStartingLogin" | "onStartSignIn" | "onCancelSignIn"
>) {
  return (
    <SettingsGroup data-testid="account-identity" title="Profile">
      {loginAttempt ? (
        <div
          className="flex flex-col gap-3 px-4 py-4"
          data-testid="account-sign-in-waiting"
          role="status"
        >
          <div className="flex flex-wrap items-center justify-between gap-2">
            <span className="inline-flex items-center gap-2 text-small-body font-medium text-fg-strong">
              <Spinner aria-hidden="true" className="size-3.5" />
              <span>Continue in your browser</span>
            </span>
            {onCancelSignIn ? (
              <Button onClick={onCancelSignIn} size="xs" type="button" variant="outline">
                Cancel
              </Button>
            ) : null}
          </div>
          <div className="flex flex-wrap items-center gap-3">
            <code
              className="rounded-md border border-line bg-code-bg px-3 py-1.5 font-mono text-base font-semibold tracking-widest text-fg-strong select-all"
              data-testid="account-sign-in-code"
            >
              {loginAttempt.user_code}
            </code>
            <a
              className="inline-flex items-center gap-1.5 text-small-body font-medium text-fg underline underline-offset-3"
              href={loginAttempt.verification_uri_complete || loginAttempt.verification_uri}
              rel="noreferrer"
              target="_blank"
            >
              Open verification page
              <ExternalLink aria-hidden="true" className="size-3.5" />
            </a>
          </div>
          <p className="text-meta text-muted-foreground">
            Check that your browser shows the same code.
          </p>
        </div>
      ) : (
        <SettingRow
          control={
            <div className="flex flex-wrap items-center gap-2">
              <Button
                disabled={isStartingLogin || !onStartSignIn}
                onClick={() => onStartSignIn?.("google")}
                size="sm"
                type="button"
                variant="secondary"
              >
                Continue with Google
              </Button>
              <Button
                disabled={isStartingLogin || !onStartSignIn}
                onClick={() => onStartSignIn?.("github")}
                size="sm"
                type="button"
                variant="outline"
              >
                Continue with GitHub
              </Button>
            </div>
          }
          description="Sign in to add remote computers, manage devices, and check your plan."
          label="Not signed in"
        />
      )}
      {loginError ? (
        <div
          className="border-t border-line-soft px-4 py-2.5 text-meta text-destructive"
          role="alert"
        >
          {loginError}
        </div>
      ) : null}
    </SettingsGroup>
  );
}

function LinkOnlyIdentityCard({
  email,
  linkMismatchNotice,
  isStartingLogin,
  onStartSignIn,
  onRequestUnlink,
}: {
  email: string;
  linkMismatchNotice: boolean;
  isStartingLogin: boolean;
  onStartSignIn?: (provider: AccountProvider) => void;
  onRequestUnlink?: () => void;
}) {
  return (
    <SettingsGroup data-testid="account-identity" title="Profile">
      <SettingRow
        control={
          <Button
            disabled={isStartingLogin || !onStartSignIn}
            onClick={() => onStartSignIn?.("google")}
            size="sm"
            type="button"
          >
            Sign in
          </Button>
        }
        description="Sign in on this computer to manage your plan, devices, email, and account tokens from here."
        label={`Linked to ${email} for remote access`}
      />
      {linkMismatchNotice ? (
        <div
          className="flex flex-wrap items-center justify-between gap-3 border-t border-line-soft bg-warning-tint/50 px-4 py-3"
          data-testid="account-link-mismatch-notice"
          role="alert"
        >
          <div className="flex min-w-0 flex-1 items-start gap-2.5 text-small-body text-fg">
            <TriangleAlert aria-hidden="true" className="mt-0.5 size-4 shrink-0 text-warning" />
            <span>
              This computer is linked to {email} for remote access. Unlink it before signing in with
              another account.
            </span>
          </div>
          <Button
            className="shrink-0"
            onClick={onRequestUnlink}
            size="sm"
            type="button"
            variant="outline"
          >
            Unlink
          </Button>
        </div>
      ) : null}
    </SettingsGroup>
  );
}

function TokenIdentityCard({ account }: { account: AccountStateFixture }) {
  const tokenName = account.tokenName ?? "ci";
  return (
    <SettingsGroup data-testid="account-identity" title="Profile">
      <SettingRow
        control={
          <Pill size="xs" tone="neutral">
            <KeyRound aria-hidden="true" className="size-3" />
            Local use only
          </Pill>
        }
        description={`${account.email} · Signed in via COMPOZY_ACCOUNT_TOKEN. Sign in with Google or GitHub for remote computers, devices, and billing.`}
        label={`Account token "${tokenName}"`}
      />
    </SettingsGroup>
  );
}

function SessionIdentityCard({
  account,
  isAdmin,
  onRequestSignOut,
}: {
  account: AccountStateFixture;
  isAdmin: boolean;
  onRequestSignOut: () => void;
}) {
  const showTierPill = !account.paymentIssue && !account.cancelAtPeriodEnd;
  const subtitle = `${account.email} · ${providerLabel(account.provider)}`;

  return (
    <SettingsGroup data-testid="account-identity" title="Profile">
      <SettingRow
        control={
          <Button onClick={onRequestSignOut} size="sm" type="button" variant="outline">
            Sign out
          </Button>
        }
        description={
          <span className="inline-flex flex-wrap items-center gap-1.5">
            <span>{subtitle}</span>
            <span aria-hidden="true">·</span>
            {isAdmin ? (
              <span className="inline-flex items-center gap-1 text-fg">
                <ShieldCheck aria-hidden="true" className="size-3.5 text-info" />
                <span>Trusted for admin</span>
              </span>
            ) : (
              <span>Standard device</span>
            )}
          </span>
        }
        label={
          <span className="inline-flex flex-wrap items-center gap-2">
            <span>{account.name}</span>
            {showTierPill ? (
              <Pill size="xs" tone={account.plan === "pro" ? "brand" : "neutral"}>
                {account.plan === "pro" ? "Pro" : "Free"}
              </Pill>
            ) : null}
          </span>
        }
      />
    </SettingsGroup>
  );
}

export function AccountIdentitySection({
  account,
  currentDevice,
  linkMismatchNotice = false,
  loginAttempt = null,
  loginError = null,
  isStartingLogin = false,
  onStartSignIn,
  onCancelSignIn,
  onRequestUnlink,
  onRequestSignOut,
}: AccountIdentitySectionProps) {
  if (!account.signedIn) {
    return (
      <SignedOutIdentityCard
        isStartingLogin={isStartingLogin}
        loginAttempt={loginAttempt}
        loginError={loginError}
        onCancelSignIn={onCancelSignIn}
        onStartSignIn={onStartSignIn}
      />
    );
  }
  if (account.source === "host_link") {
    return (
      <LinkOnlyIdentityCard
        email={account.email}
        isStartingLogin={isStartingLogin}
        linkMismatchNotice={linkMismatchNotice}
        onRequestUnlink={onRequestUnlink}
        onStartSignIn={onStartSignIn}
      />
    );
  }
  if (account.source === "account_token") {
    return <TokenIdentityCard account={account} />;
  }
  return (
    <SessionIdentityCard
      account={account}
      isAdmin={Boolean(currentDevice?.admin)}
      onRequestSignOut={onRequestSignOut}
    />
  );
}

export interface AccountPlanSectionProps {
  account: AccountStateFixture;
  downgradedNeedsProCount?: number;
}

/**
 * Plan and usage display for Settings → Account (`_uiux.md` U3).
 * Checkout, Upgrade sheet, and Customer Portal actions belong to S12.
 */
export function AccountPlanSection({
  account,
  downgradedNeedsProCount = 0,
}: AccountPlanSectionProps) {
  const renewsLabel = formatDateLabel(account.renewsAt);
  const endsLabel = formatDateLabel(account.endsAt);
  const hostedAtCap =
    account.plan === "pro" &&
    account.cloudRoutesLimit > 0 &&
    account.cloudRoutesUsed >= account.cloudRoutesLimit;

  const description = (() => {
    if (account.plan === "pro" && account.cancelAtPeriodEnd) {
      return endsLabel
        ? `Compozy Pro · Ends ${endsLabel}. After that, 1 computer stays active on Free.`
        : "Compozy Pro · Ends at period end.";
    }
    if (account.plan === "pro") {
      return renewsLabel ? `Compozy Pro · Renews ${renewsLabel}` : "Compozy Pro";
    }
    if (account.formerSubscriber) {
      return "Free plan";
    }
    return "Everything on this computer stays free. Pro adds unlimited computers, hosted routes, and phone access.";
  })();

  return (
    <SettingsGroup
      data-testid="account-plan"
      description={description}
      title={
        account.plan === "pro" && account.cancelAtPeriodEnd ? (
          <span className="inline-flex items-center gap-2">
            <span>Plan</span>
            <Pill size="xs" tone="warning">
              Ending
            </Pill>
          </span>
        ) : (
          "Plan"
        )
      }
    >
      <div className="flex flex-col divide-y divide-line-soft">
        <div className="px-4 py-3.5">
          <PlanUsageMeter
            label="Remote computers"
            limit={account.remoteComputersLimit}
            needsProCount={downgradedNeedsProCount}
            unitLabel="remote computers"
            used={account.remoteComputersUsed}
          />
        </div>

        <div className="flex flex-col gap-2 px-4 py-3.5">
          <PlanUsageMeter
            label="Hosted routes"
            limit={account.cloudRoutesLimit}
            lockedLabel="Part of Pro"
            unitLabel="hosted routes"
            used={account.cloudRoutesUsed}
          />
          {hostedAtCap ? (
            <p className="text-meta text-muted-foreground">
              Pro includes 10 computers on the hosted route. Contact support to raise it.
            </p>
          ) : null}
        </div>
      </div>
    </SettingsGroup>
  );
}
