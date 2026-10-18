import { describe, expect, it } from "vitest";
import { generateMetadata, generateStaticParams } from "../../app/blog/[slug]/page";
import { allPosts, authorByHandle } from "../blog";

function pageProps(slug: string) {
  return {
    params: Promise.resolve({ slug }),
  };
}

describe("blog metadata", () => {
  it("generates one static route per public post slug", () => {
    expect(generateStaticParams()).toEqual(
      allPosts().map(post => ({ slug: post.slug.replace(/^posts\//, "") }))
    );
  });

  it("publishes canonical metadata for every blog post", async () => {
    for (const post of allPosts()) {
      const publicSlug = post.slug.replace(/^posts\//, "");
      const metadata = await generateMetadata(pageProps(publicSlug));

      expect(metadata.title, post.slug).toBe(post.title);
      expect(metadata.description, post.slug).toBe(post.description);
      expect(metadata.alternates?.canonical, post.slug).toBe(`${post.permalink}/`);
      expect(metadata.openGraph?.title, post.slug).toBe(post.title);
      expect(metadata.openGraph?.description, post.slug).toBe(post.description);
      expect(metadata.openGraph?.url, post.slug).toBe(`https://compozy.com${post.permalink}/`);
      const author = authorByHandle(post.author);
      expect(metadata.authors, post.slug).toEqual([
        { name: author?.name ?? post.author, url: author?.github },
      ]);
      expect(metadata.openGraph, post.slug).toMatchObject({
        type: "article",
        publishedTime: post.date,
        modifiedTime: post.updated ?? post.date,
        authors: author?.github ? [author.github] : undefined,
        tags: post.tags,
      });
      expect(metadata.robots, post.slug).toEqual({
        index: true,
        follow: true,
        googleBot: { index: true, follow: true, "max-image-preview": "large" },
      });
      expect(metadata.twitter?.title, post.slug).toBe(post.title);
      expect(metadata.twitter?.description, post.slug).toBe(post.description);
    }
  });

  it("does not publish metadata for unknown posts", async () => {
    const metadata = await generateMetadata(pageProps("unknown-post"));

    expect(metadata).toEqual({});
  });

  it("uses the generated social card when a post has no cover", async () => {
    const metadata = await generateMetadata(pageProps("introducing-compozyos"));
    const openGraphImage =
      Array.isArray(metadata.openGraph?.images) && typeof metadata.openGraph.images[0] === "object"
        ? (metadata.openGraph.images[0] as { url?: string; alt?: string })
        : null;
    const twitterImage =
      Array.isArray(metadata.twitter?.images) && typeof metadata.twitter.images[0] === "string"
        ? metadata.twitter.images[0]
        : null;

    expect(openGraphImage?.url).toBe("/og/blog/introducing-compozyos/image.png");
    expect(openGraphImage?.alt).toBe(
      `${allPosts().find(post => post.slug === "posts/introducing-compozyos")?.title} | CompozyOS`
    );
    expect(twitterImage).toBe("/og/blog/introducing-compozyos/image.png");
  });
});
