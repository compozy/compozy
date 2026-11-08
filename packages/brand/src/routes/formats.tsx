import { createFileRoute } from "@tanstack/react-router";

import { chapterHead } from "../manual/head";
import { ChapterPage } from "../manual/parts/chapter-page";

const DESCRIPTION =
  "Compozy formats: every canvas, the grid, platform safe areas, composition and the three surfaces.";

export const Route = createFileRoute("/formats")({
  head: () => chapterHead("formats", DESCRIPTION),
  component: FormatsChapter,
});

// Cover and intro are ported; the sections arrive with slice S4.
function FormatsChapter() {
  return (
    <ChapterPage
      slug="formats"
      title={
        <>
          Every piece starts on the right <em>canvas</em>.
        </>
      }
      intro="Sizes, margins, columns and safe areas for every channel, and the three surfaces a piece can stand on. Decide these first; the design comes after."
    />
  );
}
