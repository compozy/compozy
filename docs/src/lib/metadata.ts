import type { Metadata } from "next/types";

export function createMetadata(override: Metadata): Metadata {
  // Use absolute URL with metadataBase
  const ogImage = {
    url: "/banner.png",
    width: 1200,
    height: 630,
    alt: "Compozy - Next-level Agentic Orchestration Platform",
  };

  return {
    ...override,
    openGraph: {
      title: override.title ?? undefined,
      description: override.description ?? undefined,
      url: "https://compozy.com",
      images: [ogImage],
      siteName: "Compozy",
      ...override.openGraph,
    },
    twitter: {
      card: "summary_large_image",
      creator: "@compozyai",
      title: override.title ?? undefined,
      description: override.description ?? undefined,
      images: [ogImage.url],
      ...override.twitter,
    },
  };
}

export const baseUrl = process.env.NEXT_PUBLIC_SITE_URL
  ? new URL(process.env.NEXT_PUBLIC_SITE_URL)
  : process.env.NODE_ENV === "production"
    ? new URL("https://compozy.com")
    : new URL("http://localhost:5006");
