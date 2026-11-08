import { createFileRoute } from "@tanstack/react-router";

import { chapterHead } from "../manual/head";
import { ChapterPage } from "../manual/parts/chapter-page";

const DESCRIPTION =
  "Compozy imagery: photography direction, product shots, the cool grade, generated images and consent.";

export const Route = createFileRoute("/imagery")({
  head: () => chapterHead("imagery", DESCRIPTION),
  component: ImageryChapter,
});

// Cover and intro are ported; the sections arrive with slice S4.
function ImageryChapter() {
  return (
    <ChapterPage
      slug="imagery"
      title={
        <>
          Show the work, <em>not</em> the robot.
        </>
      }
      intro="Photography, product shots and generated images: what Compozy shows, the one grade every picture shares, and the AI clichés it never uses."
    />
  );
}
