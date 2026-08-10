import { describe, expect, it, vi } from "vitest";

const searchApi = vi.hoisted(() => ({
  calls: [] as Array<{
    mode: string;
    options: {
      indexes: () => Promise<
        Array<{
          id: string;
          title: string;
          description?: string;
          breadcrumbs?: string[];
          structuredData: unknown;
          url: string;
          tag: string;
        }>
      >;
    };
    requests: string[];
  }>,
}));

const mockedContent = vi.hoisted(() => ({
  posts: [
    {
      slug: "posts/introducing-site-search",
      title: "Introducing Site Search",
      description:
        "Restore CompozyOS search from the home shell and the docs shell with one runtime API.",
      excerpt:
        "Search now spans runtime docs, protocol docs, blog entries, and changelog receipts.",
      toc: [
        {
          title: "Why the search broke",
          url: "#why-the-search-broke",
          items: [
            {
              title: "The client contract",
              url: "#the-client-contract",
              items: [],
            },
          ],
        },
      ],
      permalink: "/blog/introducing-site-search",
    },
  ],
  releases: [
    {
      version: "v0.3.0-beta.1",
      summary: "Runtime search now works from both the home shell and docs shell.",
      body: "### Runtime\n\nSearch spans runtime docs, protocol docs, blog posts, and changelog entries.",
      headings: [{ id: "runtime", label: "Runtime", level: 3 }],
    },
  ],
  protocolPages: [
    {
      url: "/docs/network/protocol/implementation-status",
      data: {
        title: "Implementation Status",
        description: "Understand the current compozy-network/v0 reference implementation.",
        structuredData: { headings: [{ content: "Current runtime" }] },
      },
    },
    {
      url: "/docs/network/protocol/guide/testing",
      data: {
        title: "Protocol Testing",
        description: "Test a protocol implementation against the v0 contract.",
        structuredData: { headings: [{ content: "Test matrix" }] },
      },
    },
  ],
  runtimePages: [
    {
      url: "/docs/api/sessions",
      data: {
        title: "Sessions",
        description: "CompozyOS Sessions HTTP endpoints.",
        structuredData: { headings: [{ content: "List sessions" }] },
      },
    },
    {
      url: "/docs/sessions",
      data: {
        title: "Sessions",
        description: "The durable runtime unit CompozyOS creates, attaches, audits, and governs.",
        structuredData: { headings: [{ content: "Browse the session catalog" }] },
      },
    },
    {
      url: "/docs/cli/session/resume",
      data: {
        title: "compozy session resume",
        description: "Attach to a resumable session.",
        structuredData: { headings: [{ content: "Options" }] },
      },
    },
    {
      url: "/docs/use-cases/prepare-a-project-workspace",
      data: {
        title: "Prepare a Project Workspace",
        description: "Register a project workspace and run a first smoke-test session.",
        structuredData: { headings: [{ content: "Setup" }] },
      },
    },
    {
      url: "/docs/how-to-use-these-docs",
      data: {
        title: "How to Use These Docs",
        description: "Choose the right CompozyOS documentation path for your goal.",
        structuredData: { headings: [{ content: "Choose a path" }] },
      },
    },
  ],
}));

vi.mock("@/lib/blog", () => ({
  allPosts: () => mockedContent.posts,
}));

vi.mock("@/lib/changelog/github-client", () => ({
  loadChangelogReleases: async () => ({ status: "ready", releases: mockedContent.releases }),
}));

vi.mock("@/lib/source", () => ({
  docsSource: {
    getPages: () => [...mockedContent.runtimePages, ...mockedContent.protocolPages],
    getPage: (slugs: string[]) =>
      slugs[0] === "bridges" && slugs[1] ? { url: `/docs/bridges/${slugs[1]}` } : undefined,
  },
}));

vi.mock("fumadocs-core/search/server", () => ({
  createSearchAPI: (
    mode: string,
    options: { indexes: (typeof searchApi.calls)[number]["options"]["indexes"] }
  ) => {
    const call = { mode, options, requests: [] as string[] };
    searchApi.calls.push(call);

    return {
      GET: async (request: Request) => {
        call.requests.push(request.url);
        const query = new URL(request.url).searchParams.get("query");
        const indexes = await options.indexes();

        return Response.json({
          mode: "live",
          query,
          count: indexes.length,
        });
      },
      staticGET: async () =>
        Response.json({ mode: "static", count: (await options.indexes()).length }),
    };
  },
}));

