import { existsSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { generateMetadata, generateStaticParams } from "../../app/blog/[slug]/page";
import { allPosts } from "../blog";

const siteRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");

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
      expect(metadata.openGraph?.url, post.slug).toBe(`https://agh.network${post.permalink}/`);
      expect(metadata.twitter?.title, post.slug).toBe(post.title);
      expect(metadata.twitter?.description, post.slug).toBe(post.description);
    }
  });

  it("does not publish metadata for unknown posts", async () => {
    const metadata = await generateMetadata(pageProps("unknown-post"));

    expect(metadata).toEqual({});
  });

  it("uses the launch cover art in OpenGraph and Twitter metadata", async () => {
    const metadata = await generateMetadata(
      pageProps("introducing-agh-the-first-agent-network-protocol")
    );
    const openGraphImage =
      Array.isArray(metadata.openGraph?.images) && typeof metadata.openGraph.images[0] === "object"
        ? (metadata.openGraph.images[0] as { url?: string; alt?: string })
        : null;
    const twitterImage =
      Array.isArray(metadata.twitter?.images) && typeof metadata.twitter.images[0] === "string"
        ? metadata.twitter.images[0]
        : null;

    expect(openGraphImage?.url).toBe("/static/blog/introducing-agh-cover.png");
    expect(openGraphImage?.alt).toBe(
      "agh-network/v0, three peers exchanging direct, receipt, and trace envelopes"
    );
    expect(twitterImage).toBe("/static/blog/introducing-agh-cover.png");
    expect(existsSync(resolve(siteRoot, "public/static/blog/introducing-agh-cover.png"))).toBe(
      true
    );
  });
});
