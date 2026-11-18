import { createFileRoute } from "@tanstack/react-router";

import { Checklist } from "../manual/library/checklist";
import { Downloads } from "../manual/library/downloads";
import { Governance } from "../manual/library/governance";
import { Kit } from "../manual/library/kit";
import { References } from "../manual/library/references";
import { chapterHead } from "../manual/head";
import { ChapterPage } from "../manual/parts/chapter-page";

const DESCRIPTION = "Compozy brand files, template kit, checklist and governance.";

export const Route = createFileRoute("/library")({
  head: () => chapterHead("library", DESCRIPTION),
  component: LibraryChapter,
});

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
    >
      <Downloads />
      <Kit />
      <Checklist />
      <Governance />
      <References />
    </ChapterPage>
  );
}
