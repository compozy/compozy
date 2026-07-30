import { docsSource } from "@/lib/source";
import { DocsBody, DocsPage } from "fumadocs-ui/page";
import { notFound } from "next/navigation";
import type { Metadata } from "next";
import { DocPageMasthead } from "@/components/docs/doc-page-masthead";
import { DocsMainContainer } from "@/components/site/docs-main-container";
import {
  BreadcrumbListJsonLd,
  TechArticleJsonLd,
  type BreadcrumbItem,
} from "@/components/seo/structured-data";
import { getMDXComponents } from "@/mdx-components";
import { openapi } from "@/lib/openapi";
import { resolveDocMastheadMeta } from "@/lib/doc-masthead-meta";
import { absoluteUrl, canonicalPath, createPageMetadata, docsSourceUrl } from "@/lib/site-config";

interface PageProps {
  params: Promise<{ slug?: string[] }>;
}

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

export default async function Page(props: PageProps) {
  const params = await props.params;
  const slug = params.slug ?? [];
  const page = docsSource.getPage(slug);
  if (!page) notFound();

  const MDX = page.data.body;
  const resolvedSlug = resolvedDocSlug(page.url);
  const actions = buildActionUrls(page.url, slug, page.path);
  const breadcrumbs = buildBreadcrumbs(resolvedSlug, page.data.title);
  const masthead = resolveDocMastheadMeta(slug, docsSource.pageTree, page.url, page.data.title);
  const ogImagePath = `/og/docs/${resolvedSlug.length ? `${resolvedSlug.join("/")}/` : ""}image.png`;
  const openapiPreload = slug[0] === "api" ? await openapi.preloadOpenAPIPage(page) : undefined;

  return (
    <DocsPage
      id="main-content"
      toc={page.data.toc}
      tableOfContent={{ enabled: slug.length > 0 }}
      breadcrumb={{ enabled: false }}
      tableOfContentPopover={{ enabled: false }}
      slots={{ container: DocsMainContainer }}
      className="px-4 pt-8 pb-12 md:px-6 xl:layout:[--fd-toc-width:14rem] xl:pt-10"
    >
      <TechArticleJsonLd
        title={page.data.title}
        description={page.data.description}
        path={page.url}
        imageUrl={ogImagePath}
      />
      <BreadcrumbListJsonLd items={breadcrumbs} />
      <DocPageMasthead
        product={masthead.product}
        audience={masthead.audience}
        crumbs={masthead.crumbs}
        title={page.data.title}
        description={page.data.description}
        maturity={page.data.maturity}
        sectionPageCount={masthead.sectionPageCount}
        markdownUrl={actions.markdownUrl}
        pageUrl={actions.pageUrl}
        githubUrl={actions.githubUrl}
      />
      <DocsBody className="site-doc-body mt-8 max-w-none">
        <MDX components={getMDXComponents(openapiPreload)} />
      </DocsBody>
    </DocsPage>
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
