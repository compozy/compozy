import Link from "next/link";
import { Rss } from "lucide-react";
import { CategoryPill } from "@/components/blog/category-pill";
import { ChangelogRail } from "@/components/blog/changelog-rail";
import { BlogEmptyState } from "@/components/blog/empty-state";
import { FeaturedPost } from "@/components/blog/featured-post";
import { PostCard } from "@/components/blog/post-card";
import { SubscribeRail } from "@/components/blog/subscribe-rail";
import {
  BLOG_CATEGORIES,
  allPosts,
  allReleases,
  authorInitial,
  categoryCounts,
  featuredPost,
} from "@/lib/blog";
import { categoryLabel } from "@/components/blog/format";
import { blogMetadata } from "./metadata";
import { Eyebrow } from "@agh/ui";

export const metadata = blogMetadata;

export default function BlogIndexPage() {
  const featured = featuredPost();
  const posts = allPosts();
  const grid = featured ? posts.filter(post => post.slug !== featured.slug) : posts;
  const counts = categoryCounts();
  const releases = allReleases();

  return (
    <>
      <section className="border-b border-line px-4 pt-14 pb-14">
        <div className="mx-auto max-w-site-layout-width">
          <div className="flex items-center gap-3">
            <Eyebrow className="text-accent">BLOG</Eyebrow>
            <span className="inline-block h-px w-9 bg-line" />
            <Eyebrow className="text-muted">Field notes from the runtime</Eyebrow>
          </div>
          <h1 className="mt-6 max-w-[20ch] font-display text-site-blog-title font-normal leading-none tracking-tight text-fg">
            The runtime, the protocol, the receipts.
          </h1>
          <p className="mt-6 max-w-[58ch] text-lg leading-7 text-muted">
            Protocol design, runtime engineering, and release receipts from the team shipping{" "}
            <span className="text-fg">agh-network/v0</span>. Read in any order.
          </p>
          <div className="mt-9 flex flex-wrap items-center gap-2">
            <CategoryPill label="All" count={posts.length} href="/blog" active />
            {BLOG_CATEGORIES.map(category => (
              <CategoryPill
                key={category}
                label={categoryLabel(category)}
                count={counts[category]}
                href={`/blog/categories/${category}`}
              />
            ))}
            <span className="mx-1 inline-block h-4 w-px bg-line" />
            <Link
              href="/blog/feed.xml"
              className="inline-flex h-8 items-center gap-1.5 rounded-full px-3 font-sans text-small-body text-muted hover:text-fg"
            >
              <Rss size={12} aria-hidden />
              <Eyebrow className="text-muted">RSS</Eyebrow>
            </Link>
          </div>
        </div>
      </section>

      {featured && (
        <section className="px-4 pt-12 pb-6">
          <div className="mx-auto max-w-site-layout-width">
            <FeaturedPost post={featured} authorInitial={authorInitial(featured.author)} />
          </div>
        </section>
      )}

      <section className="px-4 pt-8 pb-20">
        <div className="mx-auto max-w-site-layout-width">
          <div className="flex items-baseline justify-between">
            <div className="flex items-center gap-3">
              <Eyebrow className="text-muted tracking-badge!">LATEST</Eyebrow>
              <span className="inline-block h-px w-9 bg-line" />
              <span className="text-small-body text-muted">Newest first</span>
            </div>
          </div>
          <div className="mt-6 grid gap-6 lg:grid-cols-[minmax(0,1fr)_320px]">
            <div className="grid gap-5 sm:grid-cols-2">
              {grid.map(post => (
                <PostCard key={post.slug} post={post} />
              ))}
              {grid.length === 0 && (
                <BlogEmptyState
                  eyebrow="Archive pending"
                  title="More field notes are being prepared."
                  description="The featured post is the full archive for now. Use the RSS feed to catch the next runtime note, protocol note, or release receipt as soon as it is published."
                  primaryAction={{ href: "/blog/feed.xml", label: "Subscribe via RSS" }}
                  secondaryAction={{ href: "/changelog", label: "Open the changelog" }}
                />
              )}
            </div>
            <div className="flex flex-col gap-5">
              {releases.length > 0 && <ChangelogRail releases={releases} />}
              <SubscribeRail />
            </div>
          </div>
        </div>
      </section>
    </>
  );
}
