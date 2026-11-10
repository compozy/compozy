import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { useUrlBackedValue } from "@/components/site/hooks/use-url-backed-value";

/** The hook under a URL the test controls: `type` is a local write, `url` is what the router reports. */
function setup(initial: string) {
  const hook = renderHook(({ url }) => useUrlBackedValue(url), { initialProps: { url: initial } });
  return {
    value: () => hook.result.current[0],
    type: (...values: string[]) =>
      act(() => {
        for (const value of values) hook.result.current[1](value);
      }),
    url: (url: string) => hook.rerender({ url }),
  };
}

describe("useUrlBackedValue", () => {
  it("follows a navigation, with or without writes of its own in between", () => {
    const box = setup("retries");
    box.url("");
    expect(box.value()).toBe("");

    box.type("a");
    box.url("a");
    box.url("");
    expect(box.value()).toBe("");
  });

  it("never lets a late report of its own write overwrite newer typing", () => {
    const box = setup("");
    box.type("a");
    box.type("ab");
    box.type("abc");
    box.url("a");
    expect(box.value()).toBe("abc");
    box.url("ab");
    expect(box.value()).toBe("abc");
    box.url("abc");
    expect(box.value()).toBe("abc");
  });

  it("forgets writes the URL has caught up with, so navigating back to one still applies", () => {
    const box = setup("");
    box.type("a");
    box.type("ab");
    // The router batches both writes into one report.
    box.url("ab");
    box.url("a");
    expect(box.value()).toBe("a");
  });

  it("clears on navigation after typing and deleting in the same tick, then typing again", () => {
    const box = setup("");
    // "a" and Backspace land together: the URL never reports "a", it stays "".
    box.type("a", "");
    box.type("a");
    box.url("a");
    expect(box.value()).toBe("a");
    // The header "Blog" link.
    box.url("");
    expect(box.value()).toBe("");
  });
});
