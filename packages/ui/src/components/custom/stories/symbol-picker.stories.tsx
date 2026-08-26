import type { Meta, StoryObj } from "@storybook/react-vite";
import spriteUrl from "lucide-static/sprite.svg?url";
import { useEffect, useState } from "react";
import { fireEvent, within } from "storybook/test";

import type { SymbolIconOption, SymbolSwatch, SymbolValue } from "../../../lib/symbol-palette";
import { SymbolPicker } from "../symbol-picker";

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

/** The real catalog: every Lucide slug with its search tags. */
function useLucideCatalog(): { icons: readonly SymbolIconOption[]; loading: boolean } {
  const [icons, setIcons] = useState<readonly SymbolIconOption[]>([]);
  useEffect(() => {
    let cancelled = false;
    import("lucide-static/tags.json").then(module => {
      if (cancelled) return;
      const tags = module.default as Record<string, readonly string[]>;
      setIcons(
        Object.entries(tags).map(([name, keywords]) => ({
          name,
          label: name.replace(/-/g, " "),
          keywords: keywords.join(" "),
        }))
      );
    });
    return () => {
      cancelled = true;
    };
  }, []);
  return { icons, loading: icons.length === 0 };
}

const meta: Meta<typeof SymbolPicker> = {
  title: "components/custom/SymbolPicker",
  component: SymbolPicker,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Pick a symbol and a color together across the full Lucide catalog and every emoji. The grid re-inks as the color changes so the pairing is judged live; icons render from one shared sprite and the grid virtualizes past the first rows.",
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
  const catalog = useLucideCatalog();
  return (
    <SymbolPicker
      color={color}
      onColorChange={setColor}
      symbol={symbol}
      onSymbolChange={setSymbol}
      icons={catalog.icons}
      iconsLoading={catalog.loading}
      spriteUrl={spriteUrl}
      emojibaseUrl="/vendor/emojibase"
      swatches={SWATCHES}
    />
  );
}

/** The full virtualized catalog with a violet identity tinting every cell. */
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
        story: "The Emojis tab is a frimousse composition with search and skin-tone control.",
      },
    },
  },
  render: () => <Harness initial={{ kind: "emoji", value: "🌱" }} initialColor="#4cb782" />,
};

/** The selected tab remains visible while an empty search points to the other symbol family. */
export const IconsSearchEmpty: Story = {
  args: {} as never,
  tags: ["play-fn"],
  render: () => <Harness initial={{ kind: "icon", value: "compass" }} initialColor="#4cb782" />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    fireEvent.change(await canvas.findByLabelText("Search icons"), {
      target: { value: "zzzz-no-such-icon" },
    });
    await canvas.findByText('No icons match "zzzz-no-such-icon". Try the Emojis tab.');
  },
};

/** A malformed custom color stays inline and preserves the last valid identity color. */
export const InvalidCustomHex: Story = {
  args: {} as never,
  tags: ["play-fn"],
  render: () => <Harness initial={{ kind: "icon", value: "megaphone" }} initialColor="#c26ad6" />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const input = canvas.getByLabelText("Custom color");
    fireEvent.change(input, { target: { value: "12ZZ" } });
    await canvas.findByText("Enter a color like #4ea7fc.");
  },
};

/** The spectrum toggle opens the free saturation + hue area. */
export const CustomColorArea: Story = {
  args: {} as never,
  tags: ["play-fn"],
  render: () => <Harness initial={{ kind: "icon", value: "compass" }} initialColor="#4ea7fc" />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    fireEvent.click(canvas.getByRole("button", { name: "Pick a custom color" }));
    await within(canvasElement).findByLabelText("Hue");
  },
};
