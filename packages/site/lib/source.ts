import { createElement, type ReactElement } from "react";
import { loader } from "fumadocs-core/source";
import { getLayoutTabs } from "fumadocs-ui/layouts/shared";
import { runtime, protocol } from "@/.source/server";
import { DOCS_ICONS } from "./docs-icons";
import { overviewPageTransformer } from "./docs-overview-tree";
import { createRuntimeLayoutTree } from "./runtime-navigation";

function iconResolver(icon?: string): ReactElement | undefined {
  if (!icon) return undefined;
  const Icon = DOCS_ICONS[icon];
  return Icon ? createElement(Icon) : undefined;
}

// The positional `loader(source, options)` overload keeps the collection page-data types inferred
// from the source alone; the single-object form widens them to the base schema once `pageTree` is
// supplied.
export const runtimeDocs = loader(runtime.toFumadocsSource(), {
  baseUrl: "/runtime",
  icon: iconResolver,
  pageTree: { transformers: [overviewPageTransformer()] },
});

export const protocolDocs = loader(protocol.toFumadocsSource(), {
  baseUrl: "/protocol",
  icon: iconResolver,
  pageTree: { transformers: [overviewPageTransformer()] },
});

export const runtimeLayoutTree = createRuntimeLayoutTree(runtimeDocs.pageTree);

export const runtimeTabs = getLayoutTabs(runtimeLayoutTree);
