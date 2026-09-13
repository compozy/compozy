import { Eyebrow, Pill } from "@compozy/ui";
import { extensionEntries, marketplacePresets } from "@/lib/marketplace-catalog";
import { MarketplaceCatalogBrowser } from "./marketplace-catalog-browser";

/**
 * The catalog region of `/marketplace`: one heading for one kind of thing, then the searchable
 * grid. The plugin marketplace presets from `v3/marketplaces.json` are listed as data next to the
 * heading — they are sources a daemon can enable, not entries this build can render, so they get a
 * line each and no cards.
 */
export function MarketplaceCatalogSection() {
  return (
    <section
      id="catalog"
      aria-labelledby="catalog-heading"
      className="scroll-mt-24 border-t border-line pt-10"
    >
      <div className="grid gap-6 lg:grid-cols-[minmax(0,17rem)_minmax(0,1fr)] lg:gap-10">
        <div>
          <Eyebrow className="text-subtle">Catalog</Eyebrow>
          <h2
            id="catalog-heading"
            className="mt-2 flex items-baseline gap-2.5 text-xl font-semibold tracking-[-0.015em] text-fg"
          >
            Extensions
            <span className="font-mono text-small-body font-normal text-subtle">
              {extensionEntries.length}
            </span>
          </h2>
          <p className="mt-3 text-small-body leading-relaxed text-muted">
            Packages that add MCP servers, skills, loops, agents, and Host API actions to the
            runtime. Every artifact ships with a SHA-256 digest and a provenance tier, and a package
            that needs configuration declares its inputs up front.
          </p>
        </div>

        <div>
          <Eyebrow className="text-subtle">Plugin marketplaces</Eyebrow>
          <p className="mt-2 text-small-body leading-relaxed text-muted">
            Marketplaces from other agent clients that a daemon can enable as catalog sources. This
            build ships {marketplacePresets.length} presets; their plugins list in the runtime, not
            here.
          </p>
          <ul className="mt-3 flex flex-col gap-2">
            {marketplacePresets.map(preset => (
              <li
                key={preset.name}
                className="flex flex-wrap items-center gap-x-2.5 gap-y-1 text-small-body text-muted"
              >
                <code className="font-mono text-fg">{preset.name}</code>
                <Pill size="xs" form="hollow">
                  {preset.default === "on" ? "on by default" : "off by default"}
                </Pill>
                <span>{preset.description}</span>
                <span className="font-mono text-mono-id text-faint">{preset.source}</span>
              </li>
            ))}
          </ul>
        </div>
      </div>

      <div className="mt-8">
        <MarketplaceCatalogBrowser entries={extensionEntries} />
      </div>
    </section>
  );
}
