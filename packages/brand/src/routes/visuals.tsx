import { createFileRoute } from "@tanstack/react-router";

import { chapterHead } from "../manual/head";
import { ChapterPage } from "../manual/parts/chapter-page";
import { Atmosphere } from "../manual/visuals/atmosphere";
import { Composition } from "../manual/visuals/composition";
import { Donts } from "../manual/visuals/donts";
import { Faces } from "../manual/visuals/faces";
import { Families } from "../manual/visuals/families";
import { Library } from "../manual/visuals/library";
import { Light } from "../manual/visuals/light";
import { Live } from "../manual/visuals/live";
import { Pairing } from "../manual/visuals/pairing";
import { Principle } from "../manual/visuals/principle";

const DESCRIPTION =
  "The Compozy visual system: Atmosphere for mood, Live for the product and its data.";

export const Route = createFileRoute("/visuals")({
  head: () => chapterHead("visuals", DESCRIPTION),
  component: VisualsChapter,
});

function VisualsChapter() {
  return (
    <ChapterPage
      slug="visuals"
      title={
        <>
          Your agents, one autonomous <em>control</em> plane.
        </>
      }
      intro="Our UI is the main image: the desktop, the always-on agents with their faces, the board and the handoffs. Particles, light and hairlines set the mood around it."
    >
      <Principle />
      <Families />
      <Atmosphere />
      <Live />
      <Faces />
      <Light />
      <Composition />
      <Pairing />
      <Library />
      <Donts />
    </ChapterPage>
  );
}
