/** The top of every page: where you are, what the page is, and what you can do with it. */
import { Pill, cn } from "@compozy/ui";
import { ArrowLeft, ArrowRight } from "@compozy/ui/icons";
import { ClaudeLogo, GithubLogo, OpenAILogo } from "@compozy/ui/logos";
import Link from "next/link";
import { Fragment, type ReactNode } from "react";

import { FOCUS, LINE } from "@/components/docs/docs-classes";
import { LLMCopyButton, type DocPageActionsContext } from "@/components/docs/page-actions";
import { maturityLabel, type MaturityLevel } from "@/lib/doc-maturity";
import { countNavPages, topFolderOf, type DocsPlace } from "@/lib/docs-shell";

const CRUMB = cn("rounded-[6px] text-muted-foreground transition-colors hover:text-fg", FOCUS);
const ACTION = cn(
  "inline-flex h-8 items-center gap-2 rounded-[10px] border border-site-light-100 px-3 text-[13px] text-fg/80 transition-colors hover:border-site-light-200 hover:text-fg",
  FOCUS
);

function Breadcrumb({ place }: { place: DocsPlace }) {
  const crumbs: { key: string; label: string; href: string | null }[] = [
    { key: "docs", label: "Docs", href: "/docs" },
  ];
  if (place.section) {
    crumbs.push({ key: place.section.id, label: place.section.label, href: place.section.href });
  }
  for (const node of place.trail) {
    crumbs.push({ key: node.key, label: node.name, href: node.url ?? null });
  }
  return (
    <nav
      aria-label="Breadcrumb"
      className="flex flex-wrap items-center gap-x-2 gap-y-1 text-[13.5px]"
    >
      {crumbs.map(crumb => (
        <Fragment key={crumb.key}>
          {crumb.href === null ? (
            <span className="text-muted-foreground">{crumb.label}</span>
          ) : (
            <Link href={crumb.href} className={CRUMB}>
              {crumb.label}
            </Link>
          )}
          <span aria-hidden className="text-subtle">
            /
          </span>
        </Fragment>
      ))}
      <span aria-current="page" className="text-fg">
        {place.node.children ? "Overview" : place.title}
      </span>
    </nav>
  );
}

function PageActions({ markdownUrl, pageUrl, githubUrl }: DocPageActionsContext) {
  const prompt = encodeURIComponent(`Read ${pageUrl}, I want to ask questions about it`);
  return (
    <div className="flex flex-wrap items-center gap-2">
      <LLMCopyButton markdownUrl={markdownUrl} />
      <a
        href={`https://chatgpt.com/?q=${prompt}`}
        target="_blank"
        rel="noopener noreferrer"
        className={ACTION}
      >
        <OpenAILogo className="size-3.5" /> Open in ChatGPT
      </a>
      <a
        href={`https://claude.ai/new?q=${prompt}`}
        target="_blank"
        rel="noopener noreferrer"
        className={ACTION}
      >
        <ClaudeLogo className="size-3.5" /> Open in Claude
      </a>
      <a
        href={githubUrl}
        target="_blank"
        rel="noopener noreferrer"
        aria-label="Open on GitHub"
        className={cn(ACTION, "px-2.5")}
      >
        <GithubLogo className="size-3.5" />
      </a>
    </div>
  );
}

export function DocsMasthead(props: {
  place: DocsPlace;
  /** What the page is, in a few words: reading time, or a count for generated references. */
  facts?: string;
  maturity?: MaturityLevel;
  actions: DocPageActionsContext;
  /** The collapsed "On this page" shown below `xl`, where the rail is hidden. */
  toc?: ReactNode;
}) {
  const { place } = props;
  const top = topFolderOf(place);
  return (
    <header className={cn("border-b pb-8", LINE)}>
      <Breadcrumb place={place} />
      <h1 className="mt-6 font-display text-[38px]/[1.08] font-normal tracking-[-0.03em] text-balance text-fg md:text-[50px]/[1.05]">
        {place.title}
      </h1>
      {place.description ? (
        <p className="mt-4 max-w-[680px] text-[18px]/[1.55] text-pretty text-muted-foreground md:text-[19px]/[1.55]">
          {place.description}
        </p>
      ) : null}
      <div className="mt-7 flex flex-wrap items-center justify-between gap-x-6 gap-y-4">
        <p className="m-0 flex flex-wrap items-center gap-x-4 gap-y-2 text-[13.5px] text-subtle">
          {props.maturity ? (
            <Pill tone={props.maturity === "alpha" ? "warning" : "success"} size="sm">
              {maturityLabel(props.maturity)}
            </Pill>
          ) : null}
          {props.facts ? <span>{props.facts}</span> : null}
          {top ? (
            <span>
              {countNavPages(top)} pages in {top.name}
            </span>
          ) : null}
        </p>
        <PageActions {...props.actions} />
      </div>
      {props.toc}
    </header>
  );
}

function PagerLink({ place, next }: { place: DocsPlace; next: boolean }) {
  return (
    <Link
      href={place.url}
      className={cn(
        "group flex flex-col gap-2 rounded-[16px] border border-site-light-090 bg-transparent p-5 transition-colors hover:border-site-light-200 hover:bg-site-light-025",
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
      <span className="text-[16.5px]/[1.3] text-fg">{place.title}</span>
      <span className="text-[13px] text-muted-foreground">
        {topFolderOf(place)?.name ?? place.section?.label ?? "Docs"}
      </span>
    </Link>
  );
}

export function DocsPager({
  previous,
  next,
}: {
  previous: DocsPlace | null;
  next: DocsPlace | null;
}) {
  return (
    <nav
      aria-label="Pagination"
      className={cn("mt-16 grid gap-4 border-t pt-8 sm:grid-cols-2", LINE)}
    >
      {previous ? <PagerLink place={previous} next={false} /> : null}
      {next ? <PagerLink place={next} next /> : null}
    </nav>
  );
}
