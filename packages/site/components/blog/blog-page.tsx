/** Server entry for `/blog` and `/blog/categories/[category]`: loads the entries and hands them to the index. */
import { BLOG_CATEGORIES, blogEntries, blogEntry, featuredPost, postAuthor } from "@/lib/blog";

import { BlogIndex } from "./blog-index";

export function BlogPage({ category }: { category: string | null }) {
  const lead = featuredPost();
  const entries = blogEntries();
  if (!lead) return null;
  return (
    <BlogIndex
      entries={entries}
      categories={BLOG_CATEGORIES}
      category={category}
      lead={blogEntry(lead)}
      author={postAuthor(lead)}
    />
  );
}
