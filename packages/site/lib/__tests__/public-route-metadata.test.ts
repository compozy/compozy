import { describe, expect, it, vi } from "vitest";
import { GET as feedGET } from "@/app/blog/feed.xml/route";
import {
  generateStaticParams as generateLLMMarkdownStaticParams,
  GET as llmsMarkdownGET,
} from "@/app/llms.mdx/[[...slug]]/route";
import { GET as llmsGET } from "@/app/llms.txt/route";
import {
  generateStaticParams as generateOGStaticParams,
  GET as ogGET,
} from "@/app/og/[...slug]/route";
import robots from "@/app/robots";
import sitemap from "@/app/sitemap";
import { BLOG_CATEGORIES, allPosts } from "@/lib/blog";
import { absoluteUrl, canonicalPath, siteConfig } from "@/lib/site-config";
import { NextRequest } from "next/server";

const mockedDocs = vi.hoisted(() => ({
  protocolPages: [
    {
      data: { description: "Implemented protocol surface.", title: "Implementation Status" },
      slugs: ["network", "protocol", "implementation-status"],
      url: "/docs/network/protocol/implementation-status",
    },
  ],
  runtimePages: [
    {
      data: { description: "Runtime docs overview.", title: "How to use these docs" },
      slugs: ["how-to-use-these-docs"],
      url: "/docs/how-to-use-these-docs",
    },
    {
      data: {
        description: "Prepare a project workspace for agent execution.",
        title: "Prepare a project workspace",
      },
      slugs: ["use-cases", "prepare-a-project-workspace"],
      url: "/docs/use-cases/prepare-a-project-workspace",
    },
  ],
}));

const mockedOG = vi.hoisted(() => ({
  renderBlog: vi.fn(async () => new Response("blog-og")),
  renderDocs: vi.fn(async () => new Response("docs-og")),
}));

vi.mock("@/lib/og/templates/blog", () => ({
  renderBlogOG: mockedOG.renderBlog,
}));

vi.mock("@/lib/og/templates/docs", () => ({
  renderDocsOG: mockedOG.renderDocs,
}));

vi.mock("@/lib/source", () => ({
  docsSource: {
    getPages: () => [...mockedDocs.runtimePages, ...mockedDocs.protocolPages],
    generateParams: () =>
      [...mockedDocs.runtimePages, ...mockedDocs.protocolPages].map(page => ({ slug: page.slugs })),
    getPage: (slug: string[]) => {
      const page = [...mockedDocs.runtimePages, ...mockedDocs.protocolPages].find(
        candidate => candidate.slugs.join("/") === slug.join("/")
      );
      if (!page) return undefined;

      return {
        ...page,
        data: {
          ...page.data,
          content: `# ${page.data.title}`,
        },
        path: `${page.slugs.join("/")}.mdx`,
      };
    },
  },
}));

vi.mock("@/lib/marketplace-catalog", () => ({
  MARKETPLACE_KINDS: ["skills"],
  entriesForKind: (kind: string) => (kind === "skills" ? [{ entry_id: "example-skill" }] : []),
}));

vi.mock("@/lib/changelog/github-client", () => ({
  loadChangelogReleases: async () => ({
    status: "ready",
    releases: [
      {
        version: "v0.3.0-beta.1",
        summary: "First public beta.",
        body: "### Runtime\n\n- First public beta.",
        publishedAt: "2026-07-01T00:00:00Z",
        channel: "beta",
      },
    ],
  }),
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
  it("publishes canonical HTTPS sitemap entries", async () => {
    const entries = await sitemap();
    const urls = entries.map(entry => entry.url);

    expect(urls.length).toBeGreaterThan(0);
    expect(new Set(urls).size).toBe(urls.length);

    for (const url of urls) {
      const parsed = expectSiteUrl(url);
      expect(parsed.pathname).toBe(canonicalPath(parsed.pathname));
    }

    expect(urls).toContain(absoluteUrl("/"));
    expect(urls).toContain(absoluteUrl("/docs/how-to-use-these-docs/"));
    expect(urls).toContain(absoluteUrl("/docs/use-cases/prepare-a-project-workspace/"));
    expect(urls).toContain(absoluteUrl("/docs/network/protocol/implementation-status/"));
    expect(urls).toContain(absoluteUrl("/marketplace/bridges/"));
    expect(urls).toContain(absoluteUrl("/marketplace/bundled/dev-cycle/"));
    expect(urls).toContain(absoluteUrl("/marketplace/skills/example-skill/"));
    expect(urls.filter(url => url.includes("/changelog"))).toEqual([
      absoluteUrl("/changelog/"),
      absoluteUrl("/changelog/feed.xml"),
      absoluteUrl("/changelog/v0.3.0-beta.1/"),
    ]);
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

  it("publishes llms.txt with one canonical heading per public section", async () => {
    const response = await llmsGET();
    const body = await response.text();
    const headings = body.match(/^#{1,2} .+$/gm) ?? [];

    expect(response.headers.get("Content-Type")).toBe("text/plain; charset=utf-8");
    for (const heading of [
      "# CompozyOS Documentation",
      "## Docs",
      "## Guides & examples",
      "## Compozy Network",
    ]) {
      expect(headings.filter(candidate => candidate === heading)).toHaveLength(1);
    }
  });

  it("serves Markdown only from the current docs tree", async () => {
    expect(generateLLMMarkdownStaticParams()).toEqual([
      { slug: ["docs", "how-to-use-these-docs"] },
      { slug: ["docs", "use-cases", "prepare-a-project-workspace"] },
      { slug: ["docs", "network", "protocol", "implementation-status"] },
    ]);

    const response = await llmsMarkdownGET(new NextRequest("https://compozy.com/llms.mdx"), {
      params: Promise.resolve({ slug: ["docs", "how-to-use-these-docs"] }),
    });
    expect(response.headers.get("Content-Type")).toBe("text/markdown; charset=utf-8");
    await expect(response.text()).resolves.toContain("# How to use these docs");

    for (const legacySlug of [["runtime"], ["protocol"]]) {
      await expect(
        llmsMarkdownGET(new NextRequest("https://compozy.com/llms.mdx"), {
          params: Promise.resolve({ slug: legacySlug }),
        })
      ).rejects.toThrow("NEXT_HTTP_ERROR_FALLBACK;404");
    }
  });

  it("serves Open Graph images only from current public trees", async () => {
    expect(generateOGStaticParams()).toEqual(
      expect.arrayContaining([
        { slug: ["docs", "how-to-use-these-docs", "image.png"] },
        { slug: ["docs", "network", "protocol", "implementation-status", "image.png"] },
      ])
    );

    const response = await ogGET(new Request("https://compozy.com/og/docs/image.png"), {
      params: Promise.resolve({ slug: ["docs", "how-to-use-these-docs", "image.png"] }),
    });
    await expect(response.text()).resolves.toBe("docs-og");
    expect(mockedOG.renderDocs).toHaveBeenCalledWith({
      variant: "docs",
      title: "How to use these docs",
      description: "Runtime docs overview.",
      path: "docs/how-to-use-these-docs",
    });

    for (const legacyTree of ["runtime", "protocol"]) {
      await expect(
        ogGET(new Request(`https://compozy.com/og/${legacyTree}/image.png`), {
          params: Promise.resolve({ slug: [legacyTree, "how-to-use-these-docs", "image.png"] }),
        })
      ).rejects.toThrow("NEXT_HTTP_ERROR_FALLBACK;404");
    }
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
