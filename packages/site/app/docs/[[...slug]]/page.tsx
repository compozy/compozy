import type { Metadata } from "next";
import type { ReactNode } from "react";
import { notFound } from "next/navigation";

import { CliBody } from "@/components/docs/cli-page";
import { DocsDrawer } from "@/components/docs/docs-chrome";
import { BELOW_HEADER, DOCS_PROSE, LINE, textOf } from "@/components/docs/docs-classes";
import { DocsHome } from "@/components/docs/docs-home";
import { DocsMasthead, DocsPager } from "@/components/docs/docs-masthead";
import { DocsSidebar, type DocsSidebarGroup } from "@/components/docs/docs-sidebar";
import { DocsPageFrame, DocsTocDisclosure, type DocsTocItem } from "@/components/docs/docs-toc";
import {
  BreadcrumbListJsonLd,
  TechArticleJsonLd,
  type BreadcrumbItem,
} from "@/components/seo/structured-data";
import { apiOperationCount } from "@/lib/api-reference";
import { cliToc, readCliDoc, type CliDoc } from "@/lib/cli-doc";
import { getDocsPageSource } from "@/lib/get-llm-text";
import { getDocsShell, neighboursOf, type DocsShell } from "@/lib/docs-shell";
import { normalizeDocsUrl } from "@/lib/docs-navigation";
import { openapi } from "@/lib/openapi";
import { isSdkUrl, sdkFacts, sdkResourceForUrl } from "@/lib/sdk-reference";
import { SDK_PROSE } from "@/components/sdk/sdk-classes";
import { absoluteUrl, canonicalPath, createPageMetadata, docsSourceUrl } from "@/lib/site-config";
import { docsSource } from "@/lib/source";
import { cn } from "@compozy/ui";
import { getMDXComponents } from "@/mdx-components";

interface PageProps {
  params: Promise<{ slug?: string[] }>;
}

const WORDS_PER_MINUTE = 220;

function buildActionUrls(pageUrl: string, slug: string[], relativePath: string | undefined) {
  const slugSegment = slug.length ? `${slug.join("/")}/` : "";
  const markdownUrl = `/llms.mdx/docs/${slugSegment}`;
  const canonicalPageUrl = absoluteUrl(canonicalPath(pageUrl));
  const githubUrl = docsSourceUrl(relativePath ?? "");
  return { markdownUrl, pageUrl: canonicalPageUrl, githubUrl };
}

function humanize(segment: string): string {
  const words: string[] = [];
  for (const part of segment.split("-")) {
    if (part.length > 0) {
      words.push(part[0].toUpperCase() + part.slice(1));
    }
  }
  return words.join(" ");
}

function buildBreadcrumbs(slug: string[], pageTitle: string): BreadcrumbItem[] {
  const items: BreadcrumbItem[] = [
    { name: "Home", path: "/" },
    { name: "Docs", path: "/docs/" },
  ];
  let cumulative = "/docs";
  for (let i = 0; i < slug.length; i += 1) {
    cumulative += `/${slug[i]}`;
    const isLast = i === slug.length - 1;
    items.push({ name: isLast ? pageTitle : humanize(slug[i]), path: `${cumulative}/` });
  }
  return items;
}

function resolvedDocSlug(pageUrl: string): string[] {
  return pageUrl.split("/").filter(Boolean).slice(1);
}

function readingMinutes(content: string): number {
  const words = content.split(/\s+/).filter(Boolean).length;
  return Math.max(1, Math.round(words / WORDS_PER_MINUTE));
}

function tocIdOf(url: string): string {
  return url.startsWith("#") ? url.slice(1) : (url.split("#")[1] ?? url);
}

/** The sidebar shows the page's section; pages outside every section see all of them. */
function sidebarGroups(shell: DocsShell, sectionId: string | null): DocsSidebarGroup[] {
  const sections = sectionId
    ? shell.sections.filter(section => section.id === sectionId)
    : shell.sections;
  return sections.map(({ id, label, count, nodes }) => ({ id, label, count, nodes }));
}

/** Which template a page renders through: generated references get their own layouts. */
type DocTemplate =
  | { kind: "cli"; cli: CliDoc }
  | { kind: "api"; endpoints: number }
  | { kind: "sdk" }
  | { kind: "article" };

function templateOf(
  slug: string[],
  url: string,
  content: string,
  description: string
): DocTemplate {
  if (sdkResourceForUrl(url) !== undefined) return { kind: "sdk" };
  // Generated CLI pages are parsed once here, at build, into the command template.
  const cli = slug[0] === "cli" ? readCliDoc(slug.join("/"), content, description) : null;
  if (cli) return { kind: "cli", cli };
  // Generated API resource pages (not the hand-written API overview) use the resource template.
  const endpoints = slug[0] === "api" ? apiOperationCount(content) : 0;
  return endpoints > 0 ? { kind: "api", endpoints } : { kind: "article" };
}

type PageToc = { depth: number; url: string; title: ReactNode }[];

