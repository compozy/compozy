import { Eyebrow } from "@compozy/ui";
import { ArrowRight, Terminal } from "lucide-react";
import Link from "next/link";
import { bridgeProviders } from "@/lib/marketplace-bridges";
import { bundledSkills, specCycleExtension } from "@/lib/marketplace-bundled";
import {
  MARKETPLACE_FEED_FILENAMES,
  MARKETPLACE_SEARCH_COMMAND,
  extensionEntries,
  mcpEntries,
  skillEntries,
} from "@/lib/marketplace-catalog";
import { MarketplaceFeedPipeline } from "./marketplace-feed-pipeline";
import { MarketplaceInstallCommand } from "./marketplace-install-command";

/**
 * Every number here is counted from the repository at build time. Nothing is a popularity metric,
 * because no such field exists in the feeds (`internal/marketplace/entry_*.go`).
 */
function heroStats() {
  const bundledResources =
    specCycleExtension.loops.length +
    specCycleExtension.skills.length +
    specCycleExtension.agents.length +
    specCycleExtension.tools.length +
    bundledSkills.length;

  return [
    {
      value: skillEntries.length + extensionEntries.length + mcpEntries.length,
      label: "Catalog entries",
    },
    { value: bridgeProviders.length, label: "Bridge providers" },
    { value: bundledResources, label: "Bundled resources" },
    { value: MARKETPLACE_FEED_FILENAMES.length, label: "JSON feeds" },
  ];
}

export function MarketplaceHero() {
  return (
    <header className="relative border-b border-line bg-canvas-soft">
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0 bg-[radial-gradient(640px_340px_at_82%_-20%,var(--color-accent-tint),transparent_70%)]"
      />
      <div className="relative mx-auto w-full max-w-site-layout-width px-4 pt-14 pb-11">
        <div className="grid items-center gap-12 lg:grid-cols-[minmax(0,1fr)_420px] lg:gap-18">
          <div>
            <Eyebrow className="text-accent">Marketplace</Eyebrow>
            <h1 className="mt-3.5 font-display text-site-doc-title leading-tight font-normal tracking-tight text-fg">
              Install what your agents <em>need</em>
            </h1>
            <p className="mt-4 max-w-[64ch] text-site-lead text-muted">
              Skills, MCP servers, extensions, and bridge providers from this build&apos;s
              checked-in catalog snapshot. Search your daemon before installing: its active catalog
              can differ.
            </p>

            <div className="mt-7 flex flex-wrap items-center gap-4.5">
              <MarketplaceInstallCommand
                command={MARKETPLACE_SEARCH_COMMAND}
                className="min-w-0 flex-1 basis-85"
              />
              <Link
                href="/marketplace/skills"
                className="inline-flex shrink-0 items-center gap-1.5 text-small-body font-medium text-muted transition-colors hover:text-fg"
              >
                Browse the catalog
                <ArrowRight aria-hidden className="size-3" />
              </Link>
            </div>

            <dl className="mt-8.5 flex flex-wrap border-t border-line pt-4">
              {heroStats().map(stat => (
                <div
                  key={stat.label}
                  className="min-w-0 flex-1 basis-32 ps-5.5 not-first:border-s not-first:border-line first:ps-0"
                >
                  <dt className="font-mono text-kpi-compact font-medium tracking-tight text-fg-strong">
                    {stat.value}
                  </dt>
                  <dd className="mt-0.5 text-subtle">
                    <Eyebrow>{stat.label}</Eyebrow>
                  </dd>
                </div>
              ))}
            </dl>
          </div>

          <MarketplaceFeedPipeline />
        </div>

        <p className="mt-9 flex items-start gap-2.5 text-small-body leading-relaxed text-subtle">
          <Terminal aria-hidden className="mt-0.5 size-3.5 shrink-0" />
          <span>
            The CLI also reaches third-party registries: <code>compozy skill search</code> and{" "}
            <code>compozy extension search</code> cover ClawHub and GitHub Releases beyond this
            catalog.
          </span>
        </p>
      </div>
    </header>
  );
}
