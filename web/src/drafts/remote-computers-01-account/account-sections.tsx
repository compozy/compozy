import {
  CloudOff,
  ExternalLink,
  Info,
  KeyRound,
  RefreshCw,
  ShieldCheck,
  Sparkles,
  TriangleAlert,
} from "@compozy/ui/icons";

import { Alert, AlertDescription, AlertTitle, Button, Pill } from "@compozy/ui";

import { PlanUsageMeter } from "@/systems/account/components/plan-usage-meter";
import type {
  AccountDeviceFixture,
  AccountProvider,
  AccountStateFixture,
} from "@/systems/computers/mocks/remote-computers-fixtures";
import { SettingRow } from "@/systems/settings/components/setting-row";
import { SettingsGroup } from "@/systems/settings/components/settings-group";

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
  if (provider === "github") return "Signed in with GitHub";
  return "Signed in with Google";
}

export interface AccountBannersProps {
  account: AccountStateFixture;
  onRetryCloud: () => void;
  onManageBilling: () => void;
}

/**
 * Top-of-page notices for Settings → Account (one message, one place):
 * - Cloud unreachable: single warning + Try again
 * - Payment failed (`past_due`): single destructive alert + Manage billing
 * - Downgraded grace window: single info alert (Upgrade lives once in the Plan header)
 */
export function AccountBanners({ account, onRetryCloud, onManageBilling }: AccountBannersProps) {
  return (
    <>
      {account.cloudUnreachable ? (
        <Alert className="gap-y-1 py-2.5" variant="warning">
          <CloudOff aria-hidden="true" />
          <AlertTitle className="flex flex-wrap items-center justify-between gap-2">
            <span>Can&apos;t reach Compozy Cloud.</span>
            <Button onClick={onRetryCloud} size="xs" type="button" variant="secondary">
              <RefreshCw aria-hidden="true" />
              Try again
            </Button>
          </AlertTitle>
          <AlertDescription>
            Local work is unaffected. Cached account details stay visible until Compozy Cloud
            answers again.
          </AlertDescription>
        </Alert>
      ) : null}

      {account.signedIn && account.paymentIssue ? (
        <Alert className="gap-y-1 py-2.5" variant="destructive">
          <TriangleAlert aria-hidden="true" />
          <AlertTitle className="flex flex-wrap items-center justify-between gap-2">
            <span>Payment failed. Update your card.</span>
            <Button onClick={onManageBilling} size="xs" type="button" variant="secondary">
              <ExternalLink aria-hidden="true" />
              Manage billing
            </Button>
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

export function AccountIdentitySection({
  account,
  currentDevice,
  linkMismatchNotice = false,
  onRequestSignIn,
  onRequestUnlink,
  onRequestSignOut,
}: {
  account: AccountStateFixture;
  currentDevice: AccountDeviceFixture | undefined;
  linkMismatchNotice?: boolean;
  onRequestSignIn?: () => void;
  onRequestUnlink?: () => void;
  onRequestSignOut: () => void;
}) {
  if (account.source === "host_link") {
    return (
      <SettingsGroup data-testid="account-identity" title="Profile">
        <SettingRow
          control={
            <Button onClick={onRequestSignIn} size="sm" type="button">
              Sign in
            </Button>
          }
          description="Sign in on this computer to manage your plan, devices, email, and account tokens from here."
          label={`Linked to ${account.email} for remote access`}
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
                To sign in with another account, unlink this computer first.
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

  if (account.source === "account_token") {
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
          description={
            <span className="flex flex-col gap-0.5">
              <span>{account.email}</span>
              <span>
                Signed in via <code className="font-mono text-fg">COMPOZY_ACCOUNT_TOKEN</code>. Sign
                in with Google or GitHub for remote computers, devices, and billing.
              </span>
            </span>
          }
          label={`Account token "${tokenName}"`}
        />
      </SettingsGroup>
    );
  }

  const isAdmin = Boolean(currentDevice?.admin);
  const showTierPill = !account.paymentIssue && !account.cancelAtPeriodEnd;

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
            <span>{account.email}</span>
            <span aria-hidden="true" className="size-0.5 rounded-full bg-faint" />
            <span>{providerLabel(account.provider)}</span>
            <span aria-hidden="true" className="size-0.5 rounded-full bg-faint" />
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

export interface AccountPlanSectionProps {
  account: AccountStateFixture;
  downgradedNeedsProCount: number;
  onUpgrade: () => void;
  onManageBilling: () => void;
  onContactSupport: () => void;
}

export function AccountPlanSection({
  account,
  downgradedNeedsProCount,
  onUpgrade,
  onManageBilling,
  onContactSupport,
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

  const headerAction =
    account.plan === "pro" ? (
      account.paymentIssue ? undefined : (
        <div className="flex items-center gap-2">
          {account.cancelAtPeriodEnd ? (
            <Button onClick={onUpgrade} size="sm" type="button" variant="default">
              <Sparkles aria-hidden="true" />
              Resubscribe
            </Button>
          ) : null}
          <Button onClick={onManageBilling} size="sm" type="button" variant="secondary">
            <ExternalLink aria-hidden="true" />
            Manage billing
          </Button>
        </div>
      )
    ) : (
      <div className="flex items-center gap-2">
        {account.formerSubscriber ? (
          <Button onClick={onManageBilling} size="sm" type="button" variant="ghost">
            <ExternalLink aria-hidden="true" />
            Manage billing
          </Button>
        ) : null}
        <Button onClick={onUpgrade} size="sm" type="button">
          <Sparkles aria-hidden="true" />
          Upgrade
        </Button>
      </div>
    );

  return (
    <SettingsGroup
      action={headerAction}
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
            action={
              hostedAtCap ? (
                <Button onClick={onContactSupport} size="xs" type="button" variant="outline">
                  Contact support
                </Button>
              ) : undefined
            }
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
