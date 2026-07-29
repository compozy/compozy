import { describe, expect, it, vi } from "vitest";

const searchApi = vi.hoisted(() => ({
  calls: [] as Array<{
    mode: string;
    options: {
      indexes: Array<{
        id: string;
        title: string;
        description?: string;
        breadcrumbs?: string[];
        structuredData: unknown;
        url: string;
        tag: string;
      }>;
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
        "Restore Compozy search from the home shell and the docs shell with one runtime API.",
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
      version: "v0.1.0-alpha.1",
      summary: "Runtime search now works from both the home shell and docs shell.",
      added: ["Search spans runtime docs, protocol docs, blog posts, and changelog entries."],
      changed: ["The site now uses standard Next.js runtime output instead of static export."],
      fixed: ["The Fumadocs fetch client once again talks to a live query endpoint."],
      breaking: [],
    },
  ],
  protocolPages: [
    {
      url: "/protocol/implementation-status",
      data: {
        title: "Implementation Status",
        description: "Understand the current compozy-network/v0 reference implementation.",
        structuredData: { headings: [{ content: "Current runtime" }] },
      },
    },
    {
      url: "/protocol/guide/testing",
      data: {
        title: "Protocol Testing",
        description: "Test a protocol implementation against the v0 contract.",
        structuredData: { headings: [{ content: "Test matrix" }] },
      },
    },
  ],
  runtimePages: [
    {
      url: "/runtime/api-reference/sessions",
      data: {
        title: "Sessions",
        description: "Compozy Sessions HTTP endpoints.",
        structuredData: { headings: [{ content: "List sessions" }] },
      },
    },
    {
      url: "/runtime/core/sessions",
      data: {
        title: "Sessions",
        description: "The durable runtime unit Compozy creates, attaches, audits, and governs.",
        structuredData: { headings: [{ content: "Browse the session catalog" }] },
      },
    },
    {
      url: "/runtime/cli-reference/session/resume",
      data: {
        title: "compozy session resume",
        description: "Attach to a resumable session.",
        structuredData: { headings: [{ content: "Options" }] },
      },
    },
    {
      url: "/runtime/use-cases/prepare-a-project-workspace",
      data: {
        title: "Prepare a Project Workspace",
        description: "Register a project workspace and run a first smoke-test session.",
        structuredData: { headings: [{ content: "Setup" }] },
      },
    },
    {
      url: "/runtime/how-to-use-these-docs",
      data: {
        title: "How to Use These Docs",
        description: "Choose the right Compozy documentation path for your goal.",
        structuredData: { headings: [{ content: "Choose a path" }] },
      },
    },
  ],
}));

vi.mock("@/lib/blog", () => ({
  allPosts: () => mockedContent.posts,
  allReleases: () => mockedContent.releases,
}));

vi.mock("@/lib/source", () => ({
  protocolDocs: {
    getPages: () => mockedContent.protocolPages,
  },
  runtimeDocs: {
    getPages: () => mockedContent.runtimePages,
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
      GET: (request: Request) => {
        call.requests.push(request.url);
        const query = new URL(request.url).searchParams.get("query");

        return Response.json({
          mode: "live",
          query,
          count: options.indexes.length,
        });
      },
      staticGET: () => Response.json({ mode: "static", count: options.indexes.length }),
    };
  },
}));

