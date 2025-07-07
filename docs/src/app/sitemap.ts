import { source } from "@/lib/source";
import { url } from "@/utils/metadata";
import type { MetadataRoute } from "next";

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const props = await Promise.all(
    source.getPages().map(async page => {
      const { lastModified } = page.data;
      return {
        url: url(page.url),
        lastModified: lastModified ? new Date(lastModified) : undefined,
        changeFrequency: "weekly",
        priority: 0.5,
      } as MetadataRoute.Sitemap[number];
    })
  );
  return props;
}
