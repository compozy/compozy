import { getMDXComponents } from "@/components/ui/mdx-components";
import { source } from "@/lib/source";
import { createRelativeLink } from "fumadocs-ui/mdx";
import { DocsBody, DocsDescription, DocsPage, DocsTitle } from "fumadocs-ui/page";
import { notFound } from "next/navigation";
import { LLMCopyButton, ViewOptions } from "../../../components/ai/page-actions";

export default async function Page(props: { params: Promise<{ slug?: string[] }> }) {
  const params = await props.params;
  const page = source.getPage(params.slug);
  if (!page) notFound();
  const MDXContent = page.data.body;

  return (
    <DocsPage toc={page.data.toc} full={page.data.full} tableOfContent={{ style: "clerk" }}>
      <div className="flex flex-col mb-24">
        <DocsTitle className="text-4xl">{page.data.title}</DocsTitle>
        <DocsDescription>{page.data.description}</DocsDescription>
        <div className="flex flex-row gap-2 items-center border-b pb-6">
          <LLMCopyButton markdownUrl={`${page.url}.mdx`} />
          <ViewOptions
            markdownUrl={`${page.url}.mdx`}
            githubUrl={`https://github.com/compozy/compozy/blob/main/docs/content/docs/${page.path}`}
          />
        </div>
      </div>
      <DocsBody>
        <MDXContent
          components={getMDXComponents({
            a: createRelativeLink(source, page),
          })}
        />
      </DocsBody>
    </DocsPage>
  );
}

export async function generateStaticParams() {
  return source.generateParams();
}

export async function generateMetadata({ params }: { params: Promise<{ slug?: string[] }> }) {
  const { slug = [] } = await params;
  const page = source.getPage(slug);
  if (!page) notFound();
  const image = ["/og", ...slug, "image.png"].join("/");
  return {
    title: page.data.title,
    description: page.data.description,
    openGraph: {
      images: image,
    },
    twitter: {
      card: "summary_large_image",
      images: image,
    },
  };
}
