import { createFileRoute } from "@tanstack/react-router";

import { Blog } from "../manual/editorial/blog";
import { Changelog } from "../manual/editorial/changelog";
import { Code } from "../manual/editorial/code";
import { Docs } from "../manual/editorial/docs";
import { Newsletter } from "../manual/editorial/newsletter";
import { Og } from "../manual/editorial/og";
import { Slides } from "../manual/editorial/slides";
import { chapterHead } from "../manual/head";
import { ChapterPage } from "../manual/parts/chapter-page";

const DESCRIPTION =
  "Compozy editorial: link images, docs, blog, changelog, newsletter, slides and code as image.";

export const Route = createFileRoute("/editorial")({
  head: () => chapterHead("editorial", DESCRIPTION),
  component: EditorialChapter,
});

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
    >
      <Og />
      <Docs />
      <Blog />
      <Changelog />
      <Newsletter />
      <Slides />
      <Code />
    </ChapterPage>
  );
}
