import { createFileRoute } from "@tanstack/react-router";

import { chapterHead } from "../manual/head";
import { ChapterPage } from "../manual/parts/chapter-page";

const DESCRIPTION = "Compozy brand files, template kit, checklist and governance.";

export const Route = createFileRoute("/library")({
  head: () => chapterHead("library", DESCRIPTION),
  component: LibraryChapter,
});

// Cover and intro are ported; the sections arrive with slice S4.
function LibraryChapter() {
  return (
    <ChapterPage
      slug="library"
      title={
        <>
          Everything you need to <em>ship</em> a piece.
        </>
      }
      intro="Official files, the template kit with native export, the pre-publish checklist, and how this manual changes."
    />
  );
}
