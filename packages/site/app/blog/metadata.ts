import type { Metadata } from "next";
import { createPageMetadata } from "@/lib/site-config";

export const blogMetadata: Metadata = createPageMetadata({
  title: "AI Agent Engineering Guides and Experiments",
  description:
    "Practical experiments, guides, and reusable tools for reviewing AI agent work, recovering interrupted tasks, and building automation you can inspect.",
  path: "/blog",
});
