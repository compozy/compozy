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
          "Canonical eyebrow primitive — single Inter UC 11 px / 600 / -0.005em contract. The component is prop-less (children + className only); tone, size, case, and weight have been collapsed (/ §11, lesson L-022). Apply text-color utilities through `className` when a tone is needed (`text-muted`, `text-subtle`, `text-accent`, signal palette).",
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
          <span className="text-[11px] text-subtle">{label}</span>
          <Eyebrow className={className}>Active sessions</Eyebrow>
        </div>
      ))}
    </div>
  ),
};
