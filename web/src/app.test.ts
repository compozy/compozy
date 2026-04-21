import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { UIProvider } from "@compozy/ui";

import { App } from "./app";

describe("web workspace bootstrap", () => {
  it("renders the placeholder app through the shared ui provider", () => {
    const html = renderToStaticMarkup(createElement(UIProvider, null, createElement(App)));

    expect(html).toContain("Web workspace bootstrap is ready");
    expect(html).toContain("later tasks add the routed operator shell");
    expect(html).toContain('aria-label="Web UI bootstrap status"');
  });
});
