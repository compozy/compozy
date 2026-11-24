import { useState } from "react";
import {
  CheckCircle2,
  ExternalLink,
  Globe,
  Info,
  Laptop,
  RefreshCw,
  Smartphone,
  Sparkles,
  TriangleAlert,
  Webhook,
} from "@compozy/ui/icons";

import {
  ActionResultBanner,
  Alert,
  AlertDescription,
  Button,
  Dialog,
  DialogContent,
  DialogFooter,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogHeader,
  Pill,
  RadioCardGroup,
  Spinner,
  dialogShellClass,
  type RadioCardGroupOption,
} from "@compozy/ui";

import {
  StepProgressList,
  type StepProgressItem,
} from "@/systems/computers/components/step-progress-list";
import {
  ACCOUNT_PRO_BENEFITS,
  ACCOUNT_PRO_PLAN_OPTIONS,
} from "@/systems/computers/mocks/remote-computers-fixtures-lane-a";

export type UpgradeSheetPhase = "select" | "waiting" | "waiting_timeout" | "done";
export type UpgradeInterval = "year" | "month";

export interface UpgradeSheetProps {
  open: boolean;
  /** Controlled phase for live checkout flows; omit for standalone/prototype usage. */
  phase?: UpgradeSheetPhase;
  initialPhase?: UpgradeSheetPhase;
  /** Controlled interval for live checkout flows; omit for standalone/prototype usage. */
  interval?: UpgradeInterval;
  initialInterval?: UpgradeInterval;
  onIntervalChange?: (next: UpgradeInterval) => void;
  /** Shown on `select` when returning from a cancelled checkout wait. */
  checkoutCancelled?: boolean;
  /** Shown on `select` when `POST /api/account/checkout` fails. */
  checkoutError?: string | null;
  isStartingCheckout?: boolean;
  isCheckingStatus?: boolean;
  onClose: () => void;
  onCancelWaiting?: () => void;
  onContinueToCheckout?: (interval: UpgradeInterval) => void;
  onReopenCheckout?: () => void;
  onCheckAgain?: () => void;
  onCompleteUpgrade?: () => void;
}

const BENEFIT_ICONS = {
  computers: Laptop,
  "hosted-route": Globe,
  phone: Smartphone,
  webhooks: Webhook,
} as const;

/**
 * Upgrade to Compozy Pro dialog (`_uiux.md` U9, US-002):
 * - Annual pre-selected with monthly equivalent and savings badge
 * - Tax line and concise 4-item Pro benefit list
 * - Live post-checkout states: Waiting for checkout, 15m Check again, Cancelled, You're on Pro
 */
