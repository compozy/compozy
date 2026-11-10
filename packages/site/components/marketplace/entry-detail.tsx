/** One catalog entry, from feed fields only: the public site has no daemon, so no install state. */
import { Pill, cn } from "@compozy/ui";
import { Lock } from "@compozy/ui/icons";
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
import {
  installCommand,
  marketplaceSearchCommand,
  type ExtensionEntry,
} from "@/lib/marketplace-catalog";
import {
  CATALOG_SOURCE,
  categoryLabel,
  relatedCards,
  tierLabel,
  toCard,
} from "@/lib/marketplace-view";

type EntryInput = NonNullable<ExtensionEntry["inputs"]>[number];

const INPUT_TYPES: Record<EntryInput["type"], string> = {
  string: "Text",
  identifier: "Identifier",
  boolean: "Boolean",
  secret: "Secret",
};

function defaultLabel(input: EntryInput): string | null {
  if (input.default === undefined) return null;
  return typeof input.default === "boolean" ? String(input.default) : input.default;
}

/** What the package asks for at install time; the aside carries the install command. */
function Inputs({ inputs }: { inputs: readonly EntryInput[] }) {
  return (
    <ContentsBlock title="Inputs">
      <ul className="m-0 list-none p-0">
        {inputs.map(input => {
          const fallback = defaultLabel(input);
          return (
            <li
              key={input.id}
              className={cn("flex items-baseline justify-between gap-6 border-t py-3", HAIRLINE)}
            >
              <div className="min-w-0">
                <p className="m-0 text-[15px] text-fg">{input.prompt}</p>
                <code className="mt-0.5 block font-mono text-[12.5px] break-all text-subtle">
                  {input.id}
                  {input.binding.type === "env"
                    ? ` · env ${input.binding.name}`
                    : ` · ?${input.binding.name}=`}
                </code>
              </div>
              <div className="flex shrink-0 flex-wrap items-center justify-end gap-2 text-[14px] text-muted-foreground">
                {input.type === "secret" ? (
                  <Pill size="sm" form="hollow">
                    <Lock aria-hidden className="size-3" />
                    Secret
                  </Pill>
                ) : (
                  <span>{INPUT_TYPES[input.type]}</span>
                )}
                <span>{input.required ? "Required" : "Optional"}</span>
                {fallback !== null ? (
                  <span>
                    Default <code className="font-mono text-fg">{fallback}</code>
                  </span>
                ) : null}
              </div>
            </li>
          );
        })}
      </ul>
      <Muted className={cn("border-t pt-4", HAIRLINE)}>
        Pass values at install time with repeatable <code>--input id=value</code> flags, or use{" "}
        <code>--input-file inputs.json</code> so secrets never sit on the command line. See{" "}
        <Link
          href="/docs/extensions/install#supply-package-inputs"
          className="text-fg underline-offset-4 hover:underline"
        >
          Supply package inputs
        </Link>
        .
      </Muted>
    </ContentsBlock>
  );
}

export function EntryDetail({ entry }: { entry: ExtensionEntry }) {
  const card = toCard(entry);
  const inputs = entry.inputs ?? [];
  return (
    <DetailFrame
      logo={<EntryLogo entry={card} size="xl" />}
      name={entry.name}
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
      description={entry.description}
      aside={
        <InstallAside
          title="Add it from the app"
          lead={
            <Muted className="mt-2">
              Open Marketplace in CompozyOS and add {entry.name} from there, or find it from the
              terminal.
            </Muted>
          }
          command={
            <>
              <DetailCommand command={marketplaceSearchCommand(entry)} />
              {/* Every catalog extension installs from the terminal, with or without inputs. */}
              <DetailCommand command={installCommand(entry)} />
            </>
          }
          facts={
            <>
              <Row term="Type">Extension</Row>
              <Row term="Trust">{tierLabel(entry.tier)}</Row>
              <Row term="From">{CATALOG_SOURCE}</Row>
              <Row term="Version">{entry.version}</Row>
              {entry.repository ? (
                <Row term="Homepage">
                  <HomepageLink href={entry.repository} />
                </Row>
              ) : null}
            </>
          }
          note="The app shows what it contains and asks you to confirm before it installs."
        />
      }
      relatedTitle={`More in ${categoryLabel(card.category)}`}
      related={relatedCards(card)}
    >
      {inputs.length > 0 ? <Inputs inputs={inputs} /> : null}
    </DetailFrame>
  );
}
