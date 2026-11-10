// One chapter route: shell + cover + body. Bodies not yet ported render the section index.
import type { ReactNode } from "react";

import { pageBySlug } from "../chapters";
import { ChapterSectionsIndex } from "./chapter-sections-index";
import { ManualLayout } from "./manual-layout";
import { PageCover } from "./page-cover";

interface ChapterPageProps {
  slug: string;
  title: ReactNode;
  intro: ReactNode;
  front?: boolean;
  /** Ported sections; omitted while the chapter's port is pending (slice S4). */
  children?: ReactNode;
}

export function ChapterPage({ slug, title, intro, front, children }: ChapterPageProps) {
  const page = pageBySlug(slug);
  return (
    <ManualLayout page={page}>
      <PageCover page={page} title={title} front={front}>
        <p className="m-0">{intro}</p>
      </PageCover>
      {children ?? <ChapterSectionsIndex page={page} />}
    </ManualLayout>
  );
}
