import { render, screen, within } from "@testing-library/react";
import { posts, type Post } from "#site/content";
import type { AnchorHTMLAttributes, ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { ArchiveRow } from "../archive-row";
import { CategoryPill } from "../category-pill";
import { ContinueReading } from "../continue-reading";
import { FeaturedPost } from "../featured-post";
import { PostCard } from "../post-card";
import { SubscribeRail } from "../subscribe-rail";

vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    ...props
  }: {
    href: string;
    children: ReactNode;
  } & Omit<AnchorHTMLAttributes<HTMLAnchorElement>, "href">) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

function firstPost() {
  const post = posts[0];
  if (!post) {
    throw new Error("Expected generated blog content to include at least one post");
  }
  return post;
}

function postWithoutCover(): Post {
  const post = firstPost();
  return {
    ...post,
    slug: "posts/fallback-visual",
    permalink: "/blog/fallback-visual",
    cover: undefined,
  };
}

describe("blog navigation components", () => {
  it("keeps subscription links canonical, accessible, and opener-safe", () => {
    render(<SubscribeRail />);

    expect(screen.getByRole("complementary", { name: "Blog subscription links" })).toBeDefined();
    const feed = screen.getByRole("link", { name: "RSS feed" });
    const changelog = screen.getByRole("link", { name: "Read the changelog" });

    expect(feed.getAttribute("href")).toBe("/blog/feed.xml");
    expect(screen.getByText("/blog/feed.xml")).toBeDefined();
    expect(changelog.getAttribute("href")).toBe("/changelog");
    expect(changelog.getAttribute("target")).toBeNull();
    expect(changelog.getAttribute("rel")).toBeNull();
  });

  it("renders blog category filters with stable hrefs and padded counts", () => {
    render(<CategoryPill label="Runtime" count={3} href="/blog/categories/runtime" active />);

    const filter = screen.getByRole("link", { name: "Runtime (3)" });
    expect(filter.getAttribute("href")).toBe("/blog/categories/runtime");
    expect(filter.getAttribute("aria-current")).toBe("page");
    expect(filter.getAttribute("class")).toContain("border-accent");
  });

  it("keeps post cards and archive rows linked to the public permalink", () => {
    const post = firstPost();
    const { container, rerender } = render(<PostCard post={post} />);

    const cardLink = screen.getByRole("link", { name: post.title });
    expect(cardLink.getAttribute("href")).toBe(post.permalink);
    expect(screen.getByText(post.description)).toBeDefined();
    expect(screen.getByText(post.author)).toBeDefined();
    expect(container.querySelector("time")?.getAttribute("dateTime")).toBe(post.date);

    rerender(<ArchiveRow post={post} />);

    const rowLink = screen.getByRole("link", { name: new RegExp(post.title) });
    expect(rowLink.getAttribute("href")).toBe(post.permalink);
    expect(rowLink.getAttribute("class")).toContain("grid-cols-1");
    expect(rowLink.getAttribute("class")).toContain("sm:grid-cols-[104px_minmax(0,1fr)]");
    expect(rowLink.getAttribute("class")).toContain(
      "lg:grid-cols-[88px_minmax(0,1fr)_minmax(96px,140px)_70px_16px]"
    );
    expect(within(rowLink).getByText(post.description)).toBeDefined();
    expect(within(rowLink).getByText(post.author)).toBeDefined();
    expect(within(rowLink).getByText(post.author).getAttribute("class")).toContain("truncate");
    expect(rowLink.querySelector("time")?.getAttribute("dateTime")).toBe(post.date);
    expect(rowLink.querySelector("svg")?.getAttribute("class")).toContain("hidden");
  });

  it("renders related reading links and keeps the empty queue actionable", () => {
    const post = firstPost();
    const { container, rerender } = render(<ContinueReading posts={[post]} />);

    expect(screen.getByRole("link", { name: "All posts →" }).getAttribute("href")).toBe("/blog");
    expect(screen.getByRole("link", { name: post.title }).getAttribute("href")).toBe(
      post.permalink
    );
    expect(screen.getByRole("link", { name: `Read ${post.title}` }).getAttribute("href")).toBe(
      post.permalink
    );
    expect(container.querySelector("time")?.getAttribute("dateTime")).toBe(post.date);

    rerender(<ContinueReading posts={[]} />);
    expect(
      screen.getByRole("heading", { name: "More field notes are being prepared." })
    ).toBeDefined();
    expect(screen.getByRole("link", { name: "Subscribe via RSS" }).getAttribute("href")).toBe(
      "/blog/feed.xml"
    );
    expect(screen.getByRole("link", { name: "Read the release log" }).getAttribute("href")).toBe(
      "/changelog"
    );
  });

  it("keeps the featured fallback visual from making live availability claims", () => {
    const post = postWithoutCover();
    const { container } = render(<FeaturedPost post={post} authorInitial="A" />);

    expect(screen.getByText("agh-network/v0")).toBeDefined();
    expect(screen.getByText("ALPHA")).toBeDefined();
    expect(container.querySelector("time")?.getAttribute("dateTime")).toBe(post.date);
    expect(screen.queryByText("LIVE")).toBeNull();
  });
});
