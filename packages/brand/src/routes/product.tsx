import { createFileRoute } from "@tanstack/react-router";

import { chapterHead } from "../manual/head";
import { ChapterPage } from "../manual/parts/chapter-page";

const DESCRIPTION =
  "How the Compozy brand lives inside the product: the desktop control plane, surfaces, agent states, controls, data, the CLI as a secondary surface and the migration from the old brand.";

export const Route = createFileRoute("/product")({
  head: () => chapterHead("product", DESCRIPTION),
  component: ProductChapter,
});

// Cover and intro are ported; the sections arrive with slice S4.
function ProductChapter() {
  return (
    <ChapterPage
      slug="product"
      title={
        <>
          The brand, <em>inside</em> the product
        </>
      }
      intro="Where people spend hours, the brand gets quiet: ink surfaces, hairlines, agent faces, one action colour and seven states that always say what is happening. This chapter covers the desktop control plane, the CLI and API as secondary surfaces, and maps every old asset to its replacement."
    />
  );
}
