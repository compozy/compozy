import type { Metadata } from "next";
import { createPageMetadata } from "@/lib/site-config";

export const blogMetadata: Metadata = createPageMetadata({
  title: "Blog",
  description:
    "Field notes on building, operating, extending, and releasing the integrated CompozyOS agent operating system.",
  path: "/blog",
});
