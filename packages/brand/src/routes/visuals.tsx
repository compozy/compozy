import { createFileRoute } from "@tanstack/react-router";

import { chapterHead } from "../manual/head";
import { ChapterPage } from "../manual/parts/chapter-page";

const DESCRIPTION =
  "The Compozy visual system: Atmosphere for mood, Live for the product and its data.";

export const Route = createFileRoute("/visuals")({
  head: () => chapterHead("visuals", DESCRIPTION),
  component: VisualsChapter,
});

// Cover and intro are ported; the sections arrive with slice S4.
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
    />
  );
}
