import { createFileRoute } from "@tanstack/react-router";

import { chapterHead } from "../manual/head";
import { ChapterPage } from "../manual/parts/chapter-page";

const DESCRIPTION =
  "Compozy offline: print colour, badges, stickers, merch, signage, stationery and the brand on real materials.";

export const Route = createFileRoute("/offline")({
  head: () => chapterHead("offline", DESCRIPTION),
  component: OfflineChapter,
});

// Cover and intro are ported; the sections arrive with slice S4.
function OfflineChapter() {
  return (
    <ChapterPage
      slug="offline"
      title={
        <>
          The brand, <em className="em-glint">printed</em>
        </>
      }
      intro="Everything that leaves the screen: how the gradient survives on paper, badges for four roles, stickers, merch, event signage and stationery, each with a print-ready template."
    />
  );
}
