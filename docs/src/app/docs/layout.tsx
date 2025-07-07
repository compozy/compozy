import { baseOptions } from "@/app/layout.config";
import { ScrollProgress } from "@/components/magicui/scroll-progress";
import { source } from "@/lib/source";
import { DocsLayout } from "fumadocs-ui/layouts/notebook";
import type { ReactNode } from "react";

export default function Layout({ children }: { children: ReactNode }) {
  return (
    <>
      <ScrollProgress />
      <DocsLayout
        {...baseOptions}
        tree={source.pageTree}
        nav={{ ...baseOptions.nav, mode: "auto" }}
        tabMode="navbar"
      >
        {children}
      </DocsLayout>
    </>
  );
}
