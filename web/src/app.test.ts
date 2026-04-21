import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { UIProvider } from "@compozy/ui";

import { App } from "./app";

describe("web shell foundation", () => {
  it("renders the shared shell preview through the shared ui package", () => {
    const html = renderToStaticMarkup(createElement(UIProvider, null, createElement(App)));

    expect(html).toContain("Workflow operator console");
    expect(html).toContain("@compozy/ui");
    expect(html).toContain("Web imports the package directly");
    expect(html).toContain("Sync all");
    expect(html).toContain('aria-current="page"');
    expect(html).not.toContain("Web workspace bootstrap is ready");
  });
});
