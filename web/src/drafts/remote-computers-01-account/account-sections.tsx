import {
  CloudOff,
  ExternalLink,
  Info,
  KeyRound,
  LogIn,
  RefreshCw,
  ShieldCheck,
  Sparkles,
  TriangleAlert,
  UserCheck,
} from "@compozy/ui/icons";

import {
  Alert,
  AlertDescription,
  AlertTitle,
  Button,
  CopyIconButton,
  Empty,
  Pill,
} from "@compozy/ui";

import { PlanUsageMeter } from "@/systems/account/components/plan-usage-meter";
import {
  StepProgressList,
  type StepProgressItem,
} from "@/systems/computers/components/step-progress-list";
import type {
  AccountDeviceFixture,
  AccountStateFixture,
} from "@/systems/computers/mocks/remote-computers-fixtures";
import { ACCOUNT_DEVICE_FLOW } from "@/systems/computers/mocks/remote-computers-fixtures-lane-a";
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

export function AccountSignedOutSection({
  onSignIn,
  onSignInWithCode,
}: {
  onSignIn: () => void;
  onSignInWithCode: () => void;
}) {
  return (
    <SettingsGroup bare title="Account">
      <Empty
        action={
          <>
            <Button onClick={onSignIn} size="sm" type="button">
              <LogIn aria-hidden="true" />
              Sign in
            </Button>
            <Button onClick={onSignInWithCode} size="sm" type="button" variant="outline">
              <KeyRound aria-hidden="true" />
              Sign in with a code
            </Button>
          </>
        }
        description="Sign in to link your computers, connect from your phone, and manage trusted devices. Everything on this computer works without an account."
        framed
        icon={UserCheck}
        title="Sign in to Compozy"
      />
    </SettingsGroup>
  );
}

export function AccountSigningInSection({
  onCancel,
  onOpenBrowser,
}: {
  onCancel: () => void;
  onOpenBrowser: () => void;
}) {
  const steps: StepProgressItem[] = [
    {
      id: "open",
      title: "Opened Compozy sign-in in your browser",
      status: "done",
      action: (
        <Button onClick={onOpenBrowser} size="xs" type="button" variant="ghost">
          <ExternalLink aria-hidden="true" />
          Open again
        </Button>
      ),
    },
    {
      id: "wait",
      title: "Waiting for approval in your browser…",
      status: "running",
      detail: ACCOUNT_DEVICE_FLOW.expiresLabel,
      caption: "Settings updates automatically once you confirm this code in your browser.",
      prompt: (
        <div className="flex flex-col gap-2 pt-1">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-mono text-ui-title-lg font-semibold tracking-[0.22em] text-fg-strong tabular-nums select-all">
              {ACCOUNT_DEVICE_FLOW.userCode}
            </span>
            <CopyIconButton
              className="text-subtle hover:text-fg"
              copiedLabel="Code copied"
              copiedToastLabel="Verification code copied"
              copyLabel="Copy verification code"
              value={ACCOUNT_DEVICE_FLOW.userCode}
            />
          </div>
          <div className="flex flex-wrap items-center gap-1.5 text-meta text-muted-foreground">
            <span className="font-mono text-fg select-all">
              {ACCOUNT_DEVICE_FLOW.verificationUri}
            </span>
            <CopyIconButton
              className="size-5 text-subtle hover:text-fg"
              copiedLabel="Link copied"
              copiedToastLabel="Activation link copied"
              copyLabel="Copy activation link"
              value={ACCOUNT_DEVICE_FLOW.verificationUri}
            />
          </div>
        </div>
      ),
    },
  ];

  return (
    <SettingsGroup
      action={
        <Button onClick={onCancel} size="sm" type="button" variant="outline">
          Cancel
        </Button>
      }
      data-testid="account-signing-in"
      description="Approve this device in your browser or enter the code from another device."
      title="Sign in to Compozy"
    >
      <div className="px-4 py-4">
        <StepProgressList steps={steps} />
      </div>
      <p className="border-t border-line-soft px-4 py-3 text-meta text-muted-foreground">
        Headless machine? Run <code className="font-mono text-fg">compozy account login</code> in a
        terminal.
      </p>
    </SettingsGroup>
  );
}

export function AccountIdentitySection({
  account,
  currentDevice,
  onSignOut,
}: {
  account: AccountStateFixture;
  currentDevice: AccountDeviceFixture | undefined;
  onSignOut: () => void;
}) {
  const isAdmin = Boolean(currentDevice?.admin);
  const showTierPill = !account.paymentIssue && !account.cancelAtPeriodEnd;

  return (
    <SettingsGroup data-testid="account-identity" title="Account">
      <SettingRow
        control={
          <Button onClick={onSignOut} size="sm" type="button" variant="outline">
            Sign out
          </Button>
        }
        description={
          <span className="inline-flex flex-wrap items-center gap-1.5">
            <span>Signed in on {currentDevice?.name ?? "This device"}</span>
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
            <span>{account.email}</span>
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
