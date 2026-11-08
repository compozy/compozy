import { createFileRoute } from "@tanstack/react-router";

import { chapterHead } from "../manual/head";
import { ChapterPage } from "../manual/parts/chapter-page";

const DESCRIPTION =
  "Compozy editorial: link images, docs, blog, changelog, newsletter, slides and code as image.";

export const Route = createFileRoute("/editorial")({
  head: () => chapterHead("editorial", DESCRIPTION),
  component: EditorialChapter,
});

// Cover and intro are ported; the sections arrive with slice S4.
function EditorialChapter() {
  return (
    <ChapterPage
      slug="editorial"
      title={
        <>
          Words, <em>shipped</em> with the product.
        </>
      }
      intro="Link previews, docs, posts, release notes, email and talks. The places where Compozy explains itself at length, with templates for every image they need."
    />
  );
}
