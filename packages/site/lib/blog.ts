import { authors, posts, type Author, type Post } from "#site/content";

import type { BlogEntry } from "./blog-index";

export const BLOG_CATEGORIES = ["runtime", "engineering"] as const;
export type BlogCategory = (typeof BLOG_CATEGORIES)[number];
export type BlogCover = { src: string; alt: string; width: number; height: number };

/** Newest first. On the same day the featured post leads, then slugs keep the order stable. */
const sortedPostsCache = posts.toSorted(
  (a, b) =>
    new Date(b.date).getTime() - new Date(a.date).getTime() ||
    Number(b.featured) - Number(a.featured) ||
    a.slug.localeCompare(b.slug)
);

export function allPosts(): Post[] {
  return sortedPostsCache;
}

export function postBySlug(slug: string): Post | undefined {
  const target = slug.startsWith("posts/") ? slug : `posts/${slug}`;
  return posts.find(post => post.slug === target);
}

export function postsByCategory(category: BlogCategory): Post[] {
  return sortedPostsCache.filter(post => post.category === category);
}

export function categoryCounts(): Record<BlogCategory, number> {
  const counts = Object.fromEntries(BLOG_CATEGORIES.map(c => [c, 0])) as Record<
    BlogCategory,
    number
  >;
  for (const post of sortedPostsCache) {
    counts[post.category as BlogCategory] += 1;
  }
  return counts;
}

export function featuredPost(): Post | undefined {
  return sortedPostsCache.find(post => post.featured) ?? sortedPostsCache[0];
}

export function blogPostCover(post: Pick<Post, "cover" | "title">): BlogCover {
  return {
    src: post.cover.src,
    alt: `${post.title} cover art`,
    width: post.cover.width,
    height: post.cover.height,
  };
}

/** The post's issue number: the oldest post is 01, so a number never changes once published. */
export function postNumber(post: Pick<Post, "slug">): string {
  const index = sortedPostsCache.findIndex(candidate => candidate.slug === post.slug);
  return String(sortedPostsCache.length - index).padStart(2, "0");
}

export function readingMinutes(post: Pick<Post, "metadata">): number {
  return Math.max(1, Math.round(post.metadata.readingTime));
}

export function blogEntry(post: Post): BlogEntry {
  return {
    slug: post.slug.replace(/^posts\//, ""),
    permalink: post.permalink,
    title: post.title,
    description: post.description,
    date: post.date,
    category: post.category,
    tags: post.tags,
    featured: post.featured,
    minutes: readingMinutes(post),
    number: postNumber(post),
    cover: {
      src: post.cover.src,
      width: post.cover.width,
      height: post.cover.height,
      blurDataURL: post.cover.blurDataURL,
    },
  };
}

export function blogEntries(): BlogEntry[] {
  return sortedPostsCache.map(blogEntry);
}

export function relatedPosts(post: Post, limit = 3): Post[] {
  const candidates = sortedPostsCache.filter(candidate => candidate.slug !== post.slug);
  const tagSet = new Set(post.tags);
  const scored = candidates.map(candidate => {
    let score = 0;
    if (candidate.category === post.category) score += 3;
    for (const tag of candidate.tags) {
      if (tagSet.has(tag)) score += 1;
    }
    return { candidate, score };
  });
  return scored
    .toSorted((a, b) => {
      if (b.score !== a.score) return b.score - a.score;
      return new Date(b.candidate.date).getTime() - new Date(a.candidate.date).getTime();
    })
    .slice(0, limit)
    .map(entry => entry.candidate);
}

export function authorByHandle(handle: string): Author | undefined {
  return authors.find(author => author.handle === handle);
}

export interface PostAuthor {
  name: string;
  role?: string;
  url?: string;
  avatar?: string;
}

/** The byline: the author's GitHub avatar when a profile is known. */
export function postAuthor(post: Pick<Post, "author">): PostAuthor {
  const author = authorByHandle(post.author);
  if (!author) return { name: post.author };
  return {
    name: author.name,
    role: author.role,
    url: author.github,
    avatar: author.github ? `${author.github}.png?size=96` : undefined,
  };
}

export type { Author, Post };
