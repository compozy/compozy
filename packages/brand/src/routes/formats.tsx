import { createFileRoute } from "@tanstack/react-router";

import { Canvas } from "../manual/formats/canvas";
import { Composition } from "../manual/formats/composition";
import { Grid } from "../manual/formats/grid";
import { SafeAreas } from "../manual/formats/safe-areas";
import { Surfaces } from "../manual/formats/surfaces";
import { chapterHead } from "../manual/head";
import { ChapterPage } from "../manual/parts/chapter-page";

const DESCRIPTION =
  "Compozy formats: every canvas, the grid, platform safe areas, composition and the three surfaces.";

export const Route = createFileRoute("/formats")({
  head: () => chapterHead("formats", DESCRIPTION),
  component: FormatsChapter,
});

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
    >
      <Canvas />
      <Grid />
      <SafeAreas />
      <Composition />
      <Surfaces />
    </ChapterPage>
  );
}
