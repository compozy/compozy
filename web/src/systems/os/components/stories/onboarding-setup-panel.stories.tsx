import type { Meta, StoryObj } from "@storybook/react-vite";
import type { ReactNode } from "react";

import { OnboardingSetupFrame } from "@/systems/onboarding";
import { onboardingWizardFixture } from "@/systems/onboarding/mocks";

import { buildDeskItems, DesktopShell } from "./_desktop";

/**
 * First run as the OS shows it: the desktop the operator is about to unlock,
 * dimmed and dormant, with the blocking setup panel over it. The story harness
 * renders the same wallpaper/menubar/dock composition as the OpenDesign
 * reference so the visual contract compares like for like.
 */
const meta: Meta = {
  title: "systems/os/components/OnboardingSetupPanel",
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

const DORMANT_DOCK = "translate-y-1.5 opacity-50 saturate-50";

function FirstRunDesktop({ children }: { children: ReactNode }) {
  return (
    <>
      <div inert>
        <DesktopShell
          dockItems={buildDeskItems({ badges: {} })}
          dockClassName={DORMANT_DOCK}
          menubarClassName="opacity-68"
          workspace={{ name: "No workspace", monogram: "··" }}
          notifications={0}
        />
      </div>
      {children}
    </>
  );
}

export const StepRuntime: Story = {
  render: () => (
    <FirstRunDesktop>
      <OnboardingSetupFrame wizard={onboardingWizardFixture()} />
    </FirstRunDesktop>
  ),
};

export const StepWorkspace: Story = {
  render: () => (
    <FirstRunDesktop>
      <OnboardingSetupFrame wizard={onboardingWizardFixture({ wizard: { step: 2, maxStep: 2 } })} />
    </FirstRunDesktop>
  ),
};

export const StepWorkspaceEmpty: Story = {
  render: () => (
    <FirstRunDesktop>
      <OnboardingSetupFrame
        wizard={onboardingWizardFixture({
          wizard: { step: 2, maxStep: 2 },
          workspaces: { workspaces: [], isAdded: () => false },
        })}
      />
    </FirstRunDesktop>
  ),
};

export const Committing: Story = {
  render: () => (
    <FirstRunDesktop>
      <OnboardingSetupFrame
        wizard={onboardingWizardFixture({ wizard: { step: 2, maxStep: 2, isBusy: true } })}
      />
    </FirstRunDesktop>
  ),
};
