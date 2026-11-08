import { createFileRoute } from "@tanstack/react-router";

import { chapterHead } from "../manual/head";
import { Avoid } from "../manual/imagery/avoid";
import { Consent } from "../manual/imagery/consent";
import { Direction } from "../manual/imagery/direction";
import { Generated } from "../manual/imagery/generated";
import { ProductShots } from "../manual/imagery/product-shots";
import { Treatment } from "../manual/imagery/treatment";
import { Types } from "../manual/imagery/types";
import { ChapterPage } from "../manual/parts/chapter-page";

const DESCRIPTION =
  "Compozy imagery: photography direction, product shots, the cool grade, generated images and consent.";

export const Route = createFileRoute("/imagery")({
  head: () => chapterHead("imagery", DESCRIPTION),
  component: ImageryChapter,
});

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
    >
      <Direction />
      <Types />
      <ProductShots />
      <Treatment />
      <Generated />
      <Avoid />
      <Consent />
    </ChapterPage>
  );
}
