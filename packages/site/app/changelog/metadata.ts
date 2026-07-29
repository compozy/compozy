import type { Metadata } from "next";
import { createPageMetadata } from "@/lib/site-config";

export const changelogMetadata: Metadata = createPageMetadata({
  title: "Changelog",
  description: "Release notes, breaking changes, and verification receipts for CompozyOS.",
  path: "/changelog",
});
