/** The docs front door: search first, then the three paths in, then the map of every section. */
import { Kbd, cn } from "@compozy/ui";
import { ArrowRight, Search } from "@compozy/ui/icons";
import { m, useReducedMotion, useScroll } from "motion/react";
import { use, useRef } from "react";

import { INSTALL_COMMAND } from "../landing/content";
import { InstallCommand } from "../landing/primitives";
import { DocBlocks } from "./blocks";
import { DocsGlyph } from "./glyph";
import type { Block, Doc, PathCardItem } from "./mdx";
import { GROUPS, TOTAL_PAGES, findPage, firstSlug, slugFromHref, type DocPage } from "./nav";
import { DocsNavContext } from "./nav-context";
import { DocsProse } from "./prose";
import { BEAM, FOCUS, LINE } from "./text";

const SHORTCUTS = ["getting-started/installation", "getting-started/quick-start", "cli", "api"].map(
  findPage
);
const WIDE = "mx-auto w-full max-w-[1280px] px-5 md:px-10";
/** The first three commands of the Quick Start page. */
const FIRST_RUN = [
  "compozy daemon start",
  'compozy workspace add "$PWD" --name current',
  "compozy session new --workspace current --agent general",
];
const QUICK_START = findPage("getting-started/quick-start");

function FirstRun() {
  const open = use(DocsNavContext);
  return (
    <div className="w-full max-w-[460px] overflow-hidden rounded-[18px] border border-[color:var(--light-100)] bg-[#0d0d0d] shadow-[0_30px_80px_rgba(0,0,0,0.5)]">
      <div className={cn("flex h-11 items-center justify-between border-b px-4", LINE)}>
        <span className="font-mono text-[12.5px] text-subtle">{QUICK_START.title}</span>
        <button
          type="button"
          onClick={() => open(QUICK_START.slug)}
          className={cn(
            "group inline-flex cursor-pointer items-center gap-1.5 rounded-[6px] bg-transparent text-[13px] text-fg/80 transition-colors hover:text-fg",
            FOCUS
          )}
        >
          Open
          <ArrowRight
            className="size-3.5 transition-transform group-hover:translate-x-0.5"
            aria-hidden
          />
        </button>
      </div>
      <ol className="m-0 list-none px-4 py-3 font-mono text-[13px]/[1.6]">
        {FIRST_RUN.map((command, order) => (
          <li key={command} className="flex gap-3 py-2">
            <span className="text-subtle tabular-nums select-none">{order + 1}</span>
            <span aria-hidden className="text-brand-strong select-none">
              $
            </span>
            <code className="min-w-0 break-words text-fg">{command}</code>
          </li>
        ))}
      </ol>
    </div>
  );
}

function HomeHero({ page, onSearch }: { page: DocPage; onSearch: () => void }) {
  const open = use(DocsNavContext);
  return (
    <section className={cn("relative border-b", LINE)}>
      <div
        className={cn(
          WIDE,
          "grid items-end gap-x-12 gap-y-12 pt-16 pb-16 md:pt-24 md:pb-20 xl:grid-cols-[minmax(0,1fr)_460px]"
        )}
      >
        <div className="min-w-0">
          <p className="m-0 font-mono text-[13px] text-subtle">
            {TOTAL_PAGES} pages · {GROUPS.length} sections
          </p>
          <h1 className="mt-5 max-w-[900px] font-display text-[44px]/[1.02] font-normal tracking-[-0.035em] text-balance text-fg md:text-[76px]/[1]">
            {page.title}
          </h1>
          <p className="mt-6 max-w-[640px] text-[19px]/[1.5] text-pretty text-muted-foreground md:text-[21px]/[1.5]">
            {page.description}
          </p>
          <div className="mt-10 flex flex-wrap items-center gap-3">
            <button
              type="button"
              onClick={onSearch}
              className={cn(
                "flex h-[52px] w-full max-w-[420px] cursor-pointer items-center gap-3 rounded-pill border border-[color:var(--light-150)] bg-[color:var(--light-050)] pr-4 pl-5 text-left text-[15.5px] text-muted-foreground transition-colors hover:border-[color:var(--light-250)] hover:text-fg",
                FOCUS
              )}
            >
              <Search className="size-[18px] shrink-0" aria-hidden />
              <span className="flex-1">Search the docs</span>
              <Kbd>⌘K</Kbd>
            </button>
            <InstallCommand command={INSTALL_COMMAND} />
          </div>
          <ul className="mt-8 flex list-none flex-wrap gap-x-7 gap-y-2 p-0">
            {SHORTCUTS.map(shortcut => (
              <li key={shortcut.slug}>
                <button
                  type="button"
                  onClick={() => open(shortcut.slug)}
                  className={cn(
                    "group inline-flex cursor-pointer items-center gap-1.5 rounded-[6px] bg-transparent text-[14.5px] text-fg/80 transition-colors hover:text-fg",
                    FOCUS
                  )}
                >
                  {shortcut.node.c ? shortcut.node.t : shortcut.title}
                  <ArrowRight
                    className="size-3.5 text-subtle transition-transform group-hover:translate-x-0.5"
                    aria-hidden
                  />
                </button>
              </li>
            ))}
          </ul>
        </div>
        <FirstRun />
      </div>
      <span
        aria-hidden
        className="absolute inset-x-0 -bottom-px h-px bg-[linear-gradient(90deg,transparent,#1f87fc_35%,#1df0d7_65%,transparent)] opacity-70"
      />
    </section>
  );
}

