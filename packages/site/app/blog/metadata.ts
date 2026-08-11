import type { Metadata } from "next";
import { createPageMetadata } from "@/lib/site-config";

export const blogMetadata: Metadata = createPageMetadata({
  title: "Blog",
  description:
    "Field notes on building, operating, and extending CompozyOS: loops, automation, memory, permissions, and the runtime behind them.",
  path: "/blog",
});
