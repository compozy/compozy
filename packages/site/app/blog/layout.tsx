import type { ReactNode } from "react";

import "@/components/blog/blog.css";
import { SiteShell } from "@/components/site/site-shell";

export default function BlogLayout({ children }: { children: ReactNode }) {
  return <SiteShell>{children}</SiteShell>;
}
