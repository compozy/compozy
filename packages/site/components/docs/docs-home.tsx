/** The docs front door: search first, then the three paths in, then the map of every section. */
import { cn } from "@compozy/ui";
import { ArrowRight } from "@compozy/ui/icons";
import Link from "next/link";
import type { ReactNode } from "react";

import { DOCS_PROSE, FOCUS, LINE } from "@/components/docs/docs-classes";
import { DocsHomeSearch } from "@/components/docs/docs-search-trigger";
import { InstallCommand } from "@/components/site/primitives";
import type { DocsShell } from "@/lib/docs-shell";
import { VERIFIED_INSTALLER_COMMAND } from "@/lib/release/install-commands";

const SHORTCUTS = [
  "/docs/getting-started/installation",
  "/docs/getting-started/quick-start",
  "/docs/cli",
  "/docs/api",
];
const QUICK_START = "/docs/getting-started/quick-start";
const WIDE = "mx-auto w-full max-w-[1280px] px-5 md:px-10";
/** The first three commands of the Quick Start page. */
const FIRST_RUN = [
  "compozy daemon start",
  'compozy workspace add "$PWD" --name current',
  "compozy session new --workspace current --agent general",
];

function FirstRun({ title }: { title: string }) {
  return (
    <div className="w-full max-w-[460px] overflow-hidden rounded-[18px] border border-site-light-100 bg-[#0d0d0d] shadow-[0_30px_80px_var(--site-dark-550)]">
      <div className={cn("flex h-11 items-center justify-between border-b px-4", LINE)}>
        <span className="font-mono text-[12.5px] text-subtle">{title}</span>
        <Link
          href={QUICK_START}
          className={cn(
            "group inline-flex items-center gap-1.5 rounded-[6px] text-[13px] text-fg/80 transition-colors hover:text-fg",
            FOCUS
          )}
        >
          Open
          <ArrowRight
            className="size-3.5 transition-transform group-hover:translate-x-0.5"
            aria-hidden
          />
        </Link>
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

function HomeHero({ shell }: { shell: DocsShell }) {
  const home = shell.byUrl.get("/docs");
  const shortcuts = SHORTCUTS.flatMap(url => {
    const place = shell.byUrl.get(url);
    return place ? [{ url, label: place.node.children ? place.node.name : place.title }] : [];
  });
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
            {shell.total} pages · {shell.tabs.length} sections
          </p>
          <h1 className="mt-5 max-w-[900px] font-display text-[44px]/[1.02] font-normal tracking-[-0.035em] text-balance text-fg md:text-[76px]/[1]">
            {home?.title}
          </h1>
          <p className="mt-6 max-w-[640px] text-[19px]/[1.5] text-pretty text-muted-foreground md:text-[21px]/[1.5]">
            {home?.description}
          </p>
          <div className="mt-10 flex flex-wrap items-center gap-3">
            <DocsHomeSearch />
            <InstallCommand command={VERIFIED_INSTALLER_COMMAND} />
          </div>
          <ul className="mt-8 flex list-none flex-wrap gap-x-7 gap-y-2 p-0">
            {shortcuts.map(shortcut => (
              <li key={shortcut.url}>
                <Link
                  href={shortcut.url}
                  className={cn(
                    "group inline-flex items-center gap-1.5 rounded-[6px] text-[14.5px] text-fg/80 transition-colors hover:text-fg",
                    FOCUS
                  )}
                >
                  {shortcut.label}
                  <ArrowRight
                    className="size-3.5 text-subtle transition-transform group-hover:translate-x-0.5"
                    aria-hidden
                  />
                </Link>
              </li>
            ))}
          </ul>
        </div>
        <FirstRun title={shell.byUrl.get(QUICK_START)?.title ?? "Quick Start"} />
      </div>
      <span
        aria-hidden
        className="absolute inset-x-0 -bottom-px h-px bg-[linear-gradient(90deg,transparent,#1f87fc_35%,#1df0d7_65%,transparent)] opacity-70"
      />
    </section>
  );
}

/** The docs home: the hero, then the page's own MDX (paths, section map and prose). */
export function DocsHome({ shell, children }: { shell: DocsShell; children: ReactNode }) {
  return (
    <>
      <HomeHero shell={shell} />
      <div className={cn(WIDE, "pt-14 pb-24")}>
        <div className={cn(DOCS_PROSE, "docs-home [&>:not(.docs-wide)]:max-w-[780px]")}>
          {children}
        </div>
      </div>
    </>
  );
}
