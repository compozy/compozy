import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, within } from "storybook/test";

import { Pill, type PillSize, type PillTone } from "../pill";

const meta: Meta<typeof Pill> = {
  title: "components/custom/Pill",
  component: Pill,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Unified semantic pill — flat 4px radius across every size, sentence-case by default (no `uppercase` prop). Mono variants render at 10.5px / 600. Compose with `Pill.Dot` for leading status dots.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const TONES: PillTone[] = ["neutral", "accent", "success", "warning", "danger", "info"];
const SIZES: PillSize[] = ["xs", "sm", "md"];

const KIND_DOT_COLORS: Record<string, string> = {
  say: "var(--color-kind-say)",
  greet: "var(--color-kind-greet)",
  direct: "var(--color-kind-direct)",
  receipt: "var(--color-kind-receipt)",
  capability: "var(--color-kind-capability)",
  trace: "var(--color-kind-trace)",
  whois: "var(--color-kind-whois)",
};

export const Default: Story = {
  args: { children: "label" },
};

export const Tones: Story = {
  args: {},
  render: () => (
    <div className="flex flex-wrap items-center gap-2">
      {TONES.map(tone => (
        <Pill key={tone} tone={tone} mono>
          {tone}
        </Pill>
      ))}
    </div>
  ),
};

export const TonesSans: Story = {
  args: {},
  render: () => (
    <div className="flex flex-wrap items-center gap-2">
      {TONES.map(tone => (
        <Pill key={tone} tone={tone}>
          {tone}
        </Pill>
      ))}
    </div>
  ),
};

export const SolidEmphasis: Story = {
  args: {},
  render: () => (
    <div className="flex flex-wrap items-center gap-2">
      {TONES.map(tone => (
        <Pill key={tone} tone={tone} mono solid>
          {tone}
        </Pill>
      ))}
    </div>
  ),
  parameters: {
    docs: {
      description: {
        story: "`solid` swaps the 15% tinted bg for a fully filled accent + ink-text formula.",
      },
    },
  },
};

export const Sizes: Story = {
  args: {},
  render: () => (
    <div className="flex flex-wrap items-center gap-3">
      {SIZES.map(size => (
        <Pill key={size} mono size={size} tone="neutral">
          {`size=${size}`}
        </Pill>
      ))}
    </div>
  ),
  parameters: {
    docs: {
      description: {
        story:
          "All sizes share the flat `rounded-xs` (4 px) chip radius Heights: xs = 17 px, sm = 19 px, md = 22 px.",
      },
    },
  },
};

export const TonesBySizeMatrix: Story = {
  args: {},
  render: () => (
    <div className="flex flex-col gap-3">
      {SIZES.map(size => (
        <div key={size} className="flex flex-wrap items-center gap-2">
          {TONES.map(tone => (
            <Pill key={`${size}-${tone}`} tone={tone} size={size} mono>
              {`${tone}/${size}`}
            </Pill>
          ))}
        </div>
      ))}
    </div>
  ),
  parameters: {
    docs: {
      description: {
        story:
          "Every tone × every size — used to lock visual baselines for the design-system primitives.",
      },
    },
  },
};

export const MonoIdentifier: Story = {
  args: {},
  render: () => <Pill mono>agh-network/v0</Pill>,
  parameters: {
    docs: {
      description: {
        story:
          "Mono pills render their content at the casing the caller passes — no `uppercase` prop.",
      },
    },
  },
};

export const WithDot: Story = {
  args: {},
  render: () => (
    <div className="flex flex-wrap items-center gap-2">
      <Pill mono tone="success">
        <Pill.Dot />
        Connected
      </Pill>
      <Pill mono tone="warning">
        <Pill.Dot pulse />
        Reconnecting
      </Pill>
      <Pill mono tone="danger">
        <Pill.Dot />
        Disconnected
      </Pill>
    </div>
  ),
};

export const KindChipReplacement: Story = {
  args: {},
  render: () => (
    <div className="flex flex-wrap items-center gap-2">
      {Object.keys(KIND_DOT_COLORS).map(kind => (
        <Pill key={kind} mono size="sm" tone="neutral">
          <Pill.Dot color={KIND_DOT_COLORS[kind]} size="sm" />
          {kind}
        </Pill>
      ))}
    </div>
  ),
  parameters: {
    docs: {
      description: {
        story: "Protocol kind markers, leading dot keyed off the kind, label preserved.",
      },
    },
  },
};

export const ToggleInteractive: Story = {
  args: {},
  render: () => (
    <div className="flex flex-wrap items-center gap-2">
      <Pill mono active render={<button type="button" />}>
        all
      </Pill>
      <Pill mono active={false} render={<button type="button" />}>
        say
      </Pill>
      <Pill mono active={false} render={<button type="button" />}>
        direct
      </Pill>
    </div>
  ),
  parameters: {
    docs: {
      description: {
        story:
          "Pass `render={<button />}` and `active` to render a stand-alone toggle chip (replaces `WireChip`).",
      },
    },
  },
};

export const LinkChip: Story = {
  args: {},
  render: () => <Pill.Link href="/tasks/task-102">Open task</Pill.Link>,
  parameters: {
    docs: {
      description: {
        story: "`Pill.Link` renders the same semantic pill chrome as an accessible anchor.",
      },
    },
  },
};

export const ConnectionIndicator: Story = {
  args: {},
  render: () => (
    <div className="flex flex-col gap-2">
      <div role="status" aria-live="polite" className="inline-flex items-center gap-2">
        <Pill.Dot tone="success" />
        <span className="eyebrow text-subtle">Connected</span>
      </div>
      <div role="status" aria-live="polite" className="inline-flex items-center gap-2">
        <Pill.Dot tone="warning" pulse />
        <span className="eyebrow text-subtle">Reconnecting</span>
      </div>
      <div role="status" aria-live="polite" className="inline-flex items-center gap-2">
        <Pill.Dot tone="danger" />
        <span className="eyebrow text-subtle">Disconnected</span>
      </div>
    </div>
  ),
  parameters: {
    docs: {
      description: {
        story:
          "Replacement composition for the legacy `ConnectionIndicator`, `Pill.Dot` + eyebrow label inside an `aria-live=polite` region.",
      },
    },
  },
};

export const StandaloneDots: Story = {
  args: {},
  render: () => (
    <div className="flex items-center gap-6" data-testid="pill-dots">
      {TONES.map(tone => (
        <div key={tone} className="flex items-center gap-2" data-testid={`pill-dot-${tone}`}>
          <Pill.Dot tone={tone} />
          <span className="eyebrow text-subtle">{tone}</span>
        </div>
      ))}
    </div>
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    for (const tone of TONES) {
      const wrapper = await canvas.findByTestId(`pill-dot-${tone}`);
      const dot = wrapper.querySelector('[data-slot="pill-dot"]') as HTMLElement;
      await expect(dot).toBeInTheDocument();
      await expect(dot.getAttribute("data-tone")).toBe(tone);
    }
  },
};

export const DotSizes: Story = {
  args: {},
  render: () => (
    <div className="flex items-center gap-6">
      <div className="flex items-center gap-2">
        <Pill.Dot size="sm" tone="success" />
        <span className="eyebrow text-subtle">sm · 6px</span>
      </div>
      <div className="flex items-center gap-2">
        <Pill.Dot size="md" tone="success" />
        <span className="eyebrow text-subtle">md · 8px</span>
      </div>
    </div>
  ),
};

export const PulseAnimation: Story = {
  args: { tone: "accent", mono: true, children: "running" },
  render: args => (
    <Pill {...args}>
      <Pill.Dot pulse />
      {args.children}
    </Pill>
  ),
};
