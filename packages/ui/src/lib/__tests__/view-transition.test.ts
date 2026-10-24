import { act, fireEvent, render, screen } from "@testing-library/react";
import { createElement, Fragment, useState, ViewTransition } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { TOKENS_RUNTIME_CSS } from "../../__tests__/token-source";
import {
  runViewTransition,
  startTypedTransition,
  viewTransitionName,
  VIEW_TRANSITION_CLASS,
  VIEW_TRANSITION_DRILL,
  VIEW_TRANSITION_TYPE,
} from "../view-transition";

function stubTransition() {
  const handle = {
    updateCallbackDone: Promise.resolve(),
    finished: Promise.resolve(),
    ready: Promise.resolve(),
  };
  const start = vi.fn((arg: (() => void) | { update: () => void }) => {
    if (typeof arg === "function") arg();
    else arg.update();
    return handle;
  });
  Reflect.set(document, "startViewTransition", start);
  vi.spyOn(window, "matchMedia").mockReturnValue({ matches: false } as MediaQueryList);
  return start;
}

afterEach(() => {
  Reflect.deleteProperty(document, "startViewTransition");
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("runViewTransition", () => {
  it("Should run the update directly when the engine has no view transitions", async () => {
    const update = vi.fn();
    await runViewTransition(update, { types: ["drill-in"] });
    expect(update).toHaveBeenCalledTimes(1);
  });

  it("Should skip the transition but still update when motion is reduced", async () => {
    const start = stubTransition();
    const update = vi.fn();
    await runViewTransition(update, { reduced: true });
    expect(start).not.toHaveBeenCalled();
    expect(update).toHaveBeenCalledTimes(1);
  });

  it("Should pass types when the engine supports transition types", async () => {
    const start = stubTransition();
    vi.stubGlobal("CSS", { supports: () => true });
    const update = vi.fn();
    await runViewTransition(update, { types: ["drill-in"] });
    expect(start).toHaveBeenCalledWith(expect.objectContaining({ types: ["drill-in"] }));
    expect(update).toHaveBeenCalledTimes(1);
  });

  it("Should fall back to a bare callback when transition types are unsupported", async () => {
    const start = stubTransition();
    vi.stubGlobal("CSS", { supports: () => false });
    const update = vi.fn();
    await runViewTransition(update, { types: ["drill-in"] });
    expect(typeof start.mock.calls[0]?.[0]).toBe("function");
    expect(update).toHaveBeenCalledTimes(1);
  });
});

describe("viewTransitionName", () => {
  it("Should build a valid custom ident from arbitrary parts", () => {
    expect(viewTransitionName("agent", "my agent/1", 2)).toBe("vt-agent-my_agent_1-2");
  });
});

describe("startTypedTransition", () => {
  // The real renderer drives the stubbed browser API: the boundary activates only when the
  // update is a transition, and React hands the added types to `document.startViewTransition`.
  function Fixture() {
    const [view, setView] = useState("list");
    return createElement(
      Fragment,
      null,
      createElement(
        "button",
        {
          type: "button",
          onClick: () =>
            startTypedTransition(VIEW_TRANSITION_TYPE.drillIn, () => setView("detail")),
        },
        "open"
      ),
      createElement(
        ViewTransition,
        { key: view, default: "none", enter: VIEW_TRANSITION_DRILL, exit: VIEW_TRANSITION_DRILL },
        createElement("div", null, view)
      )
    );
  }

  it("Should commit the update as a transition carrying the type", async () => {
    const start = stubTransition();
    render(createElement(Fixture));
    await act(async () => {
      fireEvent.click(screen.getByText("open"));
    });
    expect(screen.getByText("detail")).toBeInTheDocument();
    expect(start).toHaveBeenCalledWith(expect.objectContaining({ types: ["drill-in"] }));
  });
});

describe("view-transition vocabulary", () => {
  it.each(Object.entries(VIEW_TRANSITION_CLASS))(
    "Should style the %s class on a view-transition snapshot",
    (_name, className) => {
      const selector = new RegExp(String.raw`::view-transition-(?:old|new)\(\.${className}\)`);
      expect(TOKENS_RUNTIME_CSS).toMatch(selector);
    }
  );

  it("Should cut every view-transition animation under reduced motion", () => {
    const cut = TOKENS_RUNTIME_CSS.match(
      /::view-transition-group\(\*\),\s*::view-transition-old\(\*\),\s*::view-transition-new\(\*\)\s*\{[^}]*animation:\s*none\s*!important/
    );
    expect(cut).not.toBeNull();
    const media = TOKENS_RUNTIME_CSS.lastIndexOf("@media", cut?.index);
    expect(TOKENS_RUNTIME_CSS.slice(media, media + 45)).toContain(
      "(prefers-reduced-motion: reduce)"
    );
  });
});
