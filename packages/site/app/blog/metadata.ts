import type { Metadata } from "next";
import { createPageMetadata } from "@/lib/site-config";

export const blogMetadata: Metadata = createPageMetadata({
  title: "Blog",
  description: "Field notes from the runtime, protocol design, engineering, and release receipts.",
  path: "/blog",
});