export function UpgradeSheet({
  open,
  phase: controlledPhase,
  initialPhase = "select",
  interval: controlledInterval,
  initialInterval = "year",
  onIntervalChange,
  checkoutCancelled = false,
  checkoutError = null,
  isStartingCheckout = false,
  isCheckingStatus = false,
  onClose,
  onCancelWaiting,
  onContinueToCheckout,
  onReopenCheckout,
  onCheckAgain,
  onCompleteUpgrade,
}: UpgradeSheetProps) {
  const [uncontrolledPhase, setUncontrolledPhase] = useState<UpgradeSheetPhase>(initialPhase);
  const [uncontrolledInterval, setUncontrolledInterval] =
    useState<UpgradeInterval>(initialInterval);

  const isControlledPhase = controlledPhase !== undefined;
  const phase = isControlledPhase ? controlledPhase : uncontrolledPhase;
  const interval = controlledInterval ?? uncontrolledInterval;

  if (!open) return null;

  const selectedPlan =
    ACCOUNT_PRO_PLAN_OPTIONS.find(option => option.interval === interval) ??
    ACCOUNT_PRO_PLAN_OPTIONS[0]!;

  const handleIntervalChange = (next: UpgradeInterval) => {
    if (controlledInterval === undefined) {
      setUncontrolledInterval(next);
    }
    onIntervalChange?.(next);
  };

  const planOptions: RadioCardGroupOption<UpgradeInterval>[] = ACCOUNT_PRO_PLAN_OPTIONS.map(
    option => ({
      value: option.interval,
      title: option.title,
      badge: option.badge ? (
        <Pill size="xs" tone="brand">
          {option.badge}
        </Pill>
      ) : undefined,
      description: (
        <span className="mt-1 flex flex-col gap-0.5">
          <span className="text-heading font-semibold text-fg-strong tabular-nums">
            {option.price}
          </span>
          <span className="text-meta text-muted-foreground">{option.cadenceDetail}</span>
        </span>
      ),
      className: "p-3.5",
    })
  );

  const waitingSteps: StepProgressItem[] = [
    {
      id: "open-browser",
      title: "Opened Stripe Checkout in your browser",
      status: "done",
      detail: selectedPlan.title,
      action: (
        <Button onClick={() => onReopenCheckout?.()} size="xs" type="button" variant="ghost">
          <ExternalLink aria-hidden="true" />
          Reopen
        </Button>
      ),
    },
    {
      id: "wait-confirmation",
      title: phase === "waiting_timeout" ? "Still waiting for checkout" : "Waiting for checkout…",
      status: phase === "waiting_timeout" ? "needs_input" : "running",
      caption:
        phase === "waiting_timeout"
          ? "Already finished paying in your browser? Check your plan status now."
          : "Your plan updates here automatically within seconds after payment, even with remote access turned off.",
    },
  ];

  const handleContinue = () => {
    onContinueToCheckout?.(interval);
    if (!isControlledPhase) {
      setUncontrolledPhase("waiting");
    }
  };

  const handleCheckAgain = () => {
    onCheckAgain?.();
    if (!isControlledPhase) {
      onCompleteUpgrade?.();
      setUncontrolledPhase("done");
    }
  };

  const handleFinish = () => {
    onCompleteUpgrade?.();
    onClose();
  };

  const handleCancelWaiting = () => {
    if (onCancelWaiting) {
      onCancelWaiting();
      return;
    }
    onClose();
  };

  return (
    <Dialog
      onOpenChange={next => {
        if (!next) onClose();
      }}
      open={open}
    >
      <DialogContent
        className={`${dialogShellClass("sm")} z-[60]`}
        data-phase={phase}
        data-testid="account-upgrade-sheet"
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          description={
            phase === "done"
              ? "Your Pro plan is active across your account."
              : "Everything on this computer stays free. Pro adds remote scale and anywhere access."
          }
          icon={phase === "done" ? CheckCircle2 : Sparkles}
          onClose={onClose}
          title={phase === "done" ? "You're on Pro" : "Upgrade to Compozy Pro"}
          tone={phase === "done" ? "neutral" : "brand"}
        />

        <EntityDialogBody className="flex flex-col gap-5">
          {phase === "select" ? (
            <>
              {checkoutCancelled ? (
                <Alert
                  className="py-2.5"
                  data-testid="upgrade-sheet-cancelled"
                  role="status"
                  variant="info"
                >
                  <Info aria-hidden="true" />
                  <AlertDescription>
                    Checkout was cancelled. Nothing was charged.
                  </AlertDescription>
                </Alert>
              ) : null}

              {checkoutError ? (
                <Alert
                  className="py-2.5"
                  data-testid="upgrade-sheet-error"
                  role="alert"
                  variant="destructive"
                >
                  <TriangleAlert aria-hidden="true" />
                  <AlertDescription>{checkoutError}</AlertDescription>
                </Alert>
              ) : null}

              <div className="flex flex-col gap-2">
                <RadioCardGroup
                  aria-label="Choose a billing period"
                  className="grid-cols-1 gap-2.5 sm:grid-cols-2"
                  onValueChange={handleIntervalChange}
                  options={planOptions}
                  value={interval}
                />
                <p className="text-meta text-muted-foreground">
                  Prices in USD. Plus applicable taxes, calculated at checkout.
                </p>
              </div>

              <div className="flex flex-col gap-2.5 border-t border-line-soft pt-4">
                <p className="eyebrow text-subtle">What Pro adds</p>
                <ul className="flex flex-col gap-2.5">
                  {ACCOUNT_PRO_BENEFITS.map(benefit => {
                    const Icon =
                      BENEFIT_ICONS[benefit.id as keyof typeof BENEFIT_ICONS] ?? Sparkles;
                    return (
                      <li className="flex items-start gap-2.5" key={benefit.id}>
                        <span
                          aria-hidden="true"
                          className="mt-0.5 inline-flex size-5 shrink-0 items-center justify-center rounded-sm bg-surface-2 text-fg"
                        >
                          <Icon className="size-3" />
                        </span>
                        <div className="min-w-0">
                          <p className="text-small-body font-medium text-fg">{benefit.label}</p>
                          <p className="text-meta text-muted-foreground">{benefit.detail}</p>
                        </div>
                      </li>
                    );
                  })}
                </ul>
              </div>
            </>
          ) : null}

          {phase === "waiting" || phase === "waiting_timeout" ? (
            <div className="flex flex-col gap-4">
              <StepProgressList steps={waitingSteps} />

              <div
                aria-live="polite"
                className="flex flex-wrap items-center justify-between gap-2 border-t border-line-soft pt-3.5 text-meta text-muted-foreground"
              >
                <span className="inline-flex items-center gap-2">
                  {phase === "waiting" ? (
                    <>
                      <Spinner aria-hidden="true" className="size-3" />
                      <span>Listening for account update…</span>
                    </>
                  ) : (
                    <span>Still waiting? Check again</span>
                  )}
                </span>
                <Button
                  disabled={isCheckingStatus}
                  onClick={handleCheckAgain}
                  size="xs"
                  type="button"
                  variant={phase === "waiting_timeout" ? "secondary" : "ghost"}
                >
                  {isCheckingStatus ? (
                    <Spinner aria-hidden="true" className="size-3" />
                  ) : (
                    <RefreshCw aria-hidden="true" />
                  )}
                  Check again
                </Button>
              </div>
            </div>
          ) : null}

          {phase === "done" ? (
            <div className="flex flex-col gap-4">
              <ActionResultBanner
                description="Unlimited computers, hosted routes, phone access, and hosted webhooks are active now. No restart needed."
                icon={CheckCircle2}
                title="You're on Pro"
                tone="success"
              />
              <div className="flex flex-col divide-y divide-line-soft border-t border-line-soft">
                <div className="flex items-center justify-between gap-3 py-2.5 text-small-body">
                  <span className="text-muted-foreground">Remote computers</span>
                  <span className="font-medium text-fg">Unlimited</span>
                </div>
                <div className="flex items-center justify-between gap-3 py-2.5 text-small-body">
                  <span className="text-muted-foreground">Hosted routes</span>
                  <span className="font-medium text-fg tabular-nums">Up to 10 computers</span>
                </div>
                <div className="flex items-center justify-between gap-3 py-2.5 text-small-body">
                  <span className="text-muted-foreground">Plan</span>
                  <span className="font-medium text-fg">{selectedPlan.title}</span>
                </div>
              </div>
            </div>
          ) : null}
        </EntityDialogBody>

        {phase === "select" ? (
          <EntityDialogFooter
            cancelLabel="Cancel"
            hint="Checkout opens in your browser."
            isSaving={isStartingCheckout}
            onCancel={onClose}
            onPrimary={handleContinue}
            primaryDisabled={isStartingCheckout}
            primaryIcon={ExternalLink}
            primaryLabel="Continue to checkout"
          />
        ) : null}

        {phase === "waiting" || phase === "waiting_timeout" ? (
          <EntityDialogFooter
            cancelLabel="Cancel"
            hint="Closing this window will not cancel a completed payment."
            isSaving={phase === "waiting_timeout" && isCheckingStatus}
            onCancel={handleCancelWaiting}
            onPrimary={phase === "waiting_timeout" ? handleCheckAgain : () => onReopenCheckout?.()}
            primaryDisabled={phase === "waiting_timeout" && isCheckingStatus}
            primaryIcon={phase === "waiting_timeout" ? RefreshCw : ExternalLink}
            primaryLabel={phase === "waiting_timeout" ? "Check again" : "Reopen checkout"}
          />
        ) : null}

        {phase === "done" ? (
          <DialogFooter className="min-h-editor-footer items-center" variant="ruled">
            <Button onClick={handleFinish} type="button">
              Done
            </Button>
          </DialogFooter>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}
