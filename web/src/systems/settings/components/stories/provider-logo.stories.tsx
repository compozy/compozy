import type { Meta, StoryObj } from "@storybook/react-vite";

import { Eyebrow } from "@agh/ui";

import { CenteredSurface } from "@/storybook/story-layout";

import { ProviderLogo } from "../provider-logo";

const meta: Meta<typeof ProviderLogo> = {
  title: "systems/settings/components/ProviderLogo",
  component: ProviderLogo,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Neutral grayscale provider mark sourced from `@agh/ui`'s `KindIcon` provider registry. Used inside the provider card icon-well — never rendered with a brand-color tint.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const PROVIDERS = ["claude", "codex", "gemini", "qwen-code", "openclaw", "hermes", "unknown"];

/**
 * Default — single Claude logo at 20px (the size used inside the provider-card icon-well).
 */
export const Default: Story = {
  args: { provider: "claude", className: "size-5" },
  render: args => (
    <CenteredSurface>
      <span className="inline-flex size-10 items-center justify-center rounded-icon-well bg-elevated ring-1 ring-line">
        <ProviderLogo {...args} />
      </span>
    </CenteredSurface>
  ),
};

/**
 * Registry — every entry the kit registry knows about, including the unknown
 * fallback. Verifies the warm-grayscale tone holds across providers.
 */
export const Registry: Story = {
  args: {},
  render: () => (
    <CenteredSurface>
      <div className="grid w-full max-w-3xl grid-cols-3 gap-3 sm:grid-cols-4 lg:grid-cols-7">
        {PROVIDERS.map(provider => (
          <div
            key={provider}
            className="flex flex-col items-center gap-2 rounded-md border border-line bg-canvas-soft p-3"
          >
            <span className="inline-flex size-10 items-center justify-center rounded-icon-well bg-elevated ring-1 ring-line">
              <ProviderLogo provider={provider} className="size-5" />
            </span>
            <Eyebrow className="text-muted">{provider}</Eyebrow>
          </div>
        ))}
      </div>
    </CenteredSurface>
  ),
};
