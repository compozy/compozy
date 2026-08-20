import type { Meta, StoryObj } from "@storybook/react-vite";

import { Eyebrow } from "../eyebrow";

const meta: Meta<typeof Eyebrow> = {
  title: "components/custom/Eyebrow",
  component: Eyebrow,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          'Canonical eyebrow primitive — single Geist 12 px / 510 / -0.005em contract, sentence case. `variant="caps"` is the only opt-in: it adds uppercase on the same size and tracking tokens, so there is still one eyebrow source (/ §11, lesson L-022). Tone, size, and weight stay collapsed; apply text-color utilities through `className` when a tone is needed (`text-muted`, `text-subtle`, `text-accent`, signal palette).',
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const TONES: { label: string; className: string }[] = [
  { label: "default (inherit)", className: "" },
  { label: "muted", className: "text-muted" },
  { label: "subtle", className: "text-subtle" },
  { label: "strong", className: "text-fg-strong" },
  { label: "accent", className: "text-accent" },
  { label: "success", className: "text-success" },
  { label: "warning", className: "text-warning" },
  { label: "danger", className: "text-danger" },
  { label: "info", className: "text-info" },
];

export const Default: Story = {
  args: {
    children: "Active sessions",
  },
};

export const Tones: Story = {
  parameters: {
    docs: {
      description: {
        story:
          "Tones are now applied through className text-color utilities. The eyebrow utility itself does not bind a default tone — consumers inherit body color unless they pass a text-(--*) class.",
      },
    },
  },
  render: () => (
    <div className="grid grid-cols-[160px_1fr] items-baseline gap-x-6 gap-y-3">
      {TONES.map(({ label, className }) => (
        <div key={label} className="contents">
          <span className="text-badge text-subtle">{label}</span>
          <Eyebrow className={className}>Active sessions</Eyebrow>
        </div>
      ))}
    </div>
  ),
};

export const Caps: Story = {
  parameters: {
    docs: {
      description: {
        story:
          "The opt-in uppercase kicker. Reach for it only where a label is a true typographic kicker; structural labels — table heads, section titles, card meta, KPI labels — stay sentence case.",
      },
    },
  },
  render: () => (
    <div className="flex flex-col gap-3">
      <Eyebrow className="text-muted">Active sessions</Eyebrow>
      <Eyebrow variant="caps" className="text-muted">
        Active sessions
      </Eyebrow>
    </div>
  ),
};