describe("public search index", () => {
  it("indexes runtime docs, protocol docs, blog posts, and changelog entries with stable route metadata", async () => {
    const { buildPublicSearchIndexes } = await import("@/lib/public-search-index");

    expect(buildPublicSearchIndexes()).toEqual([
      {
        id: "/blog/introducing-site-search",
        title: "Introducing Site Search",
        description:
          "Restore Compozy search from the home shell and the docs shell with one runtime API.",
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
                "Restore Compozy search from the home shell and the docs shell with one runtime API.\n\nSearch now spans runtime docs, protocol docs, blog entries, and changelog receipts.",
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
        id: "/changelog#v0.1.0-alpha.1",
        title: "v0.1.0-alpha.1",
        description: "Runtime search now works from both the home shell and docs shell.",
        breadcrumbs: ["Changelog"],
        tag: "Changelog",
        structuredData: {
          headings: [
            { id: "summary", content: "Summary" },
            { id: "added", content: "Added" },
            { id: "changed", content: "Changed" },
            { id: "fixed", content: "Fixed" },
          ],
          contents: [
            {
              heading: "summary",
              content: "Runtime search now works from both the home shell and docs shell.",
            },
            {
              heading: "added",
              content:
                "Search spans runtime docs, protocol docs, blog posts, and changelog entries.",
            },
            {
              heading: "changed",
              content:
                "The site now uses standard Next.js runtime output instead of static export.",
            },
            {
              heading: "fixed",
              content: "The Fumadocs fetch client once again talks to a live query endpoint.",
            },
          ],
        },
        url: "/changelog#v0.1.0-alpha.1",
      },
      {
        title: "Protocol Testing",
        description: "Test a protocol implementation against the v0 contract.",
        structuredData: { headings: [{ content: "Test matrix" }] },
        id: "/protocol/guide/testing",
        url: "/protocol/guide/testing",
        breadcrumbs: ["Compozy Network", "Implementation guide"],
        tag: "Compozy Network",
      },
      {
        title: "Implementation Status",
        description: "Understand the current compozy-network/v0 reference implementation.",
        structuredData: { headings: [{ content: "Current runtime" }] },
        id: "/protocol/implementation-status",
        url: "/protocol/implementation-status",
        breadcrumbs: ["Compozy Network"],
        tag: "Compozy Network",
      },
      {
        title: "Sessions",
        description: "Compozy Sessions HTTP endpoints.",
        structuredData: { headings: [{ content: "List sessions" }] },
        id: "/runtime/api-reference/sessions",
        url: "/runtime/api-reference/sessions",
        breadcrumbs: ["CompozyOS", "API reference"],
        tag: "Runtime",
      },
      {
        title: "compozy session resume",
        description: "Attach to a resumable session.",
        structuredData: { headings: [{ content: "Options" }] },
        id: "/runtime/cli-reference/session/resume",
        url: "/runtime/cli-reference/session/resume",
        breadcrumbs: ["CompozyOS", "CLI reference", "Session"],
        tag: "Runtime",
      },
      {
        title: "Sessions",
        description: "The durable runtime unit Compozy creates, attaches, audits, and governs.",
        structuredData: { headings: [{ content: "Browse the session catalog" }] },
        id: "/runtime/core/sessions",
        url: "/runtime/core/sessions",
        breadcrumbs: ["CompozyOS", "Core concepts"],
        tag: "Runtime",
      },
      {
        title: "How to Use These Docs",
        description: "Choose the right Compozy documentation path for your goal.",
        structuredData: { headings: [{ content: "Choose a path" }] },
        id: "/runtime/how-to-use-these-docs",
        url: "/runtime/how-to-use-these-docs",
        breadcrumbs: ["CompozyOS"],
        tag: "Runtime",
      },
      {
        title: "Prepare a Project Workspace",
        description: "Register a project workspace and run a first smoke-test session.",
        structuredData: { headings: [{ content: "Setup" }] },
        id: "/runtime/use-cases/prepare-a-project-workspace",
        url: "/runtime/use-cases/prepare-a-project-workspace",
        breadcrumbs: ["CompozyOS", "Use cases"],
        tag: "Runtime",
      },
    ]);
  });

  it("distinguishes identical document titles by their URL-derived section", async () => {
    const { buildPublicSearchIndexes } = await import("@/lib/public-search-index");

    const sessions = buildPublicSearchIndexes()
      .filter(index => index.title === "Sessions")
      .map(index => ({
        breadcrumbs: index.breadcrumbs,
        url: index.url,
      }));

    expect(sessions).toEqual([
      {
        breadcrumbs: ["CompozyOS", "API reference"],
        url: "/runtime/api-reference/sessions",
      },
      {
        breadcrumbs: ["CompozyOS", "Core concepts"],
        url: "/runtime/core/sessions",
      },
    ]);
  });

  it("wires the live GET handler instead of the static export handler", async () => {
    const route = await import("@/app/api/search/route");

    expect(searchApi.calls).toHaveLength(1);
    expect(searchApi.calls[0]?.mode).toBe("advanced");

    const response = await route.GET(new Request("https://compozy.com/api/search?query=search"));

    await expect(response.json()).resolves.toEqual({
      mode: "live",
      query: "search",
      count: 9,
    });
    expect(searchApi.calls[0]?.requests).toEqual(["https://compozy.com/api/search?query=search"]);
  });
});
