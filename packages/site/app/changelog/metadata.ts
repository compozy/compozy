import type { Metadata } from "next";
import { createPageMetadata } from "@/lib/site-config";

export const changelogMetadata: Metadata = createPageMetadata({
  title: "Changelog",
  description:
    "Every alpha receipt and release note for the Compozy runtime and compozy-network/v0.",
  path: "/changelog",
});
