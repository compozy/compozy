/** A bundled extension on the entry detail layout: what it contains instead of what it asks for. */
import { Pill, cn } from "@compozy/ui";
import Link from "next/link";

import {
  ContentsBlock,
  DetailCommand,
  DetailFrame,
  HomepageLink,
  InstallAside,
  Row,
} from "@/components/marketplace/detail-layout";
import { EntryLogo } from "@/components/marketplace/entry-card";
import { HAIRLINE } from "@/components/site/layout";
import { Muted } from "@/components/site/primitives";
import { bundledExtensions, type BundledExtension } from "@/lib/marketplace-bundled";
import type { MarketplaceCard } from "@/lib/marketplace-cards";

const BUNDLED_AUTHOR = "CompozyOS";

function plural(count: number, noun: string) {
  return `${count} ${count === 1 ? noun : `${noun}s`}`;
}

function bundledCard(extension: BundledExtension): MarketplaceCard {
  return {
    id: extension.name,
    kind: "extension",
    name: extension.displayName,
    author: BUNDLED_AUTHOR,
    category: "more",
    description: extension.description,
    icon: null,
    bleed: false,
    light: false,
    featured: false,
    href: extension.path,
  };
}

function Names({ names }: { names: readonly string[] }) {
  return (
    <div className="flex flex-wrap gap-2">
      {names.map(name => (
        <Pill key={name} size="sm" form="hollow" mono>
          {name}
        </Pill>
      ))}
    </div>
  );
}

export function BundledDetail({
  extension,
  walkthroughs,
}: {
  extension: BundledExtension;
  /** Loop name → docs walkthrough URL, when the docs have one. */
  walkthroughs: ReadonlyMap<string, string>;
}) {
  const { loops, skills, agents, tools } = extension;
  const related: MarketplaceCard[] = [];
  for (const item of bundledExtensions) {
    if (item.name !== extension.name) related.push(bundledCard(item));
  }
  return (
    <DetailFrame
      logo={<EntryLogo entry={{ name: extension.displayName, icon: null }} size="xl" />}
      name={extension.displayName}
      byline={
        <>
          <span>By {BUNDLED_AUTHOR}</span>
          <Pill size="sm" form="hollow">
            Included with CompozyOS
          </Pill>
        </>
      }
      description={extension.description}
      aside={
        <InstallAside
          title="Included with CompozyOS"
          lead={
            <Muted className="mt-2">
              Nothing to install. Updating CompozyOS updates these. Check it from the terminal.
            </Muted>
          }
          command={<DetailCommand command={extension.statusCommand} />}
          facts={
            <>
              <Row term="Type">Extension</Row>
              <Row term="Trust">Official</Row>
              <Row term="From">Bundled</Row>
              <Row term="Version">{extension.version}</Row>
              <Row term="Requires">CompozyOS ≥ {extension.minCompozyVersion}</Row>
              <Row term="Homepage">
                <HomepageLink href={extension.repositoryUrl} />
              </Row>
            </>
          }
          note="Compiled into the compozy binary and enrolled the first time the daemon starts."
        />
      }
      relatedTitle="More included with CompozyOS"
      related={related}
    >
      {loops.length > 0 ? (
        <ContentsBlock title={plural(loops.length, "loop")}>
          <ul className="m-0 list-none p-0">
            {loops.map(loop => {
              const walkthrough = walkthroughs.get(loop.name);
              return (
                <li key={loop.name} className={cn("border-t py-4", HAIRLINE)}>
                  <p className="m-0 flex flex-wrap items-center gap-2 font-mono text-[14.5px] text-fg">
                    {loop.name}
                    {loop.category ? (
                      <Pill size="xs" form="hollow" mono>
                        {loop.category}
                      </Pill>
                    ) : null}
                  </p>
                  <Muted className="mt-1">{loop.description}</Muted>
                  {loop.useWhen ? (
                    <p className="m-0 mt-1.5 text-[14px]/[1.55] text-subtle">
                      <span className="text-muted-foreground">Use when</span> {loop.useWhen}
                    </p>
                  ) : null}
                  {walkthrough ? (
                    <Link
                      href={walkthrough}
                      className="mt-2 inline-flex text-[14px] text-fg underline-offset-4 hover:underline"
                    >
                      Read the walkthrough
                    </Link>
                  ) : null}
                </li>
              );
            })}
          </ul>
        </ContentsBlock>
      ) : null}
      {skills.length > 0 ? (
        <ContentsBlock title={plural(skills.length, "skill")}>
          <Names names={skills} />
        </ContentsBlock>
      ) : null}
      {agents.length > 0 ? (
        <ContentsBlock title={plural(agents.length, "agent")}>
          <Names names={agents} />
        </ContentsBlock>
      ) : null}
      {tools.length > 0 ? (
        <ContentsBlock title={plural(tools.length, "tool")}>
          <ul className="m-0 list-none p-0">
            {tools.map(tool => (
              <li key={tool.name} className={cn("border-t py-4", HAIRLINE)}>
                <p className="m-0 font-mono text-[14.5px] break-all text-fg">{tool.name}</p>
                <Muted className="mt-1">{tool.description}</Muted>
                <span className="mt-2 flex flex-wrap gap-1.5">
                  <Pill size="xs" form="hollow" tone={tool.readOnly ? "neutral" : "warning"}>
                    {tool.readOnly ? "read-only" : tool.risk}
                  </Pill>
                  <Pill size="xs" form="hollow">
                    {tool.visibility}-visible
                  </Pill>
                  {tool.concurrencySafe ? (
                    <Pill size="xs" form="hollow">
                      concurrency-safe
                    </Pill>
                  ) : null}
                </span>
              </li>
            ))}
          </ul>
        </ContentsBlock>
      ) : null}
    </DetailFrame>
  );
}
