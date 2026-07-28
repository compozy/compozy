import { useOnboardingWizard } from "../hooks/use-onboarding-wizard";
import { OnboardingSetupFrame } from "./onboarding-setup-frame";

export interface OnboardingSetupPanelProps {
  onComplete: () => void;
}

/**
 * Live first-run setup: the wizard model wired into the blocking panel that
 * renders over the desktop shell. `onComplete` fires once the daemon has
 * recorded completion, so the gate can re-read status and wake the chrome.
 */
export function OnboardingSetupPanel({ onComplete }: OnboardingSetupPanelProps) {
  const wizard = useOnboardingWizard(onComplete);
  return <OnboardingSetupFrame wizard={wizard} />;
}
