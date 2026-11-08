import { createFileRoute } from "@tanstack/react-router";

import { chapterHead } from "../manual/head";
import { ChapterPage } from "../manual/parts/chapter-page";
import { Captions } from "../manual/social/captions";
import { Carousel } from "../manual/social/carousel";
import { Github } from "../manual/social/github";
import { Linkedin } from "../manual/social/linkedin";
import { Networks } from "../manual/social/networks";
import { X } from "../manual/social/x";
import { Youtube } from "../manual/social/youtube";

const DESCRIPTION =
  "Compozy on X, LinkedIn, YouTube, GitHub and Discord: profile assets, templates and captions.";

export const Route = createFileRoute("/social")({
  head: () => chapterHead("social", DESCRIPTION),
  component: SocialChapter,
});

function SocialChapter() {
  return (
    <ChapterPage
      slug="social"
      title={
        <>
          One idea, the right <em>shape</em> for each network.
        </>
      }
      intro="Every profile asset at its native size, the post templates, and how a Compozy caption is built: outcome, mechanism, proof, link. Pieces about the product show the desktop."
    >
      <Networks />
      <X />
      <Linkedin />
      <Carousel />
      <Youtube />
      <Github />
      <Captions />
    </ChapterPage>
  );
}
