import type { OnboardingAuthMode } from "../stores/use-onboarding-draft-store";

/** Id of the footer summary line; fields in error describe themselves through it. */
export const ONBOARDING_SUMMARY_ID = "onboarding-setup-summary";

export type OnboardingSummaryTone = "neutral" | "danger";

/** What the footer states will be saved when the operator continues. */
export interface OnboardingSummary {
  label: string;
  value: string;
  tone: OnboardingSummaryTone;
}

export interface OnboardingSummaryInput {
  step: number;
  /** Configuration or commit error; takes over the footer when present. */
  error: string | null;
  runtime: {
    providerName: string;
    modelName: string;
    /** Effort label, or null when the model exposes no selectable levels. */
    reasoningLabel: string | null;
    authMode: OnboardingAuthMode;
    envVar: string;
  };
  workspaces: readonly { name: string }[];
}

function authSummary(authMode: OnboardingAuthMode, envVar: string): string {
  if (authMode === "native_cli") return "CLI sign-in";
  const target = envVar.trim();
  return target.length > 0 ? target : "bound key";
}

function runtimeSummary(runtime: OnboardingSummaryInput["runtime"]): OnboardingSummary {
  const provider = runtime.providerName.trim();
  const model = runtime.modelName.trim();
  if (provider.length === 0 && model.length === 0) {
    return {
      label: "Saves as your default",
      value: "Nothing selected yet — pick a provider and model.",
      tone: "neutral",
    };
  }
  const bits = [provider, model].filter(bit => bit.length > 0);
  if (runtime.reasoningLabel !== null) bits.push(runtime.reasoningLabel);
  bits.push(authSummary(runtime.authMode, runtime.envVar));
  return { label: "Saves as your default", value: bits.join(" · "), tone: "neutral" };
}

function workspacesSummary(workspaces: OnboardingSummaryInput["workspaces"]): OnboardingSummary {
  if (workspaces.length === 0) {
    return {
      label: "Workspaces",
      value: "None yet — add at least one folder to finish.",
      tone: "neutral",
    };
  }
  const count = `${workspaces.length} folder${workspaces.length === 1 ? "" : "s"}`;
  return {
    label: "Workspaces",
    value: `${count} · ${workspaces.map(workspace => workspace.name).join(", ")}`,
    tone: "neutral",
  };
}

/**
 * The footer answers one question — "what happens when I press Continue?" — so
 * an unanswered requirement replaces the summary rather than sitting beside it.
 */
export function onboardingSummary(input: OnboardingSummaryInput): OnboardingSummary {
  if (input.error !== null && input.error.trim().length > 0) {
    return { label: "Needs an answer", value: input.error, tone: "danger" };
  }
  return input.step === 1 ? runtimeSummary(input.runtime) : workspacesSummary(input.workspaces);
}
