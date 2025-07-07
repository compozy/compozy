import type { Metadata } from "next";

export const baseUrl =
  process.env.NODE_ENV === "development" ? "http://localhost:3000" : "https://docs.dokploy.com";

export const url = (path: string): string => new URL(path, baseUrl).toString();

export function createMetadata(override: Metadata): Metadata {
  return {
    ...override,
    openGraph: {
      title: override.title ?? undefined,
      description: override.description ?? undefined,
      url: "https://fumadocs.vercel.app",
      images: "/og.png",
      siteName: "Fumadocs",
      ...override.openGraph,
    },
    twitter: {
      card: "summary_large_image",
      creator: "@money_is_shark",
      title: override.title ?? undefined,
      description: override.description ?? undefined,
      images: "/banner.png",
      ...override.twitter,
    },
  };
}