function PathCards({ cards }: { cards: readonly PathCardItem[] }) {
  const open = use(DocsNavContext);
  return (
    <div className="my-10 grid gap-5 lg:grid-cols-3">
      {cards.map(card => (
        <div
          key={card.title}
          className="flex flex-col rounded-[20px] border border-[color:var(--light-090)] bg-[color:var(--light-025)] p-6 md:p-7"
        >
          <p className="m-0 font-mono text-[12.5px] text-brand-strong">{card.label}</p>
          <h3 className="mt-4 font-display text-[25px]/[1.15] font-normal tracking-[-0.02em] text-fg">
            {card.title}
          </h3>
          <p className="mt-3 flex-1 text-[14.5px]/[1.6] text-muted-foreground">
            {card.description}
          </p>
          <ul className={cn("mt-6 list-none border-t p-0 pt-2", LINE)}>
            {card.links.map(link => {
              return (
                <li key={link.href}>
                  <button
                    type="button"
                    onClick={() => open(slugFromHref(link.href) ?? "")}
                    className={cn(
                      "group flex w-full cursor-pointer items-center gap-3 rounded-[8px] bg-transparent py-2.5 text-left text-[14.5px] transition-colors hover:text-fg",
                      link.cta ? "text-fg" : "text-fg/70",
                      FOCUS
                    )}
                  >
                    <DocsGlyph
                      name={link.icon}
                      className={cn(
                        "size-4 shrink-0",
                        link.cta ? "text-brand-strong" : "text-subtle"
                      )}
                    />
                    {link.text}
                    <ArrowRight
                      className="ml-auto size-3.5 text-subtle transition-transform group-hover:translate-x-0.5 group-hover:text-fg"
                      aria-hidden
                    />
                  </button>
                </li>
              );
            })}
          </ul>
        </div>
      ))}
    </div>
  );
}

function SectionMap({ rows }: { rows: readonly { name: string; text: string }[] }) {
  const open = use(DocsNavContext);
  const rail = useRef<HTMLOListElement>(null);
  const reduced = useReducedMotion();
  const { scrollYProgress } = useScroll({ target: rail, offset: ["start 70%", "end 55%"] });
  return (
    <ol ref={rail} className="relative my-10 list-none p-0">
      <span
        aria-hidden
        className="absolute top-2 bottom-2 left-[13px] w-px bg-[color:var(--light-120)]"
      >
        <m.span
          style={{ scaleY: reduced ? 1 : scrollYProgress }}
          className={cn("absolute inset-y-0 -left-px w-0.5 origin-top", BEAM)}
        />
      </span>
      {rows.map((row, order) => {
        const group = GROUPS.find(candidate => candidate.label === row.name);
        return (
          <li
            key={row.name}
            className={cn(
              "relative grid gap-x-8 gap-y-2 border-b py-6 pl-12 last:border-b-0 md:grid-cols-[230px_minmax(0,1fr)_auto]",
              LINE
            )}
          >
            <span className="absolute top-6 left-0 inline-flex size-7 items-center justify-center rounded-full border border-[color:var(--light-200)] bg-[#0a0a0a] font-mono text-[11.5px] text-brand-strong tabular-nums">
              {order + 1}
            </span>
            <button
              type="button"
              onClick={() => open(group ? firstSlug(group) : "")}
              className={cn(
                "group inline-flex cursor-pointer items-center gap-2 self-start rounded-[6px] bg-transparent text-left font-display text-[22px]/[1.25] tracking-[-0.02em] text-fg",
                FOCUS
              )}
            >
              {row.name}
              <ArrowRight
                className="size-4 text-subtle transition-transform group-hover:translate-x-1 group-hover:text-fg"
                aria-hidden
              />
            </button>
            <DocsProse text={row.text} className="text-[15px] leading-[1.65] [&_p]:my-0" />
            <span className="self-start pt-1 font-mono text-[12.5px] text-subtle tabular-nums md:text-right">
              {group?.count ?? 0} pages
            </span>
          </li>
        );
      })}
    </ol>
  );
}

function HomeBlock({ block }: { block: Block }) {
  if (block.kind === "paths") return <PathCards cards={block.cards} />;
  if (block.kind === "groups") return <SectionMap rows={block.rows} />;
  return (
    <div className="max-w-[780px]">
      <DocBlocks blocks={[block]} />
    </div>
  );
}

export function DocsHome({
  page,
  doc,
  onSearch,
}: {
  page: DocPage;
  doc: Doc | null;
  onSearch: () => void;
}) {
  return (
    <>
      <HomeHero page={page} onSearch={onSearch} />
      <div className={cn(WIDE, "pt-14 pb-24")}>
        {(doc?.blocks ?? []).map(block => (
          <HomeBlock key={block.id} block={block} />
        ))}
      </div>
    </>
  );
}
