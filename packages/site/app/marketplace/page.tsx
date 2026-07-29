import { Eyebrow } from "@compozy/ui";
import { ArrowRight, GitPullRequest, Terminal } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { MarketplaceEntryCard } from "@/components/marketplace/marketplace-entry-card";
import { MarketplaceInstallCommand } from "@/components/marketplace/marketplace-install-command";
import { MARKETPLACE_KIND_META } from "@/components/marketplace/marketplace-kind-meta";
import { entriesForKind } from "@/lib/marketplace-catalog";
import { createPageMetadata, siteConfig } from "@/lib/site-config";

export const metadata: Metadata = createPageMetadata({
  title: "Marketplace",
  description:
    "Skills, extensions, and MCP servers for CompozyOS — every entry rendered from the same catalog feeds your daemon reads.",
  path: "/marketplace",
});

const CONTRIBUTION_GUIDE_URL = `${siteConfig.repoUrl}/tree/main/catalog`;

export default function MarketplacePage() {
  return (
    <main id="main-content" className="mx-auto w-full max-w-site-layout-width px-4 pt-14 pb-20">
      <header className="max-w-[62ch]">
        <Eyebrow className="text-muted">Marketplace</Eyebrow>
        <h1 className="mt-3 text-4xl font-semibold tracking-[-0.025em] text-fg">
          Install what your agents need
        </h1>
        <p className="mt-4 text-base leading-relaxed text-muted">
          Skills, extensions, and MCP servers — every entry on this page renders from the same
          catalog feeds your daemon reads. Browse here, install with one command.
        </p>
        <MarketplaceInstallCommand command="compozy marketplace" className="mt-6 max-w-sm" />
        <p className="mt-5 flex items-start gap-2.5 text-small-body leading-relaxed text-subtle">
          <Terminal aria-hidden className="mt-0.5 size-3.5 shrink-0" />
          <span>
            The CLI also reaches third-party registries: <code>compozy skill search</code> and{" "}
            <code>compozy extension search</code> cover ClawHub and GitHub Releases beyond this
            catalog.
          </span>
        </p>
      </header>

      <div className="mt-14 flex flex-col gap-14">
        {MARKETPLACE_KIND_META.map(meta => {
          const entries = entriesForKind(meta.kind);
          return (
            <section
              key={meta.kind}
              aria-labelledby={`kind-${meta.kind}`}
              className="grid gap-6 border-t border-line pt-10 lg:grid-cols-[minmax(0,17rem)_minmax(0,1fr)] lg:gap-10"
            >
              <div>
                <Eyebrow className="text-subtle">Kind</Eyebrow>
                <h2
                  id={`kind-${meta.kind}`}
                  className="mt-2 flex items-baseline gap-2.5 text-xl font-semibold tracking-[-0.015em] text-fg"
                >
                  {meta.title}
                  <span className="font-mono text-small-body font-normal text-subtle">
                    {entries.length}
                  </span>
                </h2>
                <p className="mt-3 text-small-body leading-relaxed text-muted">
                  {meta.description}
                </p>
                <Link
                  href={`/marketplace/${meta.kind}`}
                  className="mt-4 inline-flex items-center gap-1.5 text-small-body font-medium text-muted transition-colors hover:text-accent"
                >
                  View all {meta.title.toLowerCase()}
                  <ArrowRight aria-hidden className="size-3.5" />
                </Link>
              </div>
              <div className="flex flex-col gap-4">
                {entries.map(entry => (
                  <MarketplaceEntryCard key={entry.entry_id} kind={meta.kind} entry={entry} />
                ))}
              </div>
            </section>
          );
        })}
      </div>

      <section
        aria-labelledby="contribute"
        className="mt-16 flex flex-col gap-5 rounded-lg border border-line bg-canvas-soft p-6 sm:flex-row sm:items-center sm:justify-between sm:gap-8"
      >
        <div className="max-w-[60ch]">
          <h2 id="contribute" className="text-lg font-semibold tracking-[-0.01em] text-fg">
            Add an entry to the catalog
          </h2>
          <p className="mt-2 text-small-body leading-relaxed text-muted">
            The catalog is a set of JSON feeds in the Compozy repository. Package your artifact with{" "}
            <code>compozy-catalog package</code>, validate it with{" "}
            <code>compozy-catalog validate</code>, and open a pull request against{" "}
            <code>catalog/</code> — the site and the daemon pick the entry up from the same feed.
          </p>
        </div>
        <a
          href={CONTRIBUTION_GUIDE_URL}
          target="_blank"
          rel="noreferrer noopener"
          className="inline-flex shrink-0 items-center gap-2 rounded-lg border border-line px-4 py-2 text-small-body font-medium text-fg transition-colors hover:border-accent/35 hover:text-accent"
        >
          <GitPullRequest aria-hidden className="size-4" />
          Read the contribution guide
        </a>
      </section>
    </main>
  );
}
