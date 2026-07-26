import { describe, expect, it } from "vitest";
import { generateMetadata, generateStaticParams } from "@/app/blog/categories/[category]/page";
import { categoryLabel } from "@/components/blog/format";
import { BLOG_CATEGORIES } from "../blog";

function pageProps(category: string) {
  return {
    params: Promise.resolve({ category }),
  };
}

describe("blog category routes", () => {
  it("generates a static route for every public blog category", () => {
    expect(generateStaticParams()).toEqual(BLOG_CATEGORIES.map(category => ({ category })));
  });

  it("publishes canonical metadata for every category archive", async () => {
    for (const category of BLOG_CATEGORIES) {
      const label = categoryLabel(category);
      const metadata = await generateMetadata(pageProps(category));

      expect(metadata.title, category).toBe(`${label} posts`);
      expect(metadata.description, category).toBe(`Posts filed under ${label}.`);
      expect(metadata.alternates?.canonical, category).toBe(`/blog/categories/${category}/`);
      expect(metadata.openGraph?.title, category).toBe(`${label} posts`);
      expect(metadata.openGraph?.description, category).toBe(`Posts filed under ${label}.`);
      expect(metadata.openGraph?.url, category).toBe(
        `https://agh.network/blog/categories/${category}/`
      );
      expect(metadata.twitter?.title, category).toBe(`${label} posts`);
      expect(metadata.twitter?.description, category).toBe(`Posts filed under ${label}.`);
    }
  });

  it("does not publish metadata for unknown categories", async () => {
    const metadata = await generateMetadata(pageProps("unknown-category" satisfies string));

    expect(metadata).toEqual({});
    expect((BLOG_CATEGORIES as readonly string[]).includes("unknown-category")).toBe(false);
  });

  it("keeps category route slugs stable and URL-safe", () => {
    for (const category of BLOG_CATEGORIES) {
      expect(category, category).toMatch(/^[a-z0-9]+(?:-[a-z0-9]+)*$/);
      expect(categoryLabel(category), category).not.toBe(category);
    }
  });
});
