import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { CpuIcon, ServerIcon, ZapIcon } from "lucide-react";

import { Pill } from "@agh/ui";
import { RadioCard } from "../radio-card";

const meta: Meta<typeof RadioCard> = {
  title: "components/custom/RadioCard",
  component: RadioCard,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Single radio choice rendered as a card. Resting state is flat on `--canvas-soft`; selected state lifts to `--surface-glaze` with a 1 px inset `--line-strong` ring (no accent).",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-[480px] bg-background p-4">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

interface Provider {
  id: "anthropic" | "openai" | "local";
  title: string;
  description: string;
  icon: typeof CpuIcon;
  badge?: string;
}

const PROVIDERS: ReadonlyArray<Provider> = [
  {
    id: "anthropic",
    title: "Anthropic Claude",
    description: "Bound to ~/.claude credentials.",
    icon: CpuIcon,
    badge: "Default",
  },
  { id: "openai", title: "OpenAI", description: "Bound to OPENAI_API_KEY env.", icon: ZapIcon },
  {
    id: "local",
    title: "Local llama.cpp",
    description: "Connect to a local model server.",
    icon: ServerIcon,
  },
];

/**
 * Three options stacked in a column; selection state animates only the border + tint.
 */
export const ProviderPicker: Story = {
  args: {},
  render: () => {
    const [selected, setSelected] = useState<Provider["id"]>("anthropic");
    return (
      <div role="radiogroup" aria-label="Pick a provider" className="flex flex-col gap-2">
        {PROVIDERS.map(provider => (
          <RadioCard
            key={provider.id}
            title={provider.title}
            description={provider.description}
            icon={provider.icon}
            selected={selected === provider.id}
            onSelect={() => setSelected(provider.id)}
            badge={provider.badge ? <Pill tone="accent">{provider.badge}</Pill> : undefined}
          />
        ))}
      </div>
    );
  },
};

/**
 * Static selected vs unselected for visual diffing.
 */
export const SelectedVsRest: Story = {
  args: {},
  render: () => (
    <div role="radiogroup" aria-label="Sample" className="flex flex-col gap-2">
      <RadioCard
        title="Selected option"
        description="`--surface-glaze` background + 1 px inset `--line-strong` ring."
        icon={CpuIcon}
        selected
        onSelect={() => undefined}
      />
      <RadioCard
        title="Resting option"
        description="Hover lifts the surface to `--elevated`."
        icon={ZapIcon}
        selected={false}
        onSelect={() => undefined}
      />
    </div>
  ),
};

/**
 * Long vault-style refs wrap inside the title slot via `titleClassName`
 * instead of truncating mid-path.
 */
export const WrapFriendlyTitle: Story = {
  args: {},
  render: () => (
    <div role="radiogroup" aria-label="Vault references" className="flex flex-col gap-2">
      <RadioCard
        selected
        onSelect={() => undefined}
        title={
          <code className="block break-all font-mono text-mono-id font-normal leading-snug tracking-normal text-fg">
            vault/mcp/github/personal_access_token_with_a_very_long_operator_facing_ref
          </code>
        }
        titleClassName="overflow-visible whitespace-normal font-normal tracking-normal"
        badge={
          <Pill mono size="xs" tone="success">
            present
          </Pill>
        }
      />
      <RadioCard
        selected={false}
        onSelect={() => undefined}
        title={
          <code className="block break-all font-mono text-mono-id font-normal leading-snug tracking-normal text-fg">
            vault/mcp/short-ref
          </code>
        }
        titleClassName="overflow-visible whitespace-normal font-normal tracking-normal"
        badge={
          <Pill mono size="xs" tone="danger">
            missing
          </Pill>
        }
      />
    </div>
  ),
};