/** API and SDK resource pages have no rail: their endpoint index plays that part, as in the draft. */
function tocOf(template: DocTemplate, pageToc: PageToc): DocsTocItem[] {
  if (template.kind === "cli") return cliToc(template.cli);
  if (template.kind === "api" || template.kind === "sdk") return [];
  return pageToc.flatMap(item =>
    item.depth === 2 || item.depth === 3
      ? [{ id: tocIdOf(item.url), title: textOf(item.title), depth: item.depth }]
      : []
  );
}

/** The masthead facts the draft shows: endpoints for API pages, method counts for SDK pages, nothing for CLI commands. */
function factsOf(template: DocTemplate, url: string, content: string): string | undefined {
  if (template.kind === "cli") return undefined;
  if (template.kind === "api") return `${template.endpoints} endpoints`;
  return sdkFacts(url) ?? `${readingMinutes(content)} min read`;
}

/** Layout per template: reference pages run full width and keep their chrome at reading width. */
function layoutOf(template: DocTemplate, url: string) {
  const reference = template.kind === "api" || template.kind === "sdk";
  let bodyClass = cn("pt-9", DOCS_PROSE, isSdkUrl(url) && SDK_PROSE);
  if (template.kind === "api") bodyClass = "pt-0";
  if (template.kind === "sdk") bodyClass = DOCS_PROSE;
  return {
    wide: reference,
    chromeClass: reference ? "max-w-[860px]" : undefined,
    bodyClass,
    // API pages have no "On this page" disclosure: their endpoint index plays that part.
    disclosure: template.kind !== "api",
  };
}

/** CLI commands title in mono, as the draft does. */
function mastheadTitle(template: DocTemplate, title: string): ReactNode {
  if (template.kind !== "cli") return null;
  return <span className="docs-count font-mono text-[0.72em]">{title}</span>;
}

export default async function Page(props: PageProps) {
  const params = await props.params;
  const slug = params.slug ?? [];
  const page = docsSource.getPage(slug);
  if (!page) notFound();

  const shell = getDocsShell();
  const url = normalizeDocsUrl(page.url);
  const place = shell.byUrl.get(url);
  const MDX = page.data.body;
  const resolvedSlug = resolvedDocSlug(page.url);
  const breadcrumbs = buildBreadcrumbs(resolvedSlug, page.data.title);
  const ogImagePath = `/og/docs/${resolvedSlug.length ? `${resolvedSlug.join("/")}/` : ""}image.png`;
  const openapiPreload = slug[0] === "api" ? await openapi.preloadOpenAPIPage(page) : undefined;
  const groups = sidebarGroups(shell, place?.section?.id ?? null);
  const seo = (
    <>
      <TechArticleJsonLd
        title={page.data.title}
        description={page.data.description}
        path={page.url}
        imageUrl={ogImagePath}
      />
      <BreadcrumbListJsonLd items={breadcrumbs} />
    </>
  );
  const drawer = (
    <DocsDrawer>
      <DocsSidebar groups={groups} activeUrl={url} />
    </DocsDrawer>
  );

  if (url === "/docs" || !place) {
    return (
      <>
        {seo}
        <DocsHome shell={shell}>
          <MDX components={getMDXComponents(openapiPreload)} />
        </DocsHome>
        {drawer}
      </>
    );
  }

  const content = await getDocsPageSource(page);
  const template = templateOf(slug, url, content, page.data.description ?? "");
  const toc = tocOf(template, page.data.toc);
  const layout = layoutOf(template, url);
  const { previous, next } = neighboursOf(shell, url);

  return (
    <div className="mx-auto grid max-w-[1600px] lg:grid-cols-[296px_minmax(0,1fr)]">
      {seo}
      <aside
        className={cn(
          "docs-rail sticky hidden h-[calc(100vh-104px)] overflow-y-auto border-r px-4 pt-8 pb-12 lg:block",
          BELOW_HEADER,
          LINE
        )}
      >
        <DocsSidebar groups={groups} activeUrl={url} />
      </aside>
      <DocsPageFrame toc={toc} wide={layout.wide}>
        <div className={layout.chromeClass}>
          <DocsMasthead
            place={place}
            title={mastheadTitle(template, place.title)}
            facts={factsOf(template, url, content)}
            maturity={page.data.maturity}
            actions={buildActionUrls(page.url, slug, page.path)}
            toc={layout.disclosure ? <DocsTocDisclosure toc={toc} /> : null}
          />
        </div>
        {template.kind === "cli" ? (
          <div className="pt-9">
            <CliBody cli={template.cli} />
          </div>
        ) : (
          <div className={layout.bodyClass}>
            <MDX components={getMDXComponents(openapiPreload)} />
          </div>
        )}
        <div className={layout.chromeClass}>
          <DocsPager previous={previous} next={next} />
        </div>
      </DocsPageFrame>
      {drawer}
    </div>
  );
}

export async function generateStaticParams() {
  return docsSource.generateParams();
}

export async function generateMetadata(props: PageProps): Promise<Metadata> {
  const params = await props.params;
  const page = docsSource.getPage(params.slug ?? []);
  if (!page) notFound();

  return {
    ...createPageMetadata({
      title: page.data.title,
      description: page.data.description,
      path: page.url,
    }),
  };
}
