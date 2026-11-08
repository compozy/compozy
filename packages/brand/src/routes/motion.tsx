import { createFileRoute } from "@tanstack/react-router";

import { chapterHead } from "../manual/head";
import { ChapterPage } from "../manual/parts/chapter-page";

const DESCRIPTION =
  "Compozy motion: tokens, signature moves, kinetic type, video structure, sound and delivery.";

export const Route = createFileRoute("/motion")({
  head: () => chapterHead("motion", DESCRIPTION),
  component: MotionChapter,
});

// Cover and intro are ported; the sections arrive with slice S4.
function MotionChapter() {
  return (
    <ChapterPage
      slug="motion"
      title={
        <>
          Motion shows work <em>running</em>
        </>
      }
      intro="How Compozy moves on screen and in video: six durations, three eases, eight signature moves built from product state, and the rules for type, captions, sound and files."
    />
  );
}
