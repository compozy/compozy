import { docs } from "@/.source";
import { create } from "@/components/ui/icon";
import { loader } from "fumadocs-core/source";
import { attachFile, createOpenAPI } from "fumadocs-openapi/server";
import { icons } from "lucide-react";

// See https://fumadocs.vercel.app/docs/headless/source-api for more info
export const source = loader({
  baseUrl: "/docs",
  source: docs.toFumadocsSource(),
  pageTree: {
    attachFile,
  },
  icon(icon) {
    if (!icon) return;
    if (icon in icons) {
      return create({ icon: icons[icon as keyof typeof icons] });
    }
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
