import { render } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

const mockedPageRender = vi.hoisted(() => ({
  mastheads: [] as Array<{ pageUrl?: string; githubUrl?: string }>,
  breadcrumbs: [] as Array<{ items: Array<{ name: string; path: string }> }>,
  articles: [] as Array<{ imageUrl: string }>,
}));

const mockedDocs = vi.hoisted(() => {
  function createDocs(
    pages: Array<{
      slugs: string[];
      url: string;
      path: string;
      title: string;
      description: string;
      source?: string;
    }>
  ) {
    return {
      generateParams: () => pages.map(page => ({ slug: page.slugs })),
      getPage: (slug: string[]) => {
        const page = pages.find(candidate => candidate.slugs.join("/") === slug.join("/"));
        return page
          ? {
              ...page,
              data: {
                ...page,
                body: () => null,
                toc: [],
                // What `source.config.ts` compiles into each page module.
                _exports: { rawSource: page.source ?? "Body text" },
                getText: async () => page.source ?? "Body text",
              },
            }
          : null;
      },
    };
  }

  const docsPages = [
    {
      slugs: [],
      url: "/docs",
      path: "index.mdx",
      title: "Overview",
      description: "Understand CompozyOS and choose the right docs path.",
    },
    {
      slugs: ["how-to-use-these-docs"],
      url: "/docs/how-to-use-these-docs",
      path: "how-to-use-these-docs.mdx",
      title: "How to Use These Docs",
      description: "Choose the right CompozyOS docs path for your goal.",
    },
    {
      slugs: ["loops", "reference"],
      url: "/docs/loops/reference-model",
      path: "loops/reference-model.mdx",
      title: "Loop Reference",
      description: "Understand the public Loop reference.",
    },
    {
      slugs: ["loops", "reference", "grammar"],
      url: "/docs/loops/reference/grammar",
      path: "loops/reference/grammar.mdx",
      title: "Implementation Status",
      description: "Understand the current Loop grammar.",
    },
    {
      slugs: ["api", "sessions"],
      url: "/docs/api/sessions",
      path: "api/sessions.mdx",
      title: "Sessions",
      description: "CompozyOS Sessions HTTP endpoints.",
      source:
        '<OpenAPIPage document="compozy" operations={[{"path":"/api/sessions","method":"get"}]} />',
    },
    {
      slugs: ["sdk", "sessions"],
      url: "/docs/sdk/sessions",
      path: "sdk/sessions.mdx",
      title: "Sessions",
      description: "Methods on the Sessions resource.",
      source: '<SdkResource id="sessions" />',
    },
  ];

  return {
    docsPages,
    docsSource: createDocs(docsPages),
  };
});

vi.mock("@/lib/source", () => ({
  docsSource: mockedDocs.docsSource,
}));

vi.mock("@/components/docs/docs-masthead", () => ({
  DocsMasthead: (props: { actions: { pageUrl?: string; githubUrl?: string } }) => {
    mockedPageRender.mastheads.push(props.actions);
    return null;
  },
  DocsPager: () => null,
}));

vi.mock("@/components/docs/docs-toc", () => ({
  DocsPageFrame: ({ children }: { children?: ReactNode }) => children,
  DocsTocDisclosure: () => null,
}));

vi.mock("@/lib/docs-shell", () => {
  const shell = {
    total: mockedDocs.docsPages.length,
    sections: [],
    tabs: [],
    pages: [],
    byUrl: new Map(
      mockedDocs.docsPages.map(page => [
        page.url,
        {
          url: page.url,
          title: page.title,
          description: page.description,
          section: null,
          trail: [],
          node: { key: page.url, name: page.title, url: page.url },
        },
      ])
    ),
  };
  return {
    getDocsShell: () => shell,
    neighboursOf: () => ({ previous: null, next: null }),
  };
});

vi.mock("@/components/seo/structured-data", () => ({
  BreadcrumbListJsonLd: (props: { items: Array<{ name: string; path: string }> }) => {
    mockedPageRender.breadcrumbs.push(props);
    return null;
  },
  TechArticleJsonLd: (props: { imageUrl: string }) => {
    mockedPageRender.articles.push(props);
    return null;
  },
}));

vi.mock("@/mdx-components", () => ({
  getMDXComponents: () => ({}),
}));

vi.mock("@/lib/openapi", () => ({
  openapi: { preloadOpenAPIPage: vi.fn() },
}));

import {
  default as DocsRoutePage,
  generateMetadata as generateDocsMetadata,
  generateStaticParams as generateDocsStaticParams,
} from "@/app/docs/[[...slug]]/page";

function pageProps(slug: string[]) {
  return {
    params: Promise.resolve({ slug }),
  };
}

describe("docs route metadata", () => {
  it("generates static params for the whole /docs tree from the docs source", async () => {
    await expect(generateDocsStaticParams()).resolves.toEqual(
      mockedDocs.docsPages.map(page => ({ slug: page.slugs }))
    );
  });

  it("publishes canonical docs metadata from docs frontmatter", async () => {
    for (const page of mockedDocs.docsPages) {
      const metadata = await generateDocsMetadata(pageProps(page.slugs));

      expect(metadata.title, page.url).toBe(page.title);
      expect(metadata.description, page.url).toBe(page.description);
      expect(metadata.alternates?.canonical, page.url).toBe(`${page.url}/`);
      expect(metadata.openGraph?.title, page.url).toBe(page.title);
      expect(metadata.openGraph?.description, page.url).toBe(page.description);
      expect(metadata.openGraph?.url, page.url).toBe(`https://compozy.com${page.url}/`);
      expect(metadata.twitter?.title, page.url).toBe(page.title);
      expect(metadata.twitter?.description, page.url).toBe(page.description);
    }
  });

  it("does not publish metadata for unknown docs routes", async () => {
    await expect(generateDocsMetadata(pageProps(["missing-docs-page"]))).rejects.toThrow(
      "NEXT_HTTP_ERROR_FALLBACK;404"
    );
  });

  it("uses the docs source's resolved URL for page actions and structured metadata", async () => {
    mockedPageRender.mastheads.length = 0;
    mockedPageRender.breadcrumbs.length = 0;
    mockedPageRender.articles.length = 0;

    render(await DocsRoutePage(pageProps(["loops", "reference"])));

    expect(mockedPageRender.mastheads.at(-1)?.pageUrl).toBe(
      "https://compozy.com/docs/loops/reference-model/"
    );
    expect(mockedPageRender.breadcrumbs.at(-1)?.items).toEqual([
      { name: "Home", path: "/" },
      { name: "Docs", path: "/docs/" },
      { name: "Loops", path: "/docs/loops/" },
      { name: "Loop Reference", path: "/docs/loops/reference-model/" },
    ]);
    expect(mockedPageRender.articles.at(-1)?.imageUrl).toBe(
      "/og/docs/loops/reference-model/image.png"
    );
  });

  it("opens hand-written pages on GitHub at their own file and generated ones at their source", async () => {
    const blob = "https://github.com/compozy/compozy/blob/main";
    const cases: [string[], string][] = [
      [["loops", "reference"], `${blob}/packages/site/content/docs/loops/reference-model.mdx`],
      [["api", "sessions"], `${blob}/openapi/compozy.json`],
      [["sdk", "sessions"], `${blob}/packages/site/scripts/generate-sdk-reference.ts`],
    ];
    for (const [slug, githubUrl] of cases) {
      render(await DocsRoutePage(pageProps(slug)));
      expect(mockedPageRender.mastheads.at(-1)?.githubUrl, slug.join("/")).toBe(githubUrl);
    }
  });
});
