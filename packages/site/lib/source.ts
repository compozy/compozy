import { createElement, type ReactElement } from "react";
import { loader } from "fumadocs-core/source";
import { docs } from "@/.source/server";
import { DOCS_ICONS } from "./docs-icons";
import { assertDocsTreeComplete } from "./docs-navigation";
import { overviewPageTransformer } from "./docs-overview-tree";

function iconResolver(icon?: string): ReactElement | undefined {
  if (!icon) return undefined;
  const Icon = DOCS_ICONS[icon];
  return Icon ? createElement(Icon) : undefined;
}

// The positional `loader(source, options)` overload keeps the collection page-data types inferred
// from the source alone; the single-object form widens them to the base schema once `pageTree` is
// supplied.
export const docsSource = loader(docs.toFumadocsSource(), {
  baseUrl: "/docs",
  icon: iconResolver,
  pageTree: { transformers: [overviewPageTransformer()] },
});

assertDocsTreeComplete(docsSource.pageTree);
