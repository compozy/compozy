import { describe, expect, it } from "vitest";
import { absoluteUrl, canonicalPath, createPageMetadata, siteConfig } from "../site-config";

describe("site config metadata helpers", () => {
  it("builds absolute URLs and canonical paths consistently", () => {
    expect(siteConfig.url).toBe("https://compozy.com");
    expect(absoluteUrl("/docs")).toBe("https://compozy.com/docs");
    expect(absoluteUrl("/docs/")).toBe("https://compozy.com/docs/");
    expect(canonicalPath()).toBe("/");
    expect(canonicalPath("")).toBe("/");
    expect(canonicalPath("/")).toBe("/");
    expect(canonicalPath("/docs")).toBe("/docs/");
    expect(canonicalPath("/docs/")).toBe("/docs/");
  });

  it("creates canonical OpenGraph and Twitter metadata with dynamic OG images", () => {
    const metadata = createPageMetadata({
      title: "Runtime Overview",
      path: "/docs",
    });

    expect(metadata).toEqual({
      title: "Runtime Overview",
      description: siteConfig.description,
      keywords: undefined,
      alternates: {
        canonical: "/docs/",
      },
      openGraph: {
        title: "Runtime Overview",
        description: siteConfig.description,
        url: "https://compozy.com/docs/",
        siteName: "CompozyOS",
        images: [
          {
            url: "/og/docs/image.png",
            width: 1200,
            height: 630,
            alt: "Runtime Overview | CompozyOS",
          },
        ],
      },
      twitter: {
        card: "summary_large_image",
        title: "Runtime Overview",
        description: siteConfig.description,
        images: ["/og/docs/image.png"],
      },
    });
  });

  it("falls back to static OG image for the home page", () => {
    const metadata = createPageMetadata({ title: "CompozyOS", path: "/" });
    expect(metadata.openGraph.images).toEqual([
      {
        url: "/opengraph-image",
        width: 1200,
        height: 630,
        alt: "CompozyOS | CompozyOS",
      },
    ]);
  });

  it("emits keywords when provided", () => {
    const metadata = createPageMetadata({
      title: "Launch Post",
      path: "/blog/launch",
      keywords: ["alpha", "compozy-network/v0"],
    });
    expect(metadata.keywords).toEqual(["alpha", "compozy-network/v0"]);
  });

  it("preserves page descriptions and custom social images", () => {
    const metadata = createPageMetadata({
      title: "Launch Post",
      description: "A public alpha announcement for CompozyOS.",
      path: "/blog/launch",
      image: {
        url: "/static/blog/launch.png",
        alt: "CompozyOS launch cover",
        width: 1600,
        height: 1000,
      },
    });

    expect(metadata.description).toBe("A public alpha announcement for CompozyOS.");
    expect(metadata.alternates.canonical).toBe("/blog/launch/");
    expect(metadata.openGraph.description).toBe("A public alpha announcement for CompozyOS.");
    expect(metadata.openGraph.url).toBe("https://compozy.com/blog/launch/");
    expect(metadata.openGraph.images).toEqual([
      {
        url: "/static/blog/launch.png",
        alt: "CompozyOS launch cover",
        width: 1600,
        height: 1000,
      },
    ]);
    expect(metadata.twitter.images).toEqual(["/static/blog/launch.png"]);
  });
});
