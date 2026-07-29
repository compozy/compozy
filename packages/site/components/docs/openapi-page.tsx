"use client";

import { COMPOZY_CODE_THEMES } from "@compozy/ui/lib/code-theme";
import { defaultShikiFactory } from "fumadocs-core/highlight/shiki/full";
import { createCodeUsageGeneratorRegistry } from "fumadocs-openapi/requests/generators";
import { curl } from "fumadocs-openapi/requests/generators/curl";
import { go } from "fumadocs-openapi/requests/generators/go";
import { javascript } from "fumadocs-openapi/requests/generators/javascript";
import { python } from "fumadocs-openapi/requests/generators/python";
import { createOpenAPIPage } from "fumadocs-openapi/ui";

export type { OpenAPIPageProps_Preloaded } from "fumadocs-openapi/ui";

// Compozy's operator stack: curl for anyone, JS/TS for bridge SDK users, Go
// for runtime developers, Python for agent tooling. The full default registry
// (7 languages) overflows the example column and adds noise.
const codeUsages = createCodeUsageGeneratorRegistry();
codeUsages.add("curl", curl);
codeUsages.add("js", javascript);
codeUsages.add("go", go);
codeUsages.add("python", python);

export const OpenAPIPage = createOpenAPIPage({
  shiki: defaultShikiFactory,
  shikiOptions: {
    themes: { light: COMPOZY_CODE_THEMES.light, dark: COMPOZY_CODE_THEMES.dark },
  },
  playground: { enabled: false },
  codeUsages,
  content: {
    // Site-owned wrappers so global.css can control rhythm, dividers, and the
    // two-column split without depending on fumadocs' internal DOM.
    renderPageLayout: slots => (
      <div className="site-api-page @container">
        {slots.operations?.map(op => (
          <div key={`${op.item.path}:${op.item.method}`} className="site-api-operation">
            {op.children}
          </div>
        ))}
        {slots.webhooks?.map(hook => (
          <div key={`${hook.item.name}:${hook.item.method}`} className="site-api-operation">
            {hook.children}
          </div>
        ))}
      </div>
    ),
    renderOperationLayout: slots => (
      <div className="site-api-operation-cols">
        <div className="site-api-operation-main">
          {slots.header}
          {slots.apiPlayground}
          {slots.description}
          {slots.authSchemes}
          {slots.parameters}
          {slots.body}
          {slots.responses}
          {slots.callbacks}
        </div>
        <div className="site-api-operation-example">{slots.apiExample}</div>
      </div>
    ),
  },
});
