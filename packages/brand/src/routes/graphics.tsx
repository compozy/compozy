import { createFileRoute } from "@tanstack/react-router";

import { Diagrams } from "../manual/graphics/diagrams";
import { IconSet } from "../manual/graphics/icon-set";
import { Marks } from "../manual/graphics/marks";
import { Patterns } from "../manual/graphics/patterns";
import { ProductUi } from "../manual/graphics/product-ui";
import { Shapes } from "../manual/graphics/shapes";
import { chapterHead } from "../manual/head";
import { ChapterPage } from "../manual/parts/chapter-page";

const DESCRIPTION =
  "Compozy graphic system: shapes from the visual system, our UI as image, diagrams, icons, marks and patterns.";

export const Route = createFileRoute("/graphics")({
  head: () => chapterHead("graphics", DESCRIPTION),
  component: GraphicsChapter,
});

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
    >
      <Shapes />
      <ProductUi />
      <Diagrams />
      <IconSet />
      <Marks />
      <Patterns />
    </ChapterPage>
  );
}
