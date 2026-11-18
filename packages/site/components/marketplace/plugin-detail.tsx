/** One preset-marketplace plugin on the entry detail layout, from snapshot fields only. */
import { Pill } from "@compozy/ui";
import Link from "next/link";

import {
  DetailCommand,
  DetailFrame,
  HomepageLink,
  InstallAside,
  Row,
} from "@/components/marketplace/detail-layout";
import { EntryLogo } from "@/components/marketplace/entry-card";
import { Muted } from "@/components/site/primitives";
import { pluginSearchCommand, type MarketplacePlugin } from "@/lib/marketplace-plugins";
import { categoryLabel, pluginCard, relatedCards } from "@/lib/marketplace-view";

/** Every plugin from a preset marketplace is unverified: the daemon reviews only its own catalog. */
const PLUGIN_TRUST = "Unverified";

export function PluginDetail({ entry }: { entry: MarketplacePlugin }) {
  const card = pluginCard(entry);
  const homepage = entry.plugin.homepage ?? entry.plugin.repository;
  return (
    <DetailFrame
      logo={<EntryLogo entry={card} size="xl" />}
      name={card.name}
      byline={
        <>
          {card.author ? <span>By {card.author}</span> : null}
          <Link href={`/marketplace?category=${card.category}#catalog`}>
            <Pill size="sm" form="hollow">
              {categoryLabel(card.category)}
            </Pill>
          </Link>
        </>
      }
      description={card.description}
      aside={
        <InstallAside
          title="Add it from the app"
          lead={
            <Muted className="mt-2">
              Open Marketplace in CompozyOS and add {card.name} from there, or find it from the
              terminal.
            </Muted>
          }
          command={<DetailCommand command={pluginSearchCommand(entry)} />}
          facts={
            <>
              <Row term="Type">Plugin</Row>
              <Row term="Trust">{PLUGIN_TRUST}</Row>
              <Row term="From">{entry.plugin.source}</Row>
              {entry.plugin.version ? <Row term="Version">{entry.plugin.version}</Row> : null}
              {homepage && /^https?:\/\//.test(homepage) ? (
                <Row term="Homepage">
                  <HomepageLink href={homepage} />
                </Row>
              ) : null}
            </>
          }
          note="The app shows what it contains and asks you to confirm before it installs."
        />
      }
      relatedTitle={`More in ${categoryLabel(card.category)}`}
      related={relatedCards(card)}
    />
  );
}
