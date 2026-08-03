import type { Metadata } from "next";
import { createPageMetadata } from "@/lib/site-config";

const baseMetadata = createPageMetadata({
  title: "Changelog",
  description:
    "Complete GitHub release notes, pull requests, contributors, and assets for CompozyOS.",
  path: "/changelog",
});

export const changelogMetadata: Metadata = {
  ...baseMetadata,
  alternates: {
    ...baseMetadata.alternates,
    types: { "application/rss+xml": "/changelog/feed.xml" },
  },
};
