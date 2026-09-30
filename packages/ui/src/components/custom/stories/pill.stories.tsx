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
          "Unified semantic pill — fully rounded across every size, sans sentence-case label at 12.5px / 510 by default. `mono` is the opt-in variant for raw identifiers and renders at 11px / 600. Compose with `Pill.Dot` for leading status dots.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const TONES: PillTone[] = ["neutral", "accent", "success", "warning", "danger", "info"];
const SIZES: PillSize[] = ["xs", "sm", "md"];

export const Default: Story = {
  args: { children: "label" },
};

export const SansAndMono: Story = {
  parameters: {
    docs: {
      description: {
        story:
          "The default label is sans — status words, kinds, and counts read as language. `mono` stays available for raw identifiers a reader has to match character by character.",
      },
    },
  },
  render: () => (
    <div className="flex flex-wrap items-center gap-2">
      <Pill tone="success">Completed</Pill>
      <Pill tone="info">Running</Pill>
      <Pill mono tone="neutral">
        task-102
      </Pill>
    </div>
  ),
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

/** Outline plate — a transparent rim, not a filled tint. */
export const HollowForm: Story = {
  args: {},
  render: () => (
    <div className="flex flex-wrap items-center gap-2">
      <Pill form="hollow" tone="neutral">
        Hollow
      </Pill>
      <Pill form="hollow" tone="neutral">
        Outline
      </Pill>
      <Pill tone="neutral">Tint</Pill>
    </div>
  ),
  parameters: {
    docs: {
      description: {
        story: "`form=hollow` is a transparent outline plate with an inset rim.",
      },
    },
  },
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
          "All sizes share the fully rounded `rounded-pill` chip radius. Heights: xs = 20 px, sm = 22 px, md = 26 px.",
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
  render: () => <Pill mono>task-102</Pill>,
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

export const ToggleInteractive: Story = {
  args: {},
  render: () => (
    <div className="flex flex-wrap items-center gap-2">
      <Pill mono active render={<button type="button" />}>
        all
      </Pill>
      <Pill mono active={false} render={<button type="button" />}>
        running
      </Pill>
      <Pill mono active={false} render={<button type="button" />}>
        completed
      </Pill>
    </div>
  ),
  parameters: {
    docs: {
      description: {
        story: "Pass `render={<button />}` and `active` to render a stand-alone toggle chip.",
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

export const Count: Story = {
  parameters: {
    docs: {
      description: {
        story:
          "`Pill.Count` is the compact numeric badge for unread and needs-you counts: solid accent orange, semibold (600) digits, capped at `9+`, and absent at zero.",
      },
    },
  },
  render: () => (
    <div className="flex items-center gap-3">
      <Pill.Count count={1} />
      <Pill.Count count={7} />
      <Pill.Count count={42} />
      <Pill.Count count={3} tone="neutral" />
    </div>
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByText("9+")).toBeInTheDocument();
  },
};
