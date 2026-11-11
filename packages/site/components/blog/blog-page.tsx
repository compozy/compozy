/** Server entry for `/blog` and `/blog/categories/[category]`: loads the entries and hands them to the index. */
import { Suspense } from "react";

import { BLOG_CATEGORIES, blogEntries, blogEntry, featuredPost, postAuthor } from "@/lib/blog";

import { BlogIndex, BlogIndexFromUrl } from "./blog-index";

export function BlogPage({ category }: { category: string | null }) {
  const lead = featuredPost();
  const entries = blogEntries();
  if (!lead) return null;
  const props = {
    entries,
    categories: BLOG_CATEGORIES,
    category,
    lead: blogEntry(lead),
    author: postAuthor(lead),
  };
  return (
    // The prerendered page starts unfiltered; the URL's `?topic=` and `?q=` apply once it hydrates.
    <Suspense fallback={<BlogIndex {...props} />}>
      <BlogIndexFromUrl {...props} />
    </Suspense>
  );
}
