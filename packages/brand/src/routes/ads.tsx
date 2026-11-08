import { createFileRoute } from "@tanstack/react-router";

import { chapterHead } from "../manual/head";
import { ChapterPage } from "../manual/parts/chapter-page";

const DESCRIPTION =
  "Compozy ads: brand and proof modes, structure, live templates, copy bank, channel specs and review.";

export const Route = createFileRoute("/ads")({
  head: () => chapterHead("ads", DESCRIPTION),
  component: AdsChapter,
});

// Cover and intro are ported; the sections arrive with slice S4.
function AdsChapter() {
  return (
    <ChapterPage
      slug="ads"
      title={
        <>
          Ads that show the <em>mechanism</em>.
        </>
      }
      intro="No price, no offer, no customers to quote. A Compozy ad either plants the name or shows our own UI doing its job, never a terminal, and it is honest that the product is in beta."
    />
  );
}
