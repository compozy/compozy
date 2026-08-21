import type { Meta, StoryObj } from "@storybook/react-vite";
import {
  Briefcase,
  Compass,
  Flame,
  Heart,
  Leaf,
  Megaphone,
  Palette,
  Rocket,
  Sparkles,
  Star,
  TrendingUp,
  Zap,
} from "lucide-react";
import { useState } from "react";

import type { KindIconRegistry } from "../kind-icon-registry";
import type { SymbolSwatch, SymbolValue } from "../../../lib/symbol-palette";
import { SymbolPicker } from "../symbol-picker";

const ICON_REGISTRY = {
  megaphone: Megaphone,
  briefcase: Briefcase,
  rocket: Rocket,
  palette: Palette,
  heart: Heart,
  star: Star,
  zap: Zap,
  flame: Flame,
  leaf: Leaf,
  compass: Compass,
  sparkles: Sparkles,
  "trending-up": TrendingUp,
} satisfies KindIconRegistry;

const ICONS = Object.keys(ICON_REGISTRY).map(name => ({ name }));

const EMOJIS = [
  { value: "🚀", label: "rocket", keywords: "launch ship" },
  { value: "🌱", label: "seedling", keywords: "growth plant" },
  { value: "🎨", label: "palette", keywords: "art design" },
  { value: "📣", label: "megaphone", keywords: "marketing announce" },
  { value: "💼", label: "briefcase", keywords: "work business" },
  { value: "📈", label: "chart", keywords: "growth revenue" },
  { value: "☕", label: "coffee", keywords: "break" },
  { value: "🎯", label: "target", keywords: "goal focus" },
];

const SWATCHES: SymbolSwatch[] = [
  { label: "Gray", value: "#8a8f98" },
  { label: "Blue", value: "#4ea7fc" },
  { label: "Teal", value: "#2aa8a8" },
  { label: "Green", value: "#4cb782" },
  { label: "Violet", value: "#c26ad6" },
  { label: "Pink", value: "#e56aa2" },
  { label: "Amber", value: "#e8b04a" },
  { label: "Brown", value: "#b58e5f" },
];

const meta: Meta<typeof SymbolPicker> = {
  title: "components/custom/SymbolPicker",
  component: SymbolPicker,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Pick a symbol and a color together. The grid re-inks as the color changes so the pairing is judged live rather than imagined; the foreground is derived and measured against the surface it lands on, never assumed.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-[32rem]">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

function Harness({ initial, initialColor }: { initial: SymbolValue; initialColor: string }) {
  const [symbol, setSymbol] = useState<SymbolValue>(initial);
  const [color, setColor] = useState(initialColor);
  return (
    <SymbolPicker
      color={color}
      onColorChange={setColor}
      symbol={symbol}
      onSymbolChange={setSymbol}
      icons={ICONS}
      iconRegistry={ICON_REGISTRY}
      emojis={EMOJIS}
      swatches={SWATCHES}
    />
  );
}

/** Icons tab with a violet identity — the whole grid carries the chosen hue. */
export const IconsTab: Story = {
  args: {} as never,
  render: () => <Harness initial={{ kind: "icon", value: "megaphone" }} initialColor="#c26ad6" />,
};

/** Emoji keep their own ink; the identity color owns the selection plate. */
export const EmojisTab: Story = {
  args: {} as never,
  parameters: {
    docs: {
      description: {
        story: "Switch to the Emojis tab to see the selection plate carry the identity color.",
      },
    },
  },
  render: () => <Harness initial={{ kind: "emoji", value: "🌱" }} initialColor="#4cb782" />,
};

/** A near-white identity flips the derived ink dark so the grid stays readable. */
export const LightIdentity: Story = {
  args: {} as never,
  render: () => <Harness initial={{ kind: "icon", value: "compass" }} initialColor="#f2f0ec" />,
};
