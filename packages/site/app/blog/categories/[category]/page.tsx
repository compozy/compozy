import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { BlogPage } from "@/components/blog/blog-page";
import { categoryLabel } from "@/components/blog/format";
import { BLOG_CATEGORIES, type BlogCategory, postsByCategory } from "@/lib/blog";
import { createPageMetadata } from "@/lib/site-config";

interface PageProps {
  params: Promise<{ category: string }>;
}

export function generateStaticParams() {
  return BLOG_CATEGORIES.map(category => ({ category }));
}

function isCategory(slug: string): slug is BlogCategory {
  return (BLOG_CATEGORIES as readonly string[]).includes(slug);
}

export async function generateMetadata({ params }: PageProps): Promise<Metadata> {
  const { category } = await params;
  if (!isCategory(category)) return {};
  return {
    ...createPageMetadata({
      title: `${categoryLabel(category)} posts`,
      description: `Posts filed under ${categoryLabel(category)}.`,
      path: `/blog/categories/${category}`,
    }),
    robots: { index: postsByCategory(category).length > 0, follow: true },
  };
}

export default async function CategoryArchivePage({ params }: PageProps) {
  const { category } = await params;
  if (!isCategory(category)) notFound();
  return <BlogPage category={category} />;
}
