import { ExternalLink, Sparkles } from "@compozy/ui/icons";

import { Button, Pill, buttonVariants } from "@compozy/ui";

import type { AccountStateFixture } from "@/systems/computers/mocks/remote-computers-types";
import { SettingsGroup } from "@/systems/settings/components/settings-group";

import { COMPOZY_SUPPORT_MAILTO } from "../lib/account-view";
import { PlanUsageMeter } from "./plan-usage-meter";

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

export interface AccountPlanSectionProps {
  account: AccountStateFixture;
  downgradedNeedsProCount?: number;
  isOpeningBillingPortal?: boolean;
  onUpgrade?: () => void;
  onManageBilling?: () => void;
}

/**
 * Plan and usage block for Settings → Account (`_uiux.md` U3, US-002, US-004):
 * - Free: Upgrade CTA (plus Manage billing for former subscribers)
 * - Pro (Stripe): Manage billing (plus Resubscribe when ending; omitted when paymentIssue banner owns it)
 * - Pro (Complimentary): `Pro · complimentary until <date>` with Manage billing absent
 * - Downgraded: stacked active / needs Pro usage meter
 */
export function AccountPlanSection({
  account,
  downgradedNeedsProCount = 0,
  isOpeningBillingPortal = false,
  onUpgrade,
  onManageBilling,
}: AccountPlanSectionProps) {
  const renewsLabel = formatDateLabel(account.renewsAt);
  const endsLabel = formatDateLabel(account.endsAt);
  const isComplimentary = account.plan === "pro" && account.subscriptionSource === "complimentary";
  const hostedAtCap =
    account.plan === "pro" &&
    account.cloudRoutesLimit > 0 &&
    account.cloudRoutesUsed >= account.cloudRoutesLimit;

  const description = (() => {
    if (isComplimentary) {
      return endsLabel
        ? `Compozy Pro · complimentary until ${endsLabel}`
        : "Compozy Pro · complimentary";
    }
    if (account.plan === "pro" && account.cancelAtPeriodEnd) {
      return endsLabel
        ? `Compozy Pro · Ends ${endsLabel}. After that, 1 computer stays active on Free.`
        : "Compozy Pro · Ends at period end.";
    }
    if (account.plan === "pro") {
      return renewsLabel ? `Compozy Pro · Renews ${renewsLabel}` : "Compozy Pro";
    }
    if (account.formerSubscriber || downgradedNeedsProCount > 0) {
      return "Free plan";
    }
    return "Everything on this computer stays free. Pro adds unlimited computers, hosted routes, and phone access.";
  })();

  const headerAction = (() => {
    if (account.plan === "pro") {
      if (isComplimentary || account.paymentIssue) {
        return undefined;
      }
      if (!onManageBilling && !(account.cancelAtPeriodEnd && onUpgrade)) {
        return undefined;
      }
      return (
        <div className="flex items-center gap-2">
          {account.cancelAtPeriodEnd && onUpgrade ? (
            <Button onClick={onUpgrade} size="sm" type="button" variant="default">
              <Sparkles aria-hidden="true" />
              Resubscribe
            </Button>
          ) : null}
          {onManageBilling ? (
            <Button
              disabled={isOpeningBillingPortal}
              onClick={onManageBilling}
              size="sm"
              type="button"
              variant="secondary"
            >
              <ExternalLink aria-hidden="true" />
              Manage billing
            </Button>
          ) : null}
        </div>
      );
    }

    if (!onUpgrade && !(account.formerSubscriber && onManageBilling)) {
      return undefined;
    }

    return (
      <div className="flex items-center gap-2">
        {account.formerSubscriber && onManageBilling ? (
          <Button
            disabled={isOpeningBillingPortal}
            onClick={onManageBilling}
            size="sm"
            type="button"
            variant="ghost"
          >
            <ExternalLink aria-hidden="true" />
            Manage billing
          </Button>
        ) : null}
        {onUpgrade ? (
          <Button onClick={onUpgrade} size="sm" type="button">
            <Sparkles aria-hidden="true" />
            Upgrade
          </Button>
        ) : null}
      </div>
    );
  })();

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
                <a
                  className={buttonVariants({ size: "xs", variant: "outline" })}
                  href={COMPOZY_SUPPORT_MAILTO}
                >
                  Contact support
                </a>
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
