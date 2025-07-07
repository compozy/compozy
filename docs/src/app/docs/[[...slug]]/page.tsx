import { FeatureCard, FeatureCardList } from "@/components/ui/feature-card";
import { Step, Steps } from "@/components/ui/step";
import { source } from "@/lib/source";
import { cn } from "@/lib/utils";
import { getMDXComponents } from "@/mdx-components";
import Link from "fumadocs-core/link";
import { createRelativeLink } from "fumadocs-ui/mdx";
import { DocsBody, DocsDescription, DocsPage, DocsTitle } from "fumadocs-ui/page";
import { notFound } from "next/navigation";

export default async function Page(props: { params: Promise<{ slug?: string[] }> }) {
  const params = await props.params;
  const page = source.getPage(params.slug);
  if (!page) notFound();

  const MDXContent = page.data.body;

  return (
    <DocsPage toc={page.data.toc} full={page.data.full} tableOfContent={{ style: "clerk" }}>
      <DocsTitle className="text-4xl">{page.data.title}</DocsTitle>
      <DocsDescription>{page.data.description}</DocsDescription>
      <DocsBody>
        <MDXContent
          components={getMDXComponents({
            // this allows you to link to other pages with relative file paths
            a: createRelativeLink(source, page),
            Link: ({ className, ...props }: React.ComponentProps<typeof Link>) => (
              <Link
                className={cn("font-medium underline underline-offset-4", className)}
                {...props}
              />
            ),
            Step,
            Steps,
            FeatureCard,
            FeatureCardList,
          })}
        />
      </DocsBody>
    </DocsPage>
  );
}

export async function generateStaticParams() {
  return source.generateParams();
}

export async function generateMetadata(props: { params: Promise<{ slug?: string[] }> }) {
  const params = await props.params;
  const page = source.getPage(params.slug);
  if (!page) notFound();

  return {
    title: page.data.title,
    description: page.data.description,
  };
}
