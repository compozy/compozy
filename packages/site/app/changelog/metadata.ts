import type { Metadata } from "next";
import { createPageMetadata } from "@/lib/site-config";

export const changelogMetadata: Metadata = createPageMetadata({
  title: "Changelog",
  description: "Every alpha receipt and release note for the AGH runtime and agh-network/v0.",
  path: "/changelog",
});
