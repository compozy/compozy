import { Button, Eyebrow, Spinner, cn } from "@agh/ui";

import { ONBOARDING_SUMMARY_ID, type OnboardingSummary } from "../lib/onboarding-summary";

/** Action posture of the footer for the current step. */
export interface OnboardingSetupActions {
  /** Primary action label — `Continue` until the last step, then `Finish setup`. */
  label: string;
  canContinue: boolean;
  canGoBack: boolean;
  busy: boolean;
}

export interface OnboardingSetupFooterProps {
  summary: OnboardingSummary;
  actions: OnboardingSetupActions;
  onBack: () => void;
  onContinue: () => void;
}

/**
 * The action footer states what pressing Continue will save. When a requirement
 * is unmet the same line carries the reason in danger tone — one place to look,
 * never a summary and an error competing for the operator's attention.
 */
export function OnboardingSetupFooter({
  summary,
  actions,
  onBack,
  onContinue,
}: OnboardingSetupFooterProps) {
  const danger = summary.tone === "danger";

  return (
    <footer
      data-slot="onboarding-setup-footer"
      className="flex h-setup-footer flex-none items-center gap-4 border-t border-line bg-canvas-soft px-4 max-md:h-auto max-md:flex-wrap max-md:py-3"
    >
      <div
        className="flex min-w-0 flex-1 flex-col gap-0.5"
        data-tone={summary.tone}
        {...(danger ? { role: "alert", "data-testid": "onboarding-commit-error" } : {})}
      >
        <Eyebrow className={danger ? "text-danger" : "text-faint"}>{summary.label}</Eyebrow>
        <span
          id={ONBOARDING_SUMMARY_ID}
          data-testid="onboarding-summary-value"
          className={cn("truncate text-form-hint", danger ? "text-danger" : "font-mono text-muted")}
        >
          {summary.value}
        </span>
      </div>
      <div className="flex flex-none items-center gap-2">
        <Button
          variant="ghost"
          size="lg"
          onClick={onBack}
          disabled={!actions.canGoBack || actions.busy}
        >
          Back
        </Button>
        <Button
          size="lg"
          onClick={onContinue}
          disabled={!actions.canContinue || actions.busy}
          data-testid="onboarding-continue"
        >
          {actions.busy ? <Spinner /> : null}
          {actions.label}
        </Button>
      </div>
    </footer>
  );
}
