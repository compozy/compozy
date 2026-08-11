import { describe, expect, it, vi } from "vitest";
import { siteConfig } from "../site-config";

vi.mock("next/font/google", () => ({
  Geist: () => ({ variable: "--font-geist" }),
  JetBrains_Mono: () => ({ variable: "--font-jetbrains-mono" }),
  Playfair_Display: () => ({ variable: "--font-playfair" }),
}));

describe("static public route metadata", () => {
  it("publishes canonical metadata for the blog index", async () => {
    const { blogMetadata: metadata } = await import("@/app/blog/metadata");

    expect(metadata.title).toBe("Blog");
    expect(metadata.description).toBe(
      "Field notes on building, operating, and extending CompozyOS: loops, automation, memory, permissions, and the runtime behind them."
    );
    expect(metadata.alternates?.canonical).toBe("/blog/");
    expect(metadata.openGraph?.title).toBe("Blog");
    expect(metadata.openGraph?.description).toBe(metadata.description);
    expect(metadata.openGraph?.url).toBe("https://compozy.com/blog/");
    expect(metadata.openGraph?.siteName).toBe(siteConfig.name);
    expect(metadata.twitter).toMatchObject({
      card: "summary_large_image",
      title: "Blog",
      description: metadata.description,
    });
  });

  it("publishes canonical metadata for the changelog index", async () => {
    const { changelogMetadata: metadata } = await import("@/app/changelog/metadata");

    expect(metadata.title).toBe("Changelog");
    expect(metadata.description).toBe(
      "Complete GitHub release notes, pull requests, contributors, and assets for CompozyOS."
    );
    expect(metadata.alternates?.canonical).toBe("/changelog/");
    expect(metadata.alternates?.types).toEqual({
      "application/rss+xml": "/changelog/feed.xml",
    });
    expect(metadata.openGraph?.title).toBe("Changelog");
    expect(metadata.openGraph?.description).toBe(metadata.description);
    expect(metadata.openGraph?.url).toBe("https://compozy.com/changelog/");
    expect(metadata.openGraph?.siteName).toBe(siteConfig.name);
    expect(metadata.twitter).toMatchObject({
      card: "summary_large_image",
      title: "Changelog",
      description: metadata.description,
    });
  });

  it(
    "keeps root metadata aligned with site identity and design tokens",
    { timeout: 60_000 },
    async () => {
      const { metadata, viewport } = await import("@/app/layout");

      expect(siteConfig.description).toBe(
        "The system around the agent, already built. CompozyOS keeps AI agents working continuously, without the scripts, cron jobs, and glue code people otherwise assemble."
      );
      expect(metadata.metadataBase?.toString()).toBe("https://compozy.com/");
      expect(metadata.applicationName).toBe(siteConfig.name);
      expect(metadata.title).toEqual({
        default: siteConfig.name,
        template: "%s | CompozyOS",
      });
      expect(metadata.description).toBe(siteConfig.description);
      expect(metadata.alternates?.canonical).toBe("/");
      expect(metadata.openGraph).toMatchObject({
        type: "website",
        locale: "en_US",
        url: siteConfig.url,
        siteName: siteConfig.name,
        title: siteConfig.name,
        description: siteConfig.description,
      });
      expect(metadata.twitter).toMatchObject({
        card: "summary_large_image",
        title: siteConfig.name,
        description: siteConfig.description,
        images: ["/opengraph-image"],
      });
      expect(metadata.robots).toEqual({ index: true, follow: true });
      expect(metadata.manifest).toBe("/site.webmanifest");
      expect(viewport.themeColor).toBe("#E8572A");
    }
  );
});
