import type { Metadata } from "next/types";

export function createMetadata(override: Metadata): Metadata {
  return {
    ...override,
    openGraph: {
      title: override.title ?? undefined,
      description: override.description ?? undefined,
      url: "https://compozy.com",
      images: "/banner.png",
      siteName: "Compozy",
      ...override.openGraph,
    },
    twitter: {
      card: "summary_large_image",
      creator: "@compozyai",
      title: override.title ?? undefined,
      description: override.description ?? undefined,
      images: "/banner.png",
      ...override.twitter,
    },
  };
}

export const baseUrl =
  process.env.NODE_ENV === "development" || !process.env.VERCEL_URL
    ? new URL("http://localhost:5006")
    : new URL(`https://${process.env.VERCEL_URL}`);
