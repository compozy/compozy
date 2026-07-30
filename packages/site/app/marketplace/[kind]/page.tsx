import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { MarketplaceContribute } from "@/components/marketplace/marketplace-contribute";
import { MarketplaceCrumbs } from "@/components/marketplace/marketplace-crumbs";
import { MarketplaceKindBrowser } from "@/components/marketplace/marketplace-kind-browser";
import { MarketplaceKindTabs } from "@/components/marketplace/marketplace-kind-tabs";
import { kindMeta } from "@/components/marketplace/marketplace-kind-meta";
import { entriesForKind, isMarketplaceKind, MARKETPLACE_KINDS } from "@/lib/marketplace-catalog";
import { createPageMetadata } from "@/lib/site-config";

interface PageProps {
  params: Promise<{ kind: string }>;
}

export function generateStaticParams() {
  return MARKETPLACE_KINDS.map(kind => ({ kind }));
}

export async function generateMetadata(props: PageProps): Promise<Metadata> {
  const { kind } = await props.params;
  if (!isMarketplaceKind(kind)) notFound();
  const meta = kindMeta(kind);

  return createPageMetadata({
    title: `${meta.title} — Marketplace`,
    description: meta.description,
    path: `/marketplace/${kind}`,
  });
}

export default async function MarketplaceKindPage(props: PageProps) {
  const { kind } = await props.params;
  if (!isMarketplaceKind(kind)) notFound();
  const meta = kindMeta(kind);
  const entries = entriesForKind(kind);

  return (
    <main id="main-content" className="w-full pb-20">
      <header className="border-b border-line bg-canvas-soft">
        <div className="mx-auto w-full max-w-site-layout-width px-4 pt-10 pb-9">
          <MarketplaceCrumbs leaf={meta.title} />
          <h1 className="mt-4 font-display text-site-doc-heading leading-tight font-normal tracking-tight text-fg">
            All <em>{meta.title.toLowerCase()}</em>
          </h1>
          <p className="mt-3.5 max-w-[64ch] text-site-lead text-muted">
            Every {meta.noun.toLowerCase()} in this build&apos;s checked-in catalog snapshot. Run{" "}
            <code>compozy marketplace search</code> before installing because your daemon&apos;s
            active catalog can differ.
          </p>
        </div>
      </header>

      <div className="mx-auto w-full max-w-site-layout-width px-4 pt-11">
        <MarketplaceKindBrowser
          kind={kind}
          entries={entries}
          description={meta.description}
          noun={meta.title.toLowerCase()}
          tabs={<MarketplaceKindTabs active={kind} />}
        />
        <div className="mt-14">
          <MarketplaceContribute />
        </div>
      </div>
    </main>
  );
}
