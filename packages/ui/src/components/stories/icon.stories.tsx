import type { Meta, StoryObj } from "@storybook/react-vite";
import { Activity, AlertTriangle, CircleAlert, Sparkles } from "lucide-react";

import { Icon } from "../icon";

const meta: Meta<typeof Icon> = {
  title: "components/Icon",
  component: Icon,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Lucide icon adapter that pins the 1.75 stroke-width default (2 at xs) and quantises sizes to 11 / 12 / 14 / 16 px",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** All four size steps stacked together. */
export const SizeRamp: Story = {
  args: { as: Sparkles },
  render: () => (
    <div className="flex items-end gap-4 text-fg-strong">
      <div className="flex flex-col items-center gap-2">
        <Icon as={Sparkles} size="xs" />
        <span className="text-[10px] text-muted">xs · 11</span>
      </div>
      <div className="flex flex-col items-center gap-2">
        <Icon as={Sparkles} size="sm" />
        <span className="text-[10px] text-muted">sm · 12</span>
      </div>
      <div className="flex flex-col items-center gap-2">
        <Icon as={Sparkles} />
        <span className="text-[10px] text-muted">default · 14</span>
      </div>
      <div className="flex flex-col items-center gap-2">
        <Icon as={Sparkles} size="lg" />
        <span className="text-[10px] text-muted">lg · 16</span>
      </div>
    </div>
  ),
};

/** Token-colored icons in the signal palette. */
export const Tones: Story = {
  args: { as: Sparkles },
  render: () => (
    <div className="flex items-center gap-4">
      <Icon as={Activity} className="text-accent" />
      <Icon as={AlertTriangle} className="text-warning" />
      <Icon as={CircleAlert} className="text-danger" />
      <Icon as={Sparkles} className="text-success" />
    </div>
  ),
};

/** Explicit strokeWidth override (for example, large branded marks). */
export const StrokeOverride: Story = {
  args: { as: Sparkles },
  render: () => (
    <div className="flex items-center gap-4 text-fg-strong">
      <Icon as={Sparkles} strokeWidth={1.25} />
      <Icon as={Sparkles} />
      <Icon as={Sparkles} strokeWidth={2.5} />
    </div>
  ),
};
