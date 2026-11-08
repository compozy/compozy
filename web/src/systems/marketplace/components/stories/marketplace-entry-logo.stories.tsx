import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, waitFor } from "storybook/test";

import { CenteredSurface } from "@/storybook/story-layout";

import { logoLadderCatalog } from "../../routes/marketplace-route-story-fixtures";
import { MarketplaceEntryLogo, MarketplaceMarbleProvider } from "../marketplace-entry-logo";

const meta: Meta<typeof MarketplaceEntryLogo> = {
  title: "systems/marketplace/components/MarketplaceEntryLogo",
  component: MarketplaceEntryLogo,
};

export default meta;
type Story = StoryObj<typeof meta>;

function throwingMarble(): never {
  throw new Error("Storybook marble generator failure");
}

/**
 * Exercise the production boundary when the external marble generator throws: the Logo ladder
 * entries render through a generator that always throws, so every marble rung falls back to the
 * entry's monogram while the icon and brand rungs are unaffected.
 */
export const GeneratorFailure: Story = {
  render: () => (
    <MarketplaceMarbleProvider value={throwingMarble}>
      <CenteredSurface className="gap-3">
        {logoLadderCatalog.map(entry => (
          <MarketplaceEntryLogo entry={entry} key={entry.entry_id} size="md" />
        ))}
      </CenteredSurface>
    </MarketplaceMarbleProvider>
  ),
  play: async ({ canvasElement }) => {
    const logos = () => [
      ...canvasElement.querySelectorAll<HTMLElement>('[data-slot="marketplace-entry-logo"]'),
    ];
    // Icon and brand rungs are unaffected; both marble rungs fall back to their monogram.
    await waitFor(() => {
      expect(logos().map(logo => logo.dataset.rung)).toEqual([
        "icon",
        "brand",
        "monogram",
        "monogram",
      ]);
    });
    const monograms = logos().filter(logo => logo.dataset.rung === "monogram");
    expect(monograms.map(logo => logo.textContent)).toEqual(
      logoLadderCatalog.slice(2).map(entry => entry.name.trim().charAt(0).toUpperCase())
    );
  },
};

const ICON_ROOT = "https://raw.githubusercontent.com/compozy/compozy/main/catalog/icons/";

/** Catalog entries whose feed image is a fixed-ink monochrome mark, beside a colored one. */
const monochromeBrandCatalog = [
  { entry_id: "github", name: "GitHub", icon: `${ICON_ROOT}github.svg` },
  { entry_id: "linear", name: "Linear", icon: `${ICON_ROOT}linear.svg` },
  { entry_id: "notion", name: "Notion", icon: `${ICON_ROOT}notion.png` },
];

/**
 * GitHub and Linear ship white feed images; their theme-following brand marks draw instead, so
 * both read on the light and the dark logo well. Notion keeps its colored feed image.
 */
export const MonochromeBrands: Story = {
  render: () => (
    <CenteredSurface className="gap-3">
      {monochromeBrandCatalog.map(entry => (
        <MarketplaceEntryLogo entry={entry} key={entry.entry_id} size="lg" />
      ))}
    </CenteredSurface>
  ),
  play: async ({ canvasElement }) => {
    const logos = [
      ...canvasElement.querySelectorAll<HTMLElement>('[data-slot="marketplace-entry-logo"]'),
    ];
    expect(logos.map(logo => logo.dataset.rung)).toEqual(["brand", "brand", "icon"]);
  },
};
