// Suite: onboarding setup panel chrome
// Invariant: the panel blocks until setup is answered — step strip, footer summary and
// dismissal behaviour all follow the wizard model and never let the operator out early.
// Boundary IN: OnboardingSetupFrame and its step strip/footer composition.
// Boundary OUT: wizard orchestration, provider/workspace transports, the desktop shell.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { onboardingWizardFixture } from "../../mocks/setup-fixtures";
import { OnboardingSetupFrame } from "../onboarding-setup-frame";

describe("OnboardingSetupFrame", () => {
  it("Should mark the current step and lock every step the flow has not reached", () => {
    render(<OnboardingSetupFrame wizard={onboardingWizardFixture()} />);

    const first = screen.getByTestId("onboarding-step-1");
    const second = screen.getByTestId("onboarding-step-2");
    expect(first).toHaveAttribute("data-state", "current");
    expect(first).toHaveAttribute("aria-current", "step");
    expect(second).toHaveAttribute("data-state", "upcoming");
    expect(second).toBeDisabled();
  });

  it("Should mark a passed step done and let the operator step back to it", async () => {
    const user = userEvent.setup();
    const goToStep = vi.fn();
    render(
      <OnboardingSetupFrame
        wizard={onboardingWizardFixture({ wizard: { step: 2, maxStep: 2, goToStep } })}
      />
    );

    const first = screen.getByTestId("onboarding-step-1");
    expect(first).toHaveAttribute("data-state", "done");
    expect(first).toBeEnabled();
    await user.click(first);

    expect(goToStep).toHaveBeenCalledWith(1);
  });

  it("Should summarise what the runtime step will save", () => {
    render(<OnboardingSetupFrame wizard={onboardingWizardFixture()} />);

    expect(screen.getByTestId("onboarding-summary-value")).toHaveTextContent(
      "Claude Code · Claude Opus 4.8 · High · CLI sign-in"
    );
    expect(screen.getByTestId("onboarding-continue")).toHaveTextContent("Continue");
  });

  it("Should replace the summary with the configuration error and block Continue", () => {
    render(
      <OnboardingSetupFrame
        wizard={onboardingWizardFixture({
          defaultModel: {
            authMode: "bound_secret",
            missingEnvVar: true,
            configurationError: "Enter the environment variable the provider expects.",
            isValid: false,
          },
        })}
      />
    );

    expect(screen.getByTestId("onboarding-commit-error")).toHaveTextContent(
      "Enter the environment variable the provider expects."
    );
    expect(screen.getByTestId("onboarding-env-var")).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByTestId("onboarding-continue")).toBeDisabled();
  });

  it("Should let a commit error win over the configuration summary", () => {
    render(
      <OnboardingSetupFrame
        wizard={onboardingWizardFixture({ wizard: { commitError: "Provider write failed." } })}
      />
    );

    expect(screen.getByTestId("onboarding-commit-error")).toHaveTextContent(
      "Provider write failed."
    );
  });

  it("Should lock every setup action while Finish setup is busy", async () => {
    const user = userEvent.setup();
    const next = vi.fn();
    const goToStep = vi.fn();
    render(
      <OnboardingSetupFrame
        wizard={onboardingWizardFixture({
          wizard: { step: 2, maxStep: 2, isBusy: true, next, goToStep },
        })}
      />
    );

    const finish = screen.getByTestId("onboarding-continue");
    expect(finish).toHaveTextContent("Finish setup");
    expect(finish).toBeDisabled();
    expect(finish.querySelector('[role="status"][aria-label="Loading"]')).not.toBeNull();

    const previousStep = screen.getByTestId("onboarding-step-1");
    expect(previousStep).toBeDisabled();
    await user.click(previousStep);
    expect(goToStep).not.toHaveBeenCalled();
  });

  it("Should not close on Escape or an outside press", async () => {
    const user = userEvent.setup();
    render(<OnboardingSetupFrame wizard={onboardingWizardFixture()} />);

    await user.keyboard("{Escape}");
    expect(screen.getByTestId("onboarding-setup-panel")).toBeInTheDocument();

    await user.click(document.body);
    expect(screen.getByTestId("onboarding-setup-panel")).toBeInTheDocument();
  });

  it("Should render no window controls — the panel cannot be closed, minimised or zoomed", () => {
    render(<OnboardingSetupFrame wizard={onboardingWizardFixture()} />);

    expect(screen.queryByRole("button", { name: /close/i })).toBeNull();
    expect(screen.queryByTestId("os-traffic-lights")).toBeNull();
  });
});
