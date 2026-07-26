"use client";

import { AGH_CODE_THEMES } from "@agh/ui/lib/code-theme";
import { defaultShikiFactory } from "fumadocs-core/highlight/shiki/full";
import { createOpenAPIPage } from "fumadocs-openapi/ui";

export type { OpenAPIPageProps_Preloaded } from "fumadocs-openapi/ui";

export const OpenAPIPage = createOpenAPIPage({
  shiki: defaultShikiFactory,
  shikiOptions: {
    themes: { light: AGH_CODE_THEMES.light, dark: AGH_CODE_THEMES.dark },
  },
  playground: { enabled: false },
});
