/** The top of every page: where you are, what the page is, and what you can do with it. */
import { Button, Pill, cn } from "@compozy/ui";
import { ArrowLeft, ArrowRight, Check, Copy } from "@compozy/ui/icons";
import { ClaudeLogo, GithubLogo, OpenAILogo } from "@compozy/ui/logos";
import { Fragment, use, type ReactNode } from "react";

import { useCopied } from "./hooks/use-copied";
import { countPages, firstSlug, neighbours, sectionOf, type DocPage } from "./nav";
import { DocsNavContext } from "./nav-context";
import { FOCUS, LINE } from "./text";

const CRUMB = cn(
  "cursor-pointer rounded-[6px] bg-transparent text-muted-foreground transition-colors hover:text-fg",
  FOCUS
);
const ACTION =
  "inline-flex h-8 items-center gap-2 rounded-[10px] border border-[color:var(--light-100)] px-3 text-[13px] text-fg/80 transition-colors hover:border-[color:var(--light-200)] hover:text-fg";

function Breadcrumb({ page }: { page: DocPage }) {
  const open = use(DocsNavContext);
  const crumbs: { key: string; label: string; slug: string | null }[] = [
    { key: "docs", label: "Docs", slug: "" },
  ];
  if (page.group) {
    crumbs.push({ key: page.group.id, label: page.group.label, slug: firstSlug(page.group) });
  }
  for (const node of page.trail) {
    crumbs.push({ key: node.s ?? node.t, label: node.t, slug: node.s ?? null });
  }
  return (
    <nav
      aria-label="Breadcrumb"
      className="flex flex-wrap items-center gap-x-2 gap-y-1 text-[13.5px]"
    >
      {crumbs.map(crumb => (
        <Fragment key={crumb.key}>
          {crumb.slug === null ? (
            <span className="text-muted-foreground">{crumb.label}</span>
          ) : (
            <button type="button" className={CRUMB} onClick={() => open(crumb.slug ?? "")}>
              {crumb.label}
            </button>
          )}
          <span aria-hidden className="text-subtle">
            /
          </span>
        </Fragment>
      ))}
      <span className="text-fg">{page.node.c ? "Overview" : page.title}</span>
    </nav>
  );
}

function PageActions({ source }: { source: string }) {
  const { copied, copy } = useCopied();
  return (
    <div className="flex flex-wrap items-center gap-2">
      {source ? (
        <Button
          variant="outline"
          size="sm"
          className="h-8 gap-2 rounded-[10px] px-3 text-[13px]"
          onClick={() => copy(source)}
        >
          {copied ? <Check className="text-brand-strong" /> : <Copy />}
          {copied ? "Copied" : "Copy as Markdown"}
        </Button>
      ) : null}
      <a href="#" className={ACTION}>
        <OpenAILogo className="size-3.5" /> Open in ChatGPT
      </a>
      <a href="#" className={ACTION}>
        <ClaudeLogo className="size-3.5" /> Open in Claude
      </a>
      <a href="#" aria-label="Open on GitHub" className={cn(ACTION, "px-2.5")}>
        <GithubLogo className="size-3.5" />
      </a>
    </div>
  );
}

export function DocMasthead(props: {
  page: DocPage;
  source: string;
  /** What the page is, in a few words: reading time, or a count for generated references. */
  facts?: string;
  title?: ReactNode;
}) {
  const { page } = props;
  const section = sectionOf(page);
  return (
    <header className={cn("border-b pb-8", LINE)}>
      <Breadcrumb page={page} />
      <h1 className="mt-6 font-display text-[38px]/[1.08] font-normal tracking-[-0.03em] text-balance text-fg md:text-[50px]/[1.05]">
        {props.title ?? page.title}
      </h1>
      {page.description ? (
        <p className="mt-4 max-w-[680px] text-[18px]/[1.55] text-pretty text-muted-foreground md:text-[19px]/[1.55]">
          {page.description}
        </p>
      ) : null}
      <div className="mt-7 flex flex-wrap items-center justify-between gap-x-6 gap-y-4">
        <p className="m-0 flex flex-wrap items-center gap-x-4 gap-y-2 text-[13.5px] text-subtle">
          {page.maturity ? (
            <Pill tone="success" size="sm">
              {page.maturity}
            </Pill>
          ) : null}
          {props.facts ? <span>{props.facts}</span> : null}
          {section ? (
            <span>
              {countPages(section)} pages in {section.t}
            </span>
          ) : null}
        </p>
        <PageActions source={props.source} />
      </div>
    </header>
  );
}

function PagerLink({ page, next }: { page: DocPage; next: boolean }) {
  const open = use(DocsNavContext);
  return (
    <button
      type="button"
      onClick={() => open(page.slug)}
      className={cn(
        "group flex cursor-pointer flex-col gap-2 rounded-[16px] border border-[color:var(--light-090)] bg-transparent p-5 transition-colors hover:border-[color:var(--light-200)] hover:bg-[color:var(--light-025)]",
        next ? "items-end text-right sm:col-start-2" : "items-start text-left",
        FOCUS
      )}
    >
      <span className="flex items-center gap-2 text-[13px] text-subtle">
        {next ? null : (
          <ArrowLeft
            className="size-3.5 transition-transform group-hover:-translate-x-0.5"
            aria-hidden
          />
        )}
        {next ? "Next" : "Previous"}
        {next ? (
          <ArrowRight
            className="size-3.5 transition-transform group-hover:translate-x-0.5"
            aria-hidden
          />
        ) : null}
      </span>
      <span className="text-[16.5px]/[1.3] text-fg">{page.title}</span>
      <span className="text-[13px] text-muted-foreground">
        {sectionOf(page)?.t ?? page.group?.label ?? "Docs"}
      </span>
    </button>
  );
}

export function DocPager({ slug }: { slug: string }) {
  const { previous, next } = neighbours(slug);
  return (
    <nav
      aria-label="Pagination"
      className={cn("mt-16 grid gap-4 border-t pt-8 sm:grid-cols-2", LINE)}
    >
      {previous ? <PagerLink page={previous} next={false} /> : null}
      {next ? <PagerLink page={next} next /> : null}
    </nav>
  );
}
