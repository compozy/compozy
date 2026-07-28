import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { TechArticleJsonLd } from "../structured-data";

describe("structured data JSON-LD", () => {
  it("escapes script-breaking characters while preserving JSON content", () => {
    const lineSeparator = String.fromCharCode(0x2028);
    const paragraphSeparator = String.fromCharCode(0x2029);
    const title = 'Safe </script><script>alert("x")</script> & title';
    const description = `Line${lineSeparator}separator and paragraph${paragraphSeparator}separator`;
    const markup = renderToStaticMarkup(
      <TechArticleJsonLd description={description} path="/runtime/unsafe-json" title={title} />
    );

    const [, scriptText = ""] =
      markup.match(/<script type="application\/ld\+json">(.*)<\/script>/) ?? [];

    expect(scriptText).not.toBe("");
    expect(scriptText).not.toContain("</script><script>");
    expect(scriptText).not.toContain("<script>alert");
    expect(scriptText).not.toContain("& title");
    expect(scriptText).toContain("\\u003c/script\\u003e\\u003cscript\\u003e");
    expect(scriptText).toContain("\\u0026 title");
    expect(scriptText).toContain("Line\\u2028separator");
    expect(scriptText).toContain("paragraph\\u2029separator");
    expect(JSON.parse(scriptText)).toMatchObject({ description, headline: title });
  });
});
