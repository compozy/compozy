import { describe, expect, it } from "vitest";
import { absoluteUrl, canonicalPath, createPageMetadata, siteConfig } from "../site-config";

describe("site config metadata helpers", () => {
  it("builds absolute URLs and canonical paths consistently", () => {
    expect(siteConfig.url).toBe("https://agh.network");
    expect(absoluteUrl("/runtime")).toBe("https://agh.network/runtime");
    expect(absoluteUrl("/runtime/")).toBe("https://agh.network/runtime/");
    expect(canonicalPath()).toBe("/");
    expect(canonicalPath("")).toBe("/");
    expect(canonicalPath("/")).toBe("/");
    expect(canonicalPath("/runtime")).toBe("/runtime/");
    expect(canonicalPath("/runtime/")).toBe("/runtime/");
  });

  it("creates canonical OpenGraph and Twitter metadata with dynamic OG images", () => {
    const metadata = createPageMetadata({
      title: "Runtime Overview",
      path: "/runtime",
    });

    expect(metadata).toEqual({
      title: "Runtime Overview",
      description: siteConfig.description,
      keywords: undefined,
      alternates: {
        canonical: "/runtime/",
      },
      openGraph: {
        title: "Runtime Overview",
        description: siteConfig.description,
        url: "https://agh.network/runtime/",
        siteName: "AGH",
        images: [
          {
            url: "/og/runtime/image.png",
            width: 1200,
            height: 630,
            alt: "Runtime Overview | AGH",
          },
        ],
      },
      twitter: {
        card: "summary_large_image",
        title: "Runtime Overview",
        description: siteConfig.description,
        images: ["/og/runtime/image.png"],
      },
    });
  });

  it("falls back to static OG image for the home page", () => {
    const metadata = createPageMetadata({ title: "AGH", path: "/" });
    expect(metadata.openGraph.images).toEqual([
      {
        url: "/opengraph-image",
        width: 1200,
        height: 630,
        alt: "AGH | AGH",
      },
    ]);
  });

  it("emits keywords when provided", () => {
    const metadata = createPageMetadata({
      title: "Launch Post",
      path: "/blog/launch",
      keywords: ["alpha", "agh-network/v0"],
    });
    expect(metadata.keywords).toEqual(["alpha", "agh-network/v0"]);
  });

  it("preserves page descriptions and custom social images", () => {
    const metadata = createPageMetadata({
      title: "Launch Post",
      description: "A public alpha announcement for AGH.",
      path: "/blog/launch",
      image: {
        url: "/static/blog/launch.png",
        alt: "AGH launch cover",
        width: 1600,
        height: 1000,
      },
    });

    expect(metadata.description).toBe("A public alpha announcement for AGH.");
    expect(metadata.alternates.canonical).toBe("/blog/launch/");
    expect(metadata.openGraph.description).toBe("A public alpha announcement for AGH.");
    expect(metadata.openGraph.url).toBe("https://agh.network/blog/launch/");
    expect(metadata.openGraph.images).toEqual([
      {
        url: "/static/blog/launch.png",
        alt: "AGH launch cover",
        width: 1600,
        height: 1000,
      },
    ]);
    expect(metadata.twitter.images).toEqual(["/static/blog/launch.png"]);
  });
});
