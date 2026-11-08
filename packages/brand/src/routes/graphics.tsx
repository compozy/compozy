import { createFileRoute } from "@tanstack/react-router";

import { chapterHead } from "../manual/head";
import { ChapterPage } from "../manual/parts/chapter-page";

const DESCRIPTION =
  "Compozy graphic system: shapes from the visual system, our UI as image, diagrams, icons, marks and patterns.";

export const Route = createFileRoute("/graphics")({
  head: () => chapterHead("graphics", DESCRIPTION),
  component: GraphicsChapter,
});

// Cover and intro are ported; the sections arrive with slice S4.
function GraphicsChapter() {
  return (
    <ChapterPage
      slug="graphics"
      title={
        <>
          Built from points, lines and our <em>interface</em>
        </>
      }
      intro="The parts that sit between the logo and a finished piece: the shapes the visuals are made of, how our interface is shown as an image, wireframe diagrams, a small stroke icon set, the marks that organise a page and the patterns behind it."
    />
  );
}
