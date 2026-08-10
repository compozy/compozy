"use client";

import { COMPOZY_CODE_THEMES } from "@compozy/ui/lib/code-theme";
import { defaultShikiFactory } from "fumadocs-core/highlight/shiki/full";
import {
  createCodeUsageGeneratorRegistry,
  type CodeUsageGenerator,
} from "fumadocs-openapi/requests/generators";
import { curl } from "fumadocs-openapi/requests/generators/curl";
import { javascript } from "fumadocs-openapi/requests/generators/javascript";
import { python } from "fumadocs-openapi/requests/generators/python";
import { createOpenAPIPage } from "fumadocs-openapi/ui";

export type { OpenAPIPageProps_Preloaded } from "fumadocs-openapi/ui";

// CompozyOS's operator stack: curl for anyone, JS/TS for bridge SDK users, Go
// for runtime developers, Python for agent tooling. The full default registry
// (7 languages) overflows the example column and adds noise.
function goString(value: string): string {
  const encoded = JSON.stringify(value);
  if (encoded === undefined) {
    throw new Error("Go request sample string must be serializable.");
  }
  return encoded;
}

function goJSON(value: unknown): string {
  const encoded = JSON.stringify(value, null, 2);
  if (encoded === undefined) {
    throw new Error("Go request sample body must be JSON serializable.");
  }
  return encoded;
}

/**
 * The stock Fumadocs Go generator ignores request and transport errors, then
 * dereferences a possibly nil response. The CompozyOS API is JSON-only, so this
 * focused generator can produce a complete, safe, current standard-library sample.
 */
export const compozyGo: CodeUsageGenerator = {
  label: "Go",
  lang: "go",
  generate(data) {
    const headers = new Map(
      Object.entries(data.header).map(([name, parameter]) => [name, parameter.value])
    );
    if (data.body !== undefined && data.bodyMediaType) {
      headers.set("Content-Type", data.bodyMediaType);
    }

    const cookies = Object.entries(data.cookie)
      .map(([name, parameter]) => `${name}=${parameter.value}`)
      .join("; ");
    if (cookies) headers.set("Cookie", cookies);

    const jsonBody = data.body === undefined ? undefined : goJSON(data.body);
    const imports = new Set(["fmt", "io", "log", "net/http", "time"]);
    if (jsonBody !== undefined) imports.add("strings");

    const requestBody = jsonBody === undefined ? "nil" : "strings.NewReader(payload)";
    const bodySetup = [
      `url := ${goString(data.url)}`,
      ...(jsonBody === undefined ? [] : [`payload := ${goString(jsonBody)}`]),
    ];
    const headerSetup = Array.from(headers.entries()).map(
      ([name, value]) => `req.Header.Set(${goString(name)}, ${goString(value)})`
    );

    return `package main

import (
${Array.from(imports)
  .sort()
  .map(name => `  ${goString(name)}`)
  .join("\n")}
)

func main() {
${[
  ...bodySetup,
  `req, err := http.NewRequest(${goString(data.method.toUpperCase())}, url, ${requestBody})`,
  "if err != nil {",
  "  log.Fatal(err)",
  "}",
  ...headerSetup,
]
  .map(line => `  ${line}`)
  .join("\n")}
  client := &http.Client{Timeout: 30 * time.Second}
  res, err := client.Do(req)
  if err != nil {
    log.Fatal(err)
  }
  defer func() {
    if err := res.Body.Close(); err != nil {
      log.Printf("close response body: %v", err)
    }
  }()

  body, err := io.ReadAll(res.Body)
  if err != nil {
    log.Fatal(err)
  }
  if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
    log.Printf("request failed: %s\\n%s", res.Status, body)
    return
  }

  fmt.Println(string(body))
}`;
  },
};

const codeUsages = createCodeUsageGeneratorRegistry();
codeUsages.add("curl", curl);
codeUsages.add("js", javascript);
codeUsages.add("go", compozyGo);
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
