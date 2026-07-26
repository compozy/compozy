import { Lock } from "lucide-react";

import { Dialog, DialogContent, DialogTitle, Icon, Logo, Pill, cn } from "@agh/ui";

import type { OnboardingWizardApi } from "../hooks/use-onboarding-wizard";
import { useSetupBodyHeight } from "../hooks/use-setup-body-height";
import { onboardingSummary } from "../lib/onboarding-summary";
import { OnboardingSetupFooter } from "./onboarding-setup-footer";
import { OnboardingStepStrip } from "./onboarding-step-strip";
import { StepDefaultModel } from "./step-default-model";
import { StepWorkspaces } from "./step-workspaces";

export interface OnboardingSetupFrameProps {
  wizard: OnboardingWizardApi;
}

/**
 * First-run setup wears the window grammar without being a window: identity in a
 * 52px head, progress in the strip a window gives its context tools, an animated
 * body that takes each step's shape, and a 58px action footer. It cannot be
 * closed, minimised or zoomed, so it renders no control it would have to disable
 * — there are no traffic lights.
 *
 * Presentational: every value comes from `wizard`, so the frame is storyable and
 * testable without a daemon.
 */
export function OnboardingSetupFrame({ wizard }: OnboardingSetupFrameProps) {
  const body = useSetupBodyHeight();
  const model = wizard.defaultModel;
  const summary = onboardingSummary({
    step: wizard.step,
    error: wizard.commitError ?? model.configurationError,
    runtime: {
      providerName: model.providerName,
      modelName: model.modelName,
      reasoningLabel: model.reasoningLabel,
      authMode: model.authMode,
      envVar: model.envVar,
    },
    workspaces: wizard.workspaces.workspaces,
  });

  return (
    <Dialog open disablePointerDismissal onOpenChange={() => {}}>
      <DialogContent
        unframed
        showCloseButton={false}
        // Blocking by construction: Esc and outside presses are inert, so assistive
        // tech gets the explicit-choice contract the interaction already implements.
        role="alertdialog"
        data-testid="onboarding-setup-panel"
        data-step={wizard.step}
        className={cn(
          "flex max-h-[calc(100dvh-3rem)] flex-col border border-line-strong bg-canvas",
          // Beats DIALOG_CONTENT_BASE's `sm:max-w-sm` without dropping the viewport
          // guard: the 960px step-2 panel must still fit a 768px window.
          "rounded-xl shadow-window sm:max-w-[calc(100%-3rem)]",
          "transition-[width] duration-shell-slow ease-spring motion-reduce:transition-none",
          wizard.step === 2 ? "w-setup-panel-wide" : "w-setup-panel",
          // Below md the panel is the whole viewport: a sheet, not a dialog.
          "max-md:top-0 max-md:left-0 max-md:h-dvh max-md:max-h-none max-md:w-full",
          "max-md:translate-x-0 max-md:translate-y-0 max-md:rounded-none max-md:border-0"
        )}
      >
        <header className="flex h-setup-head flex-none items-center gap-2.5 border-b border-line pr-3.5 pl-4">
          <Logo variant="symbol" decorative className="size-5" />
          <DialogTitle className="text-modal-title font-semibold tracking-modal-title text-fg-strong">
            Set up AGH
          </DialogTitle>
          <span aria-hidden="true" className="min-w-2 flex-1" />
          <Pill
            size="md"
            tone="neutral"
            className="flex-none gap-1.5 border border-line bg-transparent text-subtle"
            title="The daemon runs on this machine — no account, no upload."
          >
            <Icon as={Lock} size="sm" aria-hidden="true" />
            Runs locally
          </Pill>
        </header>

        <OnboardingStepStrip
          step={wizard.step}
          maxStep={wizard.maxStep}
          busy={wizard.isBusy}
          onSelect={wizard.goToStep}
        />

        <div
          data-slot="onboarding-setup-body"
          className={cn(
            "relative min-h-0 flex-none overflow-y-auto",
            "transition-[height] duration-shell-slow ease-spring motion-reduce:transition-none",
            "max-md:h-auto max-md:flex-1"
          )}
          style={body.height === null ? undefined : { height: `${body.height}px` }}
        >
          <div key={wizard.step} className="onboarding-setup-pane-in">
            <div ref={body.measureRef} className="flex flex-col px-6 pt-5.5 pb-6.5 max-md:px-4">
              <h3 className="text-compact-h1 font-semibold tracking-compact-h1 text-fg-strong">
                {wizard.meta.title}
              </h3>
              <p className="mt-1.75 max-w-[64ch] text-small-body leading-5 text-muted">
                {wizard.meta.lead}
              </p>
              {wizard.step === 1 ? (
                <StepDefaultModel model={wizard.defaultModel} />
              ) : (
                <StepWorkspaces workspaces={wizard.workspaces} />
              )}
            </div>
          </div>
        </div>

        <OnboardingSetupFooter
          summary={summary}
          actions={{
            label: wizard.isLastStep ? "Finish setup" : "Continue",
            canContinue: wizard.canContinue,
            canGoBack: wizard.step > 1,
            busy: wizard.isBusy,
          }}
          onBack={wizard.back}
          onContinue={() => void wizard.next()}
        />
      </DialogContent>
    </Dialog>
  );
}
