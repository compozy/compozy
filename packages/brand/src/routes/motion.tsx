import { createFileRoute } from "@tanstack/react-router";

import { chapterHead } from "../manual/head";
import { Delivery } from "../manual/motion/delivery";
import { KineticType } from "../manual/motion/kinetic-type";
import { Principles } from "../manual/motion/principles";
import { Signatures } from "../manual/motion/signatures";
import { Sound } from "../manual/motion/sound";
import { Tokens } from "../manual/motion/tokens";
import { Video } from "../manual/motion/video";
import { ChapterPage } from "../manual/parts/chapter-page";

const DESCRIPTION =
  "Compozy motion: tokens, signature moves, kinetic type, video structure, sound and delivery.";

export const Route = createFileRoute("/motion")({
  head: () => chapterHead("motion", DESCRIPTION),
  component: MotionChapter,
});

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
    >
      <Principles />
      <Tokens />
      <Signatures />
      <KineticType />
      <Video />
      <Sound />
      <Delivery />
    </ChapterPage>
  );
}
