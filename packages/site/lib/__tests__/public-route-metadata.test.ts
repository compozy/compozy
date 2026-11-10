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
import { BLOG_CATEGORIES, allPosts, postsByCategory } from "@/lib/blog";
import { absoluteUrl, canonicalPath, siteConfig } from "@/lib/site-config";
import { NextRequest } from "next/server";

type MockedDocsPage = {
  data: { description: string; title: string };
  slugs: string[];
  url: string;
  /** The page's processed Markdown; defaults to an H1 of its title. */
  body?: string;
  /** The authored MDX the build compiles into the page module (generated API pages). */
  rawSource?: string;
};

const mockedDocs = vi.hoisted(
  (): { protocolPages: MockedDocsPage[]; runtimePages: MockedDocsPage[] } => ({
    protocolPages: [
      {
        data: { description: "Implemented protocol surface.", title: "Implementation Status" },
        slugs: ["loops", "reference", "grammar"],
        url: "/docs/loops/reference/grammar",
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
  })
);

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
          // Only the processed text is what the llms output may read.
          getText: async (type: string) =>
            type === "processed" ? (page.body ?? `# ${page.data.title}`) : "RAW MDX",
          _exports: { rawSource: page.rawSource },
          info: { fullPath: `${page.slugs.join("/")}.mdx` },
        },
        path: `${page.slugs.join("/")}.mdx`,
      };
    },
  },
}));

