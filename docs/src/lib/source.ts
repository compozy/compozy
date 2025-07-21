import { docs } from "@/.source";
import { Icon } from "@/components/ui/icon";
import { loader } from "fumadocs-core/source";
import { attachFile, createOpenAPI } from "fumadocs-openapi/server";
import { createElement } from "react";

// See https://fumadocs.vercel.app/docs/headless/source-api for more info
export const source = loader({
  baseUrl: "/docs",
  source: docs.toFumadocsSource(),
  pageTree: {
    attachFile,
  },
  icon(icon) {
    if (!icon) return undefined;
    // Use our Icon component for rendering icons
    return createElement(Icon, { name: icon });
  },
});

// // Configure OpenAPI with Scalar playground for better API documentation UI
// export const openapi = createOpenAPI({
//   renderer: {
//     APIPlayground,
//   },
// });

export const openapi = createOpenAPI();

export const { getPage, getPages, pageTree } = source;
