import { createFileRoute } from "@tanstack/react-router";

import { chapterHead } from "../manual/head";
import { ChapterPage } from "../manual/parts/chapter-page";
import { Cli } from "../manual/product/cli";
import { Controls } from "../manual/product/controls";
import { DataCharts } from "../manual/product/data-charts";
import { Desktop } from "../manual/product/desktop";
import { Migration } from "../manual/product/migration";
import { Principles } from "../manual/product/principles";
import { StatusColours } from "../manual/product/status-colours";
import { Surfaces } from "../manual/product/surfaces";
import { Vocabulary } from "../manual/product/vocabulary";

const DESCRIPTION =
  "How the Compozy brand lives inside the product: the desktop control plane, surfaces, agent states, controls, data, the CLI as a secondary surface and the migration from the old brand.";

export const Route = createFileRoute("/product")({
  head: () => chapterHead("product", DESCRIPTION),
  component: ProductChapter,
});

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
    >
      <Principles />
      <Surfaces />
      <StatusColours />
      <Controls />
      <DataCharts />
      <Cli />
      <Desktop />
      <Vocabulary />
      <Migration />
    </ChapterPage>
  );
}
