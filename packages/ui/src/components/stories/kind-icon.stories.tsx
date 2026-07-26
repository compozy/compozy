import type { Meta, StoryObj } from "@storybook/react-vite";

import { KindIcon } from "../custom/kind-icon";
import { providerKindIconRegistry } from "../custom/kind-icon-registry";

const providerKeys = Object.keys(providerKindIconRegistry);

const meta: Meta<typeof KindIcon> = {
  title: "components/custom/KindIcon",
  component: KindIcon,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Registry-driven icon primitive for provider and runtime kind glyphs. Consumers supply a kind while the kit owns sizing, tone, and fallback behavior.",
      },
    },
  },
  args: {
    kind: "claude",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Default provider icon using the shared provider registry.
 */
export const Default: Story = {
  args: {},
  render: args => <KindIcon {...args} />,
};

/**
 * Every shared provider key rendered through the same primitive.
 */
export const ProviderMatrix: Story = {
  args: {},
  render: () => (
    <div className="grid w-88 grid-cols-3 gap-3">
      {providerKeys.map(provider => (
        <div
          key={provider}
          className="flex min-w-0 items-center gap-2 rounded-md border border-line bg-canvas-soft px-3 py-2"
        >
          <KindIcon kind={provider} tone="default" />
          <span className="eyebrow truncate text-muted">{provider}</span>
        </div>
      ))}
    </div>
  ),
};

/**
 * Size and tone variants keep the same registry while changing visual register.
 */
export const SizesAndTones: Story = {
  args: {},
  render: () => (
    <div className="flex items-center gap-4">
      <KindIcon kind="claude" size="xs" tone="muted" />
      <KindIcon kind="claude" size="sm" tone="default" />
      <KindIcon kind="claude" size="md" tone="accent" />
    </div>
  ),
};
