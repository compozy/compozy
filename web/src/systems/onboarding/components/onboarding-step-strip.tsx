import { Check } from "lucide-react";

import { cn } from "@agh/ui";

const STEPS: { step: number; label: string }[] = [
  { step: 1, label: "Runtime" },
  { step: 2, label: "Workspace" },
];

export interface OnboardingStepStripProps {
  step: number;
  maxStep: number;
  busy: boolean;
  onSelect: (step: number) => void;
}

type StepState = "done" | "current" | "upcoming";

function stateFor(entry: number, step: number): StepState {
  if (entry < step) return "done";
  return entry === step ? "current" : "upcoming";
}

/**
 * Progress lives in the slot an OS window gives its context strip: two segments,
 * each carrying a 2px rule that fills with accent as the step is reached. A
 * segment beyond `maxStep` is disabled — you cannot skip a question.
 */
export function OnboardingStepStrip({ step, maxStep, busy, onSelect }: OnboardingStepStripProps) {
  return (
    <nav
      aria-label="Setup progress"
      data-slot="onboarding-step-strip"
      className="grid h-setup-steps flex-none grid-cols-2 border-b border-line"
    >
      {STEPS.map(entry => {
        const state = stateFor(entry.step, step);
        const reached = state !== "upcoming";
        return (
          <button
            key={entry.step}
            type="button"
            data-testid={`onboarding-step-${entry.step}`}
            data-state={state}
            aria-current={state === "current" ? "step" : undefined}
            disabled={busy || entry.step > maxStep}
            onClick={() => onSelect(entry.step)}
            className={cn(
              "relative flex items-center gap-2.5 px-4 text-left text-small-body font-medium",
              "transition-colors duration-base focus-visible:shadow-focus-inset focus-visible:outline-none",
              "after:absolute after:inset-x-0 after:-bottom-px after:h-0.5 after:transition-colors after:duration-shell-slow",
              reached ? "after:bg-accent" : "after:bg-line",
              state === "current" && "text-fg-strong",
              state === "done" && "text-muted",
              state === "upcoming" && "text-subtle",
              "hover:text-fg disabled:pointer-events-none disabled:opacity-70"
            )}
          >
            <span
              aria-hidden="true"
              className={cn(
                "grid size-setup-step-marker flex-none place-items-center rounded-full border",
                "font-mono text-pill-group-badge font-semibold",
                "transition-colors duration-base",
                state === "done"
                  ? "border-transparent bg-accent text-accent-ink"
                  : state === "current"
                    ? "border-accent text-accent-strong"
                    : "border-line-strong text-subtle"
              )}
            >
              {state === "done" ? <Check className="size-2.5" strokeWidth={2.5} /> : entry.step}
            </span>
            {entry.label}
          </button>
        );
      })}
    </nav>
  );
}
