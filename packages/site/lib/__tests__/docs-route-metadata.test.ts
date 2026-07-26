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

  const runtimePages = [
    {
      slugs: [],
      url: "/runtime",
      title: "Runtime Overview",
      description: "Understand AGH Runtime and choose the right operator path.",
    },
    {
      slugs: ["how-to-use-these-docs"],
      url: "/runtime/how-to-use-these-docs",
      title: "How to Use These Docs",
      description: "Choose the right AGH Runtime docs path for your goal.",
    },
  ];
  const protocolPages = [
    {
      slugs: [],
      url: "/protocol",
      title: "AGH Network Protocol",
      description: "Understand the public agh-network/v0 protocol surface.",
    },
    {
      slugs: ["implementation-status"],
      url: "/protocol/implementation-status",
      title: "Implementation Status",
      description: "Understand what agh-network/v0 implements in the alpha runtime.",
    },
  ];

  return {
    protocolDocs: createDocs(protocolPages),
    protocolPages,
    runtimeDocs: createDocs(runtimePages),
    runtimePages,
  };
});

vi.mock("@/lib/source", () => ({
  protocolDocs: mockedDocs.protocolDocs,
  runtimeDocs: mockedDocs.runtimeDocs,
}));

import {
  generateMetadata as generateProtocolMetadata,
  generateStaticParams as generateProtocolStaticParams,
} from "@/app/protocol/[[...slug]]/page";
import {
  generateMetadata as generateRuntimeMetadata,
  generateStaticParams as generateRuntimeStaticParams,
} from "@/app/runtime/[[...slug]]/page";

function pageProps(slug: string[]) {
  return {
    params: Promise.resolve({ slug }),
  };
}

describe("docs route metadata", () => {
  it("generates runtime and protocol static params from the docs source", async () => {
    await expect(generateRuntimeStaticParams()).resolves.toEqual(
      mockedDocs.runtimePages.map(page => ({ slug: page.slugs }))
    );
    await expect(generateProtocolStaticParams()).resolves.toEqual(
      mockedDocs.protocolPages.map(page => ({ slug: page.slugs }))
    );
  });

  it("publishes canonical runtime metadata from docs frontmatter", async () => {
    for (const page of mockedDocs.runtimePages) {
      const metadata = await generateRuntimeMetadata(pageProps(page.slugs));

      expect(metadata.title, page.url).toBe(page.title);
      expect(metadata.description, page.url).toBe(page.description);
      expect(metadata.alternates?.canonical, page.url).toBe(`${page.url}/`);
      expect(metadata.openGraph?.title, page.url).toBe(page.title);
      expect(metadata.openGraph?.description, page.url).toBe(page.description);
      expect(metadata.openGraph?.url, page.url).toBe(`https://agh.network${page.url}/`);
      expect(metadata.twitter?.title, page.url).toBe(page.title);
      expect(metadata.twitter?.description, page.url).toBe(page.description);
    }
  });

  it("publishes canonical protocol metadata from docs frontmatter", async () => {
    for (const page of mockedDocs.protocolPages) {
      const metadata = await generateProtocolMetadata(pageProps(page.slugs));

      expect(metadata.title, page.url).toBe(page.title);
      expect(metadata.description, page.url).toBe(page.description);
      expect(metadata.alternates?.canonical, page.url).toBe(`${page.url}/`);
      expect(metadata.openGraph?.title, page.url).toBe(page.title);
      expect(metadata.openGraph?.description, page.url).toBe(page.description);
      expect(metadata.openGraph?.url, page.url).toBe(`https://agh.network${page.url}/`);
      expect(metadata.twitter?.title, page.url).toBe(page.title);
      expect(metadata.twitter?.description, page.url).toBe(page.description);
    }
  });

  it("does not publish metadata for unknown docs routes", async () => {
    await expect(generateRuntimeMetadata(pageProps(["missing-runtime-page"]))).rejects.toThrow(
      "NEXT_HTTP_ERROR_FALLBACK;404"
    );
    await expect(generateProtocolMetadata(pageProps(["missing-protocol-page"]))).rejects.toThrow(
      "NEXT_HTTP_ERROR_FALLBACK;404"
    );
  });
});
