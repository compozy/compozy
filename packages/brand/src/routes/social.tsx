import { createFileRoute } from "@tanstack/react-router";

import { chapterHead } from "../manual/head";
import { ChapterPage } from "../manual/parts/chapter-page";

const DESCRIPTION =
  "Compozy on X, LinkedIn, YouTube, GitHub and Discord: profile assets, templates and captions.";

export const Route = createFileRoute("/social")({
  head: () => chapterHead("social", DESCRIPTION),
  component: SocialChapter,
});

// Cover and intro are ported; the sections arrive with slice S4.
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
    />
  );
}
