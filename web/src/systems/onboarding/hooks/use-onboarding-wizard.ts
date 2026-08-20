import { useState } from "react";
import { toast } from "sonner";
import { useSelector } from "@xstate/store-react";

import { onboardingDraftStore } from "../stores/use-onboarding-draft-store";
import { useCompleteOnboarding } from "./use-complete-onboarding";
import {
  useOnboardingDefaultModel,
  type OnboardingDefaultModelApi,
} from "./use-onboarding-default-model";
import { useOnboardingWorkspaces, type OnboardingWorkspacesApi } from "./use-onboarding-workspaces";

export const ONBOARDING_STEP_COUNT = 2;

/**
 * The step strip carries progress and the footer carries what will be saved, so
 * a pane only needs a heading that stands alone. Skip, default, and Network
 * consequences the heading does not name live in `help`.
 */
export interface OnboardingStepMeta {
  title: string;
  help: string;
  helpLabel: string;
}

const STEP_META: Record<number, OnboardingStepMeta> = {
  1: {
    title: "Choose the model your agents run on",
    help: "New agents inherit this model. Change it any time in Settings.",
    helpLabel: "About the default model",
  },
  2: {
    title: "Pick where agents can work",
    help: "Skip starts in Global (~, your home folder). Setup does not enable Network.",
    helpLabel: "About workspace",
  },
};

/** Shared with story/test fixtures so a fixture can never drift from the copy. */
export function onboardingStepMeta(step: number): OnboardingStepMeta {
  return STEP_META[step] ?? STEP_META[1];
}

export interface OnboardingWizardApi {
  step: number;
  maxStep: number;
  meta: OnboardingStepMeta;
  defaultModel: OnboardingDefaultModelApi;
  workspaces: OnboardingWorkspacesApi;
  canContinue: boolean;
  isLastStep: boolean;
  isBusy: boolean;
  commitError: string | null;
  goToStep: (step: number) => void;
  back: () => void;
  next: () => Promise<void>;
}

export function useOnboardingWizard(onComplete: () => void): OnboardingWizardApi {
  const step = useSelector(onboardingDraftStore, state => state.context.step);
  const maxStep = useSelector(onboardingDraftStore, state => state.context.maxStep);

  const defaultModel = useOnboardingDefaultModel();
  const workspaces = useOnboardingWorkspaces();
  const complete = useCompleteOnboarding();
  const [commitError, setCommitError] = useState<string | null>(null);
  const workspaceBusy = workspaces.isResolving || workspaces.isRemoving;

  const canContinue = step === 1 ? defaultModel.isValid : !workspaceBusy;

  const goToStep = (next: number) => {
    if (next < 1 || next > ONBOARDING_STEP_COUNT || next > maxStep) return;
    onboardingDraftStore.trigger.stepVisited({ step: next });
  };

  const back = () => {
    if (step > 1) {
      onboardingDraftStore.trigger.stepVisited({ step: step - 1 });
    }
  };

  const finish = async () => {
    setCommitError(null);
    try {
      await complete.mutateAsync();
      onboardingDraftStore.trigger.draftCleared();
      onComplete();
    } catch (error) {
      const message = error instanceof Error ? error.message : "Failed to finish onboarding.";
      setCommitError(message);
      toast.error(message);
    }
  };

  const next = async () => {
    setCommitError(null);
    if (workspaceBusy) return;
    if (step === 1) {
      try {
        await defaultModel.commit();
      } catch (error) {
        const message =
          error instanceof Error ? error.message : "Failed to save your default model.";
        setCommitError(message);
        toast.error(message);
        return;
      }
      onboardingDraftStore.trigger.stepVisited({ step: 2 });
      return;
    }
    if (step === 2) {
      await finish();
      return;
    }
  };

  return {
    step,
    maxStep,
    meta: onboardingStepMeta(step),
    defaultModel,
    workspaces,
    canContinue,
    isLastStep: step === ONBOARDING_STEP_COUNT,
    isBusy: defaultModel.isCommitting || workspaceBusy || complete.isPending,
    commitError,
    goToStep,
    back,
    next,
  };
}
