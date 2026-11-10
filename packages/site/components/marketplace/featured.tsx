/** Featured comes from the catalog's own curation; the art only frames it. */
import { cn } from "@compozy/ui";
import Image from "next/image";
import Link from "next/link";

import { ASSET } from "@/components/landing/assets";
import { EntryLogo } from "@/components/marketplace/entry-card";
import { CONTAINER } from "@/components/site/layout";
import { Reveal } from "@/components/site/motion";
import { H2 } from "@/components/site/primitives";
import { firstSentence, type MarketplaceCard } from "@/lib/marketplace-cards";

const ART = [
  ASSET.plateTerrain,
  ASSET.ucTriage,
  ASSET.platePulses,
  ASSET.plateBranch,
  ASSET.plateFinal,
  ASSET.plateGates,
  ASSET.plateOrder,
  ASSET.ucSpec,
];

export function Featured({ entries }: { entries: readonly MarketplaceCard[] }) {
  if (entries.length === 0) return null;
  return (
    <section className={cn(CONTAINER, "pt-8 pb-24")}>
      <Reveal>
        <H2 size="sm">Featured</H2>
      </Reveal>
      <div className="mt-10 grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {entries.map((entry, index) => (
          <Reveal key={entry.id} delay={index * 0.05}>
            <Link
              href={entry.href}
              className="group/feat block w-full cursor-pointer overflow-hidden rounded-[20px] border border-site-light-080 bg-site-light-025 p-0 text-left outline-none transition-[border-color] duration-300 hover:border-site-light-200 focus-visible:ring-2 focus-visible:ring-brand"
            >
              <span className="relative grid h-[210px] place-items-center overflow-hidden bg-black">
                <Image
                  src={ART[index % ART.length]}
                  alt=""
                  aria-hidden
                  fill
                  sizes="(min-width: 1280px) 300px, (min-width: 640px) 50vw, 100vw"
                  draggable={false}
                  className="object-cover opacity-75 transition-transform duration-[1200ms] ease-out group-hover/feat:scale-[1.07]"
                />
                <span
                  aria-hidden
                  className="absolute inset-x-0 bottom-0 h-20 bg-gradient-to-t from-[#0e0e0e] to-transparent"
                />
                <EntryLogo
                  entry={entry}
                  size="xl"
                  className="relative shadow-[inset_0_0_0_1px_rgba(255,255,255,0.14),0_30px_60px_-14px_rgba(0,0,0,0.9)]"
                />
              </span>
              <span className="block px-6 pt-4 pb-6">
                <span className="block text-[20px]/[1.25] font-medium text-fg">{entry.name}</span>
                <span className="mt-1.5 line-clamp-2 min-h-[44px] text-[14.5px]/[1.5] text-muted-foreground">
                  {firstSentence(entry.description)}
                </span>
              </span>
            </Link>
          </Reveal>
        ))}
      </div>
    </section>
  );
}
