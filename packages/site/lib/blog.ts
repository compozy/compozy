import { authors, posts, releases, type Author, type Post, type Release } from "#site/content";

export const BLOG_CATEGORIES = ["protocol", "runtime", "engineering", "network"] as const;
export type BlogCategory = (typeof BLOG_CATEGORIES)[number];
export type BlogCover = { src: string; alt: string; width: number; height: number };

const FEATURED_COVER_BY_SLUG: Record<string, BlogCover> = {
  "posts/introducing-agh-the-first-agent-network-protocol": {
    src: "/static/blog/introducing-agh-cover.png",
    alt: "agh-network/v0, three peers exchanging direct, receipt, and trace envelopes",
    width: 1600,
    height: 1000,
  },
};

const sortedPostsCache = posts.toSorted(
  (a, b) => new Date(b.date).getTime() - new Date(a.date).getTime()
);

const sortedReleasesCache = releases.toSorted(
  (a, b) => new Date(b.date).getTime() - new Date(a.date).getTime()
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

export function blogPostCover(post: Pick<Post, "cover" | "slug" | "title">): BlogCover | null {
  if (post.cover?.src) {
    return {
      src: post.cover.src,
      alt: `${post.title} cover art`,
      width: post.cover.width,
      height: post.cover.height,
    };
  }

  return FEATURED_COVER_BY_SLUG[post.slug] ?? null;
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

export function authorInitial(handle: string): string {
  const author = authorByHandle(handle);
  if (author) return author.avatar.charAt(0).toUpperCase();
  return handle.charAt(0).toUpperCase();
}

export function allReleases(): Release[] {
  return sortedReleasesCache;
}

export type { Author, Post, Release };
