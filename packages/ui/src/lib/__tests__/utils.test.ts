import { describe, expect, it } from "vitest";

import { fontSizeClasses } from "../font-size-classes.generated";
import { cn } from "../utils";

describe("cn", () => {
  it("Should keep a project font-size utility that shares a call with a color utility", () => {
    // tailwind-merge reads an unregistered `text-*` class as a color and drops it
    // as a conflict. This silently flattened blog article headings to body size.
    expect(cn("text-site-doc-heading text-fg").split(" ")).toEqual([
      "text-site-doc-heading",
      "text-fg",
    ]);
  });

  it("Should keep every generated font-size class alongside a color utility", () => {
    const dropped = fontSizeClasses.filter(
      size => !cn(`${size} text-muted`).split(" ").includes(size)
    );
    expect(dropped).toEqual([]);
  });

  it("Should still resolve two font-size utilities to the last one", () => {
    expect(cn("text-site-doc-heading text-site-subheading")).toBe("text-site-subheading");
  });

  it("Should still resolve two color utilities to the last one", () => {
    expect(cn("text-fg text-muted")).toBe("text-muted");
  });

  it("Should keep a font-family utility beside a font-weight utility", () => {
    expect(cn("font-sans font-semibold").split(" ")).toEqual(["font-sans", "font-semibold"]);
  });
});
