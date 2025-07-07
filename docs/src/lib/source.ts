import { docs } from "@/.source";
import { create } from "@/components/ui/icon";
import { loader } from "fumadocs-core/source";
import { icons } from "lucide-react";

// See https://fumadocs.vercel.app/docs/headless/source-api for more info
export const source = loader({
  baseUrl: "/docs",
  source: docs.toFumadocsSource(),
  icon(icon) {
    if (!icon) return;
    if (icon in icons) {
      return create({ icon: icons[icon as keyof typeof icons] });
    }
  },
});

export const { getPage, getPages, pageTree } = source;
