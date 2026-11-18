// Shell of every manual page: chapter nav, in-page index with scroll-spy, prev/next, footer.
import type { ReactNode } from "react";

import type { Chapter } from "../chapters";
import { useManualBehaviors } from "../hooks/use-manual-behaviors";
import { ChapterNeighbours } from "./chapter-neighbours";
import { ManualFooter } from "./manual-footer";
import { ManualNav } from "./manual-nav";

export function ManualLayout({ page, children }: { page: Chapter; children: ReactNode }) {
  const activeSection = useManualBehaviors(page);
  return (
    <>
      <ManualNav page={page} activeSection={activeSection} />
      <main className="overflow-x-clip">{children}</main>
      <ChapterNeighbours page={page} />
      <ManualFooter />
    </>
  );
}
