/** One entry, from feed fields only: the public site has no daemon, so no install state. */
import { Pill, cn } from "@compozy/ui";
import { ArrowLeft, ArrowUpRight, Download, ShieldCheck } from "@compozy/ui/icons";

import { CONTAINER, HAIRLINE } from "../landing/layout";
import { InstallCommand, H1, H3, Muted, PrimaryCta, Small } from "../landing/primitives";
import { categoryLabel, inCategory, type Entry } from "./catalog";
import { EntryCard, EntryLogo } from "./entry-card";

function Row({ term, children }: { term: string; children: React.ReactNode }) {
  return (
    <div className={cn("flex items-baseline justify-between gap-6 border-t py-3", HAIRLINE)}>
      <dt className="text-[14px] text-subtle">{term}</dt>
      <dd className="m-0 min-w-0 truncate text-right text-[14px] text-fg">{children}</dd>
    </div>
  );
}

export function EntryDetail({
  entry,
  onBack,
  onCategory,
  onOpen,
}: {
  entry: Entry;
  onBack: () => void;
  onCategory: (id: string) => void;
  onOpen: (id: string) => void;
}) {
  const related = inCategory(entry.category)
    .filter(item => item.id !== entry.id)
    .slice(0, 6);
  return (
    <div className={cn(CONTAINER, "pt-[120px] pb-28 md:pt-[136px]")}>
      <button
        type="button"
        onClick={onBack}
        className="inline-flex cursor-pointer items-center gap-2 bg-transparent p-0 text-[14px] text-muted-foreground outline-none transition-colors hover:text-fg focus-visible:text-fg"
      >
        <ArrowLeft aria-hidden className="size-4" /> Marketplace
      </button>
      <div className="mt-10 grid gap-12 lg:grid-cols-[minmax(0,1fr)_380px] lg:gap-16">
        <div className="min-w-0">
          <EntryLogo entry={entry} size="xl" />
          <div className="mt-7 [&_h1]:md:text-[64px] [&_h1]:md:leading-[1.03]">
            <H1>{entry.name}</H1>
          </div>
          <p className="mt-4 mb-0 flex flex-wrap items-center gap-3 text-[15px] text-muted-foreground">
            {entry.author ? <span>By {entry.author}</span> : null}
            <button
              type="button"
              onClick={() => onCategory(entry.category)}
              className="cursor-pointer bg-transparent p-0"
            >
              <Pill size="sm" form="hollow">
                {categoryLabel(entry.category)}
              </Pill>
            </button>
          </p>
          <p className="mt-9 mb-0 max-w-[68ch] text-[18px]/[1.65] text-pretty text-fg/85">
            {entry.description}
          </p>
        </div>
        <aside className="lg:sticky lg:top-24 lg:self-start">
          <div className="rounded-[20px] border border-[color:var(--light-100)] bg-[color:var(--light-030)] p-6">
            <H3 className="md:text-[22px]">Add it from the app</H3>
            <Muted className="mt-2">
              Open Marketplace in Compozy and add {entry.name} from there, or find it from the
              terminal.
            </Muted>
            <InstallCommand
              command={`compozy marketplace search ${entry.id}`}
              className="mt-5 flex w-full justify-between h-auto min-h-[52px] rounded-[16px] py-2 [&_code]:[overflow-wrap:anywhere] [&_code]:whitespace-normal"
            />
            <div className="mt-4 [&_button]:w-full">
              <PrimaryCta icon={Download}>Download Compozy</PrimaryCta>
            </div>
            <dl className="m-0 mt-6">
              <Row term="Type">{entry.kind === "extension" ? "Extension" : "Plugin"}</Row>
              <Row term="Trust">{entry.tier.slice(0, 1).toUpperCase() + entry.tier.slice(1)}</Row>
              <Row term="From">{entry.source}</Row>
              {entry.homepage ? (
                <Row term="Homepage">
                  <a
                    href={entry.homepage}
                    className="inline-flex items-center gap-1 text-fg underline-offset-4 hover:underline"
                  >
                    {new URL(entry.homepage).hostname.replace(/^www\./, "")}{" "}
                    <ArrowUpRight aria-hidden className="size-3.5" />
                  </a>
                </Row>
              ) : null}
            </dl>
            <Small className="mt-4 flex gap-2">
              <ShieldCheck aria-hidden className="mt-0.5 size-3.5 shrink-0" />
              The app shows what it contains and asks you to confirm before it installs.
            </Small>
          </div>
        </aside>
      </div>
      {related.length ? (
        <div className={cn("mt-24 border-t pt-14", HAIRLINE)}>
          <H3>More in {categoryLabel(entry.category)}</H3>
          <div className="mt-8 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {related.map(item => (
              <EntryCard key={item.id} entry={item} onOpen={onOpen} />
            ))}
          </div>
        </div>
      ) : null}
    </div>
  );
}
