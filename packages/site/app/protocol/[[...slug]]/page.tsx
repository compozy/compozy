import { protocolDocs } from "@/lib/source";
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
import { absoluteUrl, createPageMetadata, docsSourceUrl } from "@/lib/site-config";

interface PageProps {
  params: Promise<{ slug?: string[] }>;
}

function buildActionUrls(slug: string[], relativePath: string | undefined) {
  const slugSegment = slug.length ? `${slug.join("/")}/` : "";
  const markdownUrl = `/llms.mdx/protocol/${slugSegment}`;
  const pageUrl = absoluteUrl(`/protocol/${slugSegment}`);
  const githubUrl = relativePath
    ? docsSourceUrl("protocol", relativePath)
    : docsSourceUrl("protocol", "");
  return { markdownUrl, pageUrl, githubUrl };
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
    { name: "Network Protocol", path: "/protocol/" },
  ];
  let cumulative = "/protocol";
  for (let i = 0; i < slug.length; i += 1) {
    cumulative += `/${slug[i]}`;
    const isLast = i === slug.length - 1;
    items.push({ name: isLast ? pageTitle : humanize(slug[i]), path: `${cumulative}/` });
  }
  return items;
}

export default async function Page(props: PageProps) {
  const params = await props.params;
  const slug = params.slug ?? [];
  const page = protocolDocs.getPage(slug);
  if (!page) notFound();

  const MDX = page.data.body;
  const actions = buildActionUrls(slug, page.path);
  const breadcrumbs = buildBreadcrumbs(slug, page.data.title);
  const ogImagePath = `/og/protocol/${slug.length ? `${slug.join("/")}/` : ""}image.png`;

  return (
    <DocsPage
      id="main-content"
      toc={page.data.toc}
      breadcrumb={{ enabled: true }}
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
        kind="protocol"
        slug={slug}
        title={page.data.title}
        description={page.data.description}
        markdownUrl={actions.markdownUrl}
        pageUrl={actions.pageUrl}
        githubUrl={actions.githubUrl}
      />
      <DocsBody className="site-doc-body mt-8 max-w-none">
        <MDX components={getMDXComponents()} />
      </DocsBody>
    </DocsPage>
  );
}

export async function generateStaticParams() {
  return protocolDocs.generateParams();
}

export async function generateMetadata(props: PageProps): Promise<Metadata> {
  const params = await props.params;
  const page = protocolDocs.getPage(params.slug ?? []);
  if (!page) notFound();

  return {
    ...createPageMetadata({
      title: page.data.title,
      description: page.data.description,
      path: page.url,
    }),
  };
}