vi.mock("@/lib/marketplace-catalog", () => ({
  extensionEntries: [{ entry_id: "example-extension" }],
  marketplaceEntryPath: (entry: { entry_id: string }) => `/marketplace/${entry.entry_id}`,
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
    expect(urls).toContain(absoluteUrl("/docs/loops/reference/grammar/"));
    expect(urls).not.toContain(absoluteUrl("/marketplace/bridges/"));
    expect(urls).toContain(absoluteUrl("/marketplace/bundled/spec-cycle/"));
    expect(urls).toContain(absoluteUrl("/marketplace/example-extension/"));
    const { marketplacePluginPath, marketplacePlugins } = await import("@/lib/marketplace-plugins");
    for (const entry of marketplacePlugins) {
      expect(urls).toContain(absoluteUrl(`${marketplacePluginPath(entry)}/`));
    }
    expect(urls.filter(url => url.includes("/changelog"))).toEqual([
      absoluteUrl("/changelog/"),
      absoluteUrl("/changelog/feed.xml"),
      absoluteUrl("/changelog/v0.3.0-beta.1/"),
    ]);
    for (const category of BLOG_CATEGORIES) {
      expect(urls.includes(absoluteUrl(`/blog/categories/${category}/`)), category).toBe(
        postsByCategory(category).length > 0
      );
    }
    for (const post of allPosts()) {
      expect(
        entries.find(entry => entry.url === absoluteUrl(canonicalPath(post.permalink)))
      ).toMatchObject({
        lastModified: post.updated ?? post.date,
      });
    }
    expect(entries.find(entry => entry.url === absoluteUrl("/"))?.lastModified).toBeUndefined();
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
    for (const heading of ["# CompozyOS Documentation", "## Docs", "## Guides & examples"]) {
      expect(headings.filter(candidate => candidate === heading)).toHaveLength(1);
    }
  });

  it("serves Markdown only from the current docs tree", async () => {
    expect(generateLLMMarkdownStaticParams()).toEqual([
      { slug: ["docs", "how-to-use-these-docs"] },
      { slug: ["docs", "use-cases", "prepare-a-project-workspace"] },
      { slug: ["docs", "loops", "reference", "grammar"] },
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

  async function llmsMarkdown(pages: MockedDocsPage[], slug: string[]): Promise<string> {
    mockedDocs.runtimePages.push(...pages);
    try {
      const response = await llmsMarkdownGET(new NextRequest("https://compozy.com/llms.mdx"), {
        params: Promise.resolve({ slug: ["docs", ...slug] }),
      });
      return await response.text();
    } finally {
      mockedDocs.runtimePages.splice(-pages.length);
    }
  }

  it("serves a hand-written page as its title, URL, description and processed Markdown", async () => {
    const markdown = await llmsMarkdown(
      [
        {
          data: { description: "Start, stop and resume sessions.", title: "Session lifecycle" },
          slugs: ["sessions", "lifecycle"],
          url: "/docs/sessions/lifecycle",
          body: "## Start\n\nRun `compozy session start`.",
        },
      ],
      ["sessions", "lifecycle"]
    );
    expect(markdown).toBe(
      [
        "# Session lifecycle",
        `URL: ${absoluteUrl("/docs/sessions/lifecycle")}`,
        "",
        "Start, stop and resume sessions.",
        "",
        "## Start\n\nRun `compozy session start`.",
      ].join("\n")
    );
  });

  it("serves an SDK guide's per-language content under language labels, without its tags", async () => {
    const markdown = await llmsMarkdown(
      [
        {
          data: { description: "Get a connected client.", title: "Client" },
          slugs: ["sdk", "client"],
          url: "/docs/sdk/client",
          body: [
            "<SdkLangBar />",
            "",
            "## Install",
            "",
            '<SdkLang lang="ts">',
            "",
            "```bash\nnpm install @compozy/client\n```",
            "",
            "</SdkLang>",
            "",
            '<SdkSymbol lang="ts">',
            "",
            '<SdkFields title="Options">',
            '  <SdkField name="baseUrl" type="string" required>',
            "    Selects a daemon",
            "    ahead of discovery.",
            "  </SdkField>",
            '  <SdkField name="onVersionMismatch" type="(versions: { daemon: string }) => void" />',
            '  <SdkField name="mode" type={\'"queue" | "steer"\'} />',
            "</SdkFields>",
            "",
            "</SdkSymbol>",
            "",
            '<SdkSymbol lang="go" none />',
          ].join("\n"),
        },
      ],
      ["sdk", "client"]
    );
    expect(markdown).not.toMatch(/<\/?Sdk/);
    expect(markdown).toContain(
      "## Install\n\n**TypeScript**\n\n```bash\nnpm install @compozy/client\n```"
    );
    expect(markdown).toContain(
      [
        "Options:",
        "",
        "- `baseUrl` (`string`, required): Selects a daemon ahead of discovery.",
        "- `onVersionMismatch` (`(versions: { daemon: string }) => void`)",
        '- `mode` (`"queue" | "steer"`)',
      ].join("\n")
    );
    expect(markdown).toContain("**Go**\n\nTypeScript only. The Go client has no equivalent.");
  });

  it("serves a generated API page as its operations, without the generator's comment", async () => {
    const comment = "{/* This file was generated by Fumadocs. Do not edit this file directly. */}";
    const operations = [
      { path: "/api/agent/spawn", method: "post" },
      { path: "/api/sessions/{session_id}", method: "get" },
    ];
    const markdown = await llmsMarkdown(
      [
        {
          data: { description: "CompozyOS Sessions HTTP endpoints.", title: "Sessions" },
          slugs: ["api", "sessions"],
          url: "/docs/api/sessions",
          body: `${comment}\n`,
          rawSource: `${comment}\n\n<OpenAPIPage document="compozy" operations={${JSON.stringify(operations)}} />`,
        },
      ],
      ["api", "sessions"]
    );
    expect(markdown).not.toContain("generated by Fumadocs");
    expect(markdown).not.toContain("RAW MDX");
    const spawn = markdown.indexOf("`POST /api/agent/spawn`");
    const snapshot = markdown.indexOf("`GET /api/sessions/{session_id}`");
    expect(spawn).toBeGreaterThan(0);
    expect(snapshot).toBeGreaterThan(spawn);
    expect(markdown).toContain("## Spawn a narrowed child session for the calling agent");
    expect(markdown).toContain(
      "- `X-Compozy-Session-ID` (header, `string`, required): Daemon-issued active session id"
    );
    expect(markdown).toContain("Request body (application/json, `AgentSpawnRequest`, required):");
    expect(markdown).toContain("- `agent_name` (`string`, required)");
    expect(markdown).toContain("- `session_id` (path, `string`, required)");
  });

  it("lists SDK pages in llms.txt and serves a resource page's methods as Markdown", async () => {
    const pages = [
      {
        data: { description: "Typed TypeScript and Go clients.", title: "SDK Reference" },
        slugs: ["sdk"],
        url: "/docs/sdk",
      },
      {
        data: { description: "98 methods on the Sessions resource.", title: "Sessions" },
        slugs: ["sdk", "sessions"],
        url: "/docs/sdk/sessions",
        body: '<SdkResource id="sessions" />',
      },
    ];
    mockedDocs.runtimePages.push(...pages);
    try {
      const index = await (await llmsGET()).text();
      const reference = index.slice(index.indexOf("## Reference"));
      expect(reference).toContain(`[SDK Reference](${absoluteUrl("/docs/sdk")})`);
      expect(reference).toContain(`[Sessions](${absoluteUrl("/docs/sdk/sessions")})`);

      const response = await llmsMarkdownGET(new NextRequest("https://compozy.com/llms.mdx"), {
        params: Promise.resolve({ slug: ["docs", "sdk", "sessions"] }),
      });
      const markdown = await response.text();
      expect(markdown).not.toContain("<SdkResource");
      expect(markdown).toContain("## listSessions");
      expect(markdown).toContain("`compozy.sessions.listSessions({ query })`");
      expect(markdown).toContain("`c.ListSessions(ctx, ListSessionsParams)`");
      expect(markdown).toContain("## prompt");
    } finally {
      mockedDocs.runtimePages.splice(-pages.length);
    }
  });

  it("serves Open Graph images only from current public trees", async () => {
    expect(generateOGStaticParams()).toEqual(
      expect.arrayContaining([
        { slug: ["docs", "how-to-use-these-docs", "image.png"] },
        { slug: ["docs", "loops", "reference", "grammar", "image.png"] },
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
    expect(document.querySelector("channel > link")?.textContent).toBe(absoluteUrl("/blog/"));
    expect(document.getElementsByTagName("atom:link")[0]?.getAttribute("href")).toBe(
      absoluteUrl("/blog/feed.xml")
    );
    expect(items).toHaveLength(posts.length);

    const feedLinks = new Set(
      items.map(item => item.getElementsByTagName("link")[0]?.textContent ?? "")
    );
    for (const post of posts) {
      const link = absoluteUrl(canonicalPath(post.permalink));
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
      const link = absoluteUrl(canonicalPath(post.permalink));
      const guid = item.getElementsByTagName("guid")[0];
      const pubDate = textOf(item, "pubDate");
      const category = textOf(item, "category");

      expect(textOf(item, "title"), post.slug).toBe(post.title);
      expect(textOf(item, "link"), post.slug).toBe(link);
      expect(guid?.textContent, post.slug).toBe(absoluteUrl(post.permalink));
      expect(guid?.getAttribute("isPermaLink"), post.slug).toBe("true");
      expect(pubDate, post.slug).toBe(new Date(post.date).toUTCString());
      expect(Number.isNaN(Date.parse(pubDate)), post.slug).toBe(false);
      expect(textOf(item, "description"), post.slug).toBe(post.description);
      expect(category, post.slug).toBe(post.category);
      expect((BLOG_CATEGORIES as readonly string[]).includes(category), post.slug).toBe(true);
    });
  });
});
