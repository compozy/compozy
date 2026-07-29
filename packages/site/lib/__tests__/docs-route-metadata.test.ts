import { describe, expect, it, vi } from "vitest";

const mockedDocs = vi.hoisted(() => {
  function createDocs(
    pages: Array<{
      slugs: string[];
      url: string;
      title: string;
      description: string;
    }>
  ) {
    return {
      generateParams: () => pages.map(page => ({ slug: page.slugs })),
      getPage: (slug: string[]) => {
        const page = pages.find(candidate => candidate.slugs.join("/") === slug.join("/"));
        return page ? { ...page, data: page } : null;
      },
    };
  }

  const docsPages = [
    {
      slugs: [],
      url: "/docs",
      title: "Overview",
      description: "Understand CompozyOS and choose the right docs path.",
    },
    {
      slugs: ["how-to-use-these-docs"],
      url: "/docs/how-to-use-these-docs",
      title: "How to Use These Docs",
      description: "Choose the right CompozyOS docs path for your goal.",
    },
    {
      slugs: ["network", "protocol"],
      url: "/docs/network/protocol",
      title: "Compozy Network Protocol",
      description: "Understand the public compozy-network/v0 protocol surface.",
    },
    {
      slugs: ["network", "protocol", "implementation-status"],
      url: "/docs/network/protocol/implementation-status",
      title: "Implementation Status",
      description: "Understand the current compozy-network/v0 reference implementation.",
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

import {
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
});