describe("public search index", () => {
  it("indexes runtime docs, protocol docs, blog posts, and changelog entries with stable route metadata", async () => {
    const { buildPublicSearchIndexes } = await import("@/lib/public-search-index");

    expect((await buildPublicSearchIndexes()).filter(index => index.tag !== "Marketplace")).toEqual(
      [
        {
          id: "/blog/introducing-site-search",
          title: "Introducing Site Search",
          description:
            "Restore CompozyOS search from the home shell and the docs shell with one runtime API.",
          breadcrumbs: ["Blog"],
          tag: "Blog",
          structuredData: {
            headings: [
              { id: "why-the-search-broke", content: "Why the search broke" },
              { id: "the-client-contract", content: "The client contract" },
            ],
            contents: [
              {
                heading: undefined,
                content:
                  "Restore CompozyOS search from the home shell and the docs shell with one runtime API.\n\nSearch now spans runtime docs, protocol docs, blog entries, and changelog receipts.",
              },
              {
                heading: "why-the-search-broke",
                content: "Why the search broke",
              },
              {
                heading: "the-client-contract",
                content: "The client contract",
              },
            ],
          },
          url: "/blog/introducing-site-search",
        },
        {
          id: "/changelog/v0.3.0-beta.1",
          title: "v0.3.0-beta.1",
          description: "Runtime search now works from both the home shell and docs shell.",
          breadcrumbs: ["Changelog"],
          tag: "Changelog",
          structuredData: {
            headings: [{ id: "runtime", content: "Runtime" }],
            contents: [
              {
                heading: undefined,
                content:
                  "Runtime search now works from both the home shell and docs shell.\n\n### Runtime\n\nSearch spans runtime docs, protocol docs, blog posts, and changelog entries.",
              },
            ],
          },
          url: "/changelog/v0.3.0-beta.1",
        },
        {
          title: "Sessions",
          description: "CompozyOS Sessions HTTP endpoints.",
          structuredData: { headings: [{ content: "List sessions" }] },
          id: "/docs/api/sessions",
          url: "/docs/api/sessions",
          breadcrumbs: ["Reference", "API reference"],
          tag: "Reference",
        },
        {
          title: "compozy session resume",
          description: "Attach to a resumable session.",
          structuredData: { headings: [{ content: "Options" }] },
          id: "/docs/cli/session/resume",
          url: "/docs/cli/session/resume",
          breadcrumbs: ["Reference", "CLI reference", "Session"],
          tag: "Reference",
        },
        {
          title: "How to Use These Docs",
          description: "Choose the right CompozyOS documentation path for your goal.",
          structuredData: { headings: [{ content: "Choose a path" }] },
          id: "/docs/how-to-use-these-docs",
          url: "/docs/how-to-use-these-docs",
          breadcrumbs: ["Docs"],
          tag: "Docs",
        },
        {
          title: "Protocol Testing",
          description: "Test a protocol implementation against the v0 contract.",
          structuredData: { headings: [{ content: "Test matrix" }] },
          id: "/docs/network/protocol/guide/testing",
          url: "/docs/network/protocol/guide/testing",
          breadcrumbs: ["Compozy Network", "Network", "Protocol spec", "Implementation guide"],
          tag: "Compozy Network",
        },
        {
          title: "Implementation Status",
          description: "Understand the current compozy-network/v0 reference implementation.",
          structuredData: { headings: [{ content: "Current runtime" }] },
          id: "/docs/network/protocol/implementation-status",
          url: "/docs/network/protocol/implementation-status",
          breadcrumbs: ["Compozy Network", "Network", "Protocol spec"],
          tag: "Compozy Network",
        },
        {
          title: "Sessions",
          description: "The durable runtime unit CompozyOS creates, attaches, audits, and governs.",
          structuredData: { headings: [{ content: "Browse the session catalog" }] },
          id: "/docs/sessions",
          url: "/docs/sessions",
          breadcrumbs: ["Core concepts"],
          tag: "Core concepts",
        },
        {
          title: "Prepare a Project Workspace",
          description: "Register a project workspace and run a first smoke-test session.",
          structuredData: { headings: [{ content: "Setup" }] },
          id: "/docs/use-cases/prepare-a-project-workspace",
          url: "/docs/use-cases/prepare-a-project-workspace",
          breadcrumbs: ["Guides & examples", "Use cases"],
          tag: "Guides & examples",
        },
      ]
    );
  });

  it("distinguishes identical document titles by their URL-derived section", async () => {
    const { buildPublicSearchIndexes } = await import("@/lib/public-search-index");

    const sessions = (await buildPublicSearchIndexes())
      .filter(index => index.title === "Sessions")
      .map(index => ({
        breadcrumbs: index.breadcrumbs,
        url: index.url,
      }));

    expect(sessions).toEqual([
      {
        breadcrumbs: ["Reference", "API reference"],
        url: "/docs/api/sessions",
      },
      {
        breadcrumbs: ["Core concepts"],
        url: "/docs/sessions",
      },
    ]);
  });

  it("indexes every marketplace surface under the Marketplace group", async () => {
    const { buildPublicSearchIndexes } = await import("@/lib/public-search-index");
    const { entriesForKind, MARKETPLACE_KINDS } = await import("@/lib/marketplace-catalog");
    const { bridgeProviders } = await import("@/lib/marketplace-bridges");

    const marketplace = (await buildPublicSearchIndexes()).filter(
      index => index.tag === "Marketplace"
    );
    const urls = new Set(marketplace.map(index => index.url));

    expect(urls.has("/marketplace")).toBe(true);
    for (const kind of MARKETPLACE_KINDS) {
      expect(urls.has(`/marketplace/${kind}`)).toBe(true);
      for (const entry of entriesForKind(kind)) {
        expect(urls.has(`/marketplace/${kind}/${entry.entry_id}`)).toBe(true);
      }
    }
    expect(urls.has("/marketplace/bridges")).toBe(true);
    for (const provider of bridgeProviders) {
      expect(urls.has(`/marketplace/bridges#${provider.platform}`)).toBe(true);
    }
    expect(urls.has("/marketplace/bundled/dev-cycle")).toBe(true);
  });

  it("wires the live GET handler instead of the static export handler", async () => {
    const route = await import("@/app/api/search/route");
    expect(searchApi.calls).toHaveLength(1);
    expect(searchApi.calls[0]?.mode).toBe("advanced");

    const response = await route.GET(new Request("https://compozy.com/api/search?query=search"));

    await expect(response.json()).resolves.toMatchObject({
      mode: "live",
      query: "search",
    });
    expect(searchApi.calls[0]?.requests).toEqual(["https://compozy.com/api/search?query=search"]);
  });
});
