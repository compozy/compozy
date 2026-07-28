import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, within } from "storybook/test";

import { onboardingWizardFixture } from "../../mocks/setup-fixtures";
import { OnboardingSetupFrame } from "../onboarding-setup-frame";

const meta: Meta = {
  title: "systems/onboarding/components/Steps",
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const DefaultModelNativeCli: Story = {
  render: () => <OnboardingSetupFrame wizard={onboardingWizardFixture()} />,
};

export const DefaultModelApiKey: Story = {
  render: () => (
    <OnboardingSetupFrame
      wizard={onboardingWizardFixture({
        defaultModel: {
          harness: "pi_acp",
          providerName: "OpenRouter",
          authMode: "bound_secret",
          envVar: "OPENROUTER_API_KEY",
        },
      })}
    />
  ),
};

export const DefaultModelApiKeyMissingEnv: Story = {
  render: () => (
    <OnboardingSetupFrame
      wizard={onboardingWizardFixture({
        defaultModel: {
          harness: "pi_acp",
          providerName: "OpenRouter",
          authMode: "bound_secret",
          envVar: "",
          missingEnvVar: true,
          configurationError: "Enter the environment variable the provider expects.",
          isValid: false,
        },
      })}
    />
  ),
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await expect(body.getByTestId("onboarding-commit-error")).toHaveTextContent(
      "Enter the environment variable the provider expects."
    );
    await expect(body.getByTestId("onboarding-continue")).toBeDisabled();
  },
};

export const Workspaces: Story = {
  render: () => (
    <OnboardingSetupFrame wizard={onboardingWizardFixture({ wizard: { step: 2, maxStep: 2 } })} />
  ),
};

export const WorkspacesEmpty: Story = {
  render: () => (
    <OnboardingSetupFrame
      wizard={onboardingWizardFixture({
        wizard: { step: 2, maxStep: 2 },
        workspaces: { workspaces: [] },
      })}
    />
  ),
};

export const WorkspacesCatalogError: Story = {
  render: () => (
    <OnboardingSetupFrame
      wizard={onboardingWizardFixture({
        wizard: { step: 2, maxStep: 2 },
        workspaces: {
          catalogError: "Workspace catalog is unavailable. Check the daemon connection.",
        },
      })}
    />
  ),
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await expect(body.getByRole("alert")).toHaveTextContent("Workspace catalog is unavailable");
    await expect(body.getByRole("button", { name: "Retry" })).toBeEnabled();
    await expect(body.getByRole("button", { name: "Remove compozy" })).toBeDisabled();
  },
};
