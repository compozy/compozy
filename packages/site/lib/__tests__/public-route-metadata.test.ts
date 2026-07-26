import { describe, expect, it, vi } from "vitest";
import { GET as feedGET } from "@/app/blog/feed.xml/route";
import { GET as llmsGET } from "@/app/llms.txt/route";
import robots from "@/app/robots";
import sitemap from "@/app/sitemap";
import { BLOG_CATEGORIES, allPosts } from "@/lib/blog";
import { absoluteUrl, canonicalPath, siteConfig } from "@/lib/site-config";

const mockedDocs = vi.hoisted(() => ({
  protocolPages: [
    {
      data: { description: "Implemented protocol surface.", title: "Implementation Status" },
      url: "/protocol/implementation-status",
    },
  ],
  runtimePages: [
    {
      data: { description: "Runtime docs overview.", title: "How to use these docs" },
      url: "/runtime/how-to-use-these-docs",
    },
    {
      data: {
        description: "Prepare a project workspace for agent execution.",
        title: "Prepare a project workspace",
      },
      url: "/runtime/use-cases/prepare-a-project-workspace",
    },
  ],
}));

vi.mock("@/lib/source", () => ({
  protocolDocs: {
    getPages: () => mockedDocs.protocolPages,
  },
  runtimeDocs: {
    getPages: () => mockedDocs.runtimePages,
  },
}));

function parseXml(xml: string): Document {
  const document = new DOMParser().parseFromString(xml, "application/xml");
  const errors = document.getElementsByTagName("parsererror");
  expect([...errors].map(error => error.textContent)).toEqual([]);
  return document;
}

function expectSiteUrl(url: string): URL {
  const parsed = new URL(url);
  const site = new URL(siteConfig.url);
  expect(parsed.protocol).toBe("https:");
  expect(parsed.origin).toBe(site.origin);
  expect(parsed.hash).toBe("");
  expect(parsed.search).toBe("");
  expect(parsed.pathname.endsWith(".md")).toBe(false);
  expect(parsed.pathname.endsWith(".mdx")).toBe(false);
  return parsed;
}

function textOf(parent: Element, tagName: string): string {
  return parent.getElementsByTagName(tagName)[0]?.textContent ?? "";
}

describe("public route metadata", () => {
  it("publishes canonical HTTPS sitemap entries", () => {
    const entries = sitemap();
    const urls = entries.map(entry => entry.url);

    expect(urls.length).toBeGreaterThan(0);
    expect(new Set(urls).size).toBe(urls.length);

    for (const url of urls) {
      const parsed = expectSiteUrl(url);
      expect(parsed.pathname).toBe(canonicalPath(parsed.pathname));
    }

    expect(urls).toContain(absoluteUrl("/"));
    expect(urls).toContain(absoluteUrl("/runtime/how-to-use-these-docs/"));
    expect(urls).toContain(absoluteUrl("/runtime/use-cases/prepare-a-project-workspace/"));
    expect(urls).toContain(absoluteUrl("/protocol/implementation-status/"));
    for (const category of BLOG_CATEGORIES) {
      expect(urls).toContain(absoluteUrl(`/blog/categories/${category}/`));
    }
  });

  it("points robots.txt at the canonical sitemap", () => {
    const route = robots();

    expect(route.rules).toEqual({
      userAgent: "*",
      allow: "/",
    });
    expect(route.sitemap).toBe(`${siteConfig.url}/sitemap.xml`);
  });

  it("publishes llms.txt with the corrected tagline and canonical doc links", async () => {
    const response = llmsGET();
    const body = await response.text();

    expect(response.headers.get("Content-Type")).toBe("text/plain; charset=utf-8");
    expect(body).toContain(
      "> An open workplace for AI agents, the runtime, the agh-network/v0 protocol, and the blog."
    );
    expect(body).toContain(
      "- [How to use these docs](https://agh.network/runtime/how-to-use-these-docs): Runtime docs overview."
    );
    expect(body).toContain(
      "- [Implementation Status](https://agh.network/protocol/implementation-status): Implemented protocol surface."
    );
  });

  it("publishes a parseable RSS feed with canonical post links", async () => {
    const response = feedGET();
    const xml = await response.text();
    const document = parseXml(xml);
    const posts = allPosts();
    const items = [...document.getElementsByTagName("item")];

    expect(response.headers.get("content-type")).toBe("application/rss+xml; charset=utf-8");
    expect(response.headers.get("cache-control")).toBe(
      "s-maxage=3600, stale-while-revalidate=86400"
    );
    expect(document.querySelector("channel > link")?.textContent).toBe(absoluteUrl("/blog"));
    expect(document.getElementsByTagName("atom:link")[0]?.getAttribute("href")).toBe(
      absoluteUrl("/blog/feed.xml")
    );
    expect(items).toHaveLength(posts.length);

    const feedLinks = new Set(
      items.map(item => item.getElementsByTagName("link")[0]?.textContent ?? "")
    );
    for (const post of posts) {
      const link = absoluteUrl(post.permalink);
      expectSiteUrl(link);
      expect(feedLinks).toContain(link);
    }
  });

  it("publishes RSS channel and item metadata aligned with generated posts", async () => {
    const response = feedGET();
    const xml = await response.text();
    const document = parseXml(xml);
    const posts = allPosts();
    const channel = document.getElementsByTagName("channel")[0];
    const items = [...document.getElementsByTagName("item")];

    expect(channel).toBeTruthy();
    expect(textOf(channel, "title")).toBe(`${siteConfig.name} blog`);
    expect(textOf(channel, "description")).toBe(siteConfig.description);
    expect(textOf(channel, "language")).toBe("en-us");
    expect(items).toHaveLength(posts.length);

    items.forEach((item, index) => {
      const post = posts[index];
      const link = absoluteUrl(post.permalink);
      const guid = item.getElementsByTagName("guid")[0];
      const pubDate = textOf(item, "pubDate");
      const category = textOf(item, "category");

      expect(textOf(item, "title"), post.slug).toBe(post.title);
      expect(textOf(item, "link"), post.slug).toBe(link);
      expect(guid?.textContent, post.slug).toBe(link);
      expect(guid?.getAttribute("isPermaLink"), post.slug).toBe("true");
      expect(pubDate, post.slug).toBe(new Date(post.date).toUTCString());
      expect(Number.isNaN(Date.parse(pubDate)), post.slug).toBe(false);
      expect(textOf(item, "description"), post.slug).toBe(post.description);
      expect(category, post.slug).toBe(post.category);
      expect((BLOG_CATEGORIES as readonly string[]).includes(category), post.slug).toBe(true);
    });
  });
});
