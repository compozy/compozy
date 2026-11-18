// Suite: docs sidebar folders and current row
// Invariant: When navigation lands on a page inside a folder the reader closed, that folder and its
// ancestors open again so the current row shows; folders the reader toggled elsewhere keep their
// state. The rail scrolls to the current row only when that row is out of view.
// Boundary IN: DocsFrame's folder state across pathname changes; DocsSidebar's rail scrolling.
// Boundary OUT: The page tree loader, the drawer, and real layout (boxes are stubbed).
import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { DocsFrame, type DocsFrameSection } from "@/components/docs/docs-frame";
import { DocsSidebar } from "@/components/docs/docs-sidebar";

const mocks = vi.hoisted(() => ({ pathname: "/docs/sessions/lifecycle/" }));

vi.mock("next/navigation", () => ({
  usePathname: () => mocks.pathname,
}));

const SECTIONS: DocsFrameSection[] = [
  {
    id: "runtime",
    label: "Runtime",
    count: 4,
    heads: ["sessions", "terminal", "agents"],
    nodes: [
      {
        key: "sessions",
        name: "Sessions",
        url: "/docs/sessions",
        children: [
          { key: "lifecycle", name: "Lifecycle", url: "/docs/sessions/lifecycle" },
          {
            key: "views",
            name: "Views",
            children: [{ key: "board", name: "Sessions board", url: "/docs/sessions/board" }],
          },
        ],
      },
      { key: "terminal", name: "Terminal", url: "/docs/terminal" },
      {
        key: "agents",
        name: "Agents",
        children: [{ key: "definitions", name: "Definitions", url: "/docs/agents/definitions" }],
      },
    ],
  },
];

function frame() {
  return (
    <DocsFrame sections={SECTIONS} railless={["/docs"]}>
      <main />
    </DocsFrame>
  );
}

function expanded(name: string): boolean {
  const toggle = screen.getByRole("button", { name: new RegExp(`^(Collapse |Expand )?${name}$`) });
  return toggle.getAttribute("aria-expanded") === "true";
}

describe("DocsFrame folders", () => {
  beforeEach(() => {
    mocks.pathname = "/docs/sessions/lifecycle/";
  });

  it("reopens the folders holding the page navigation lands on, and only those", () => {
    const view = render(frame());
    expect(expanded("Sessions")).toBe(true);
    fireEvent.click(screen.getByRole("button", { name: "Views" }));
    fireEvent.click(screen.getByRole("button", { name: "Views" }));
    expect(expanded("Views")).toBe(false);
    fireEvent.click(screen.getByRole("button", { name: "Collapse Sessions" }));
    fireEvent.click(screen.getByRole("button", { name: "Agents" }));

    mocks.pathname = "/docs/terminal/";
    view.rerender(frame());
    expect(expanded("Sessions")).toBe(false);
    expect(expanded("Agents")).toBe(true);

    mocks.pathname = "/docs/sessions/board/";
    view.rerender(frame());
    expect(expanded("Sessions")).toBe(true);
    expect(expanded("Views")).toBe(true);
    expect(expanded("Agents")).toBe(true);
    expect(screen.getByRole("link", { name: "Sessions board" }).getAttribute("aria-current")).toBe(
      "page"
    );
  });

  it("keeps a folder the reader closed on the page being read closed", () => {
    const view = render(frame());
    fireEvent.click(screen.getByRole("button", { name: "Collapse Sessions" }));
    view.rerender(frame());
    expect(expanded("Sessions")).toBe(false);
  });
});

describe("DocsSidebar current row", () => {
  const RAIL_HEIGHT = 500;
  const ROW_HEIGHT = 32;
  /** Where each row sits in the rail's content, by label: Definitions is far below the fold. */
  const ROW_TOP: Record<string, number> = { Sessions: 60, Lifecycle: 100, Definitions: 900 };
  const scrolled = new WeakMap<Element, number>();

  function box(top: number, height: number): DOMRect {
    return { top, bottom: top + height, height, left: 0, right: 296, width: 296 } as DOMRect;
  }

  beforeEach(() => {
    vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(function (
      this: HTMLElement
    ) {
      if (this.classList.contains("docs-rail")) return box(0, RAIL_HEIGHT);
      const rail = this.closest(".docs-rail");
      const top = ROW_TOP[this.textContent ?? ""] ?? 0;
      return box(top - (rail ? (scrolled.get(rail) ?? 0) : 0), ROW_HEIGHT);
    });
    for (const [prop, value] of [
      ["scrollHeight", 1200],
      ["clientHeight", RAIL_HEIGHT],
    ] as const) {
      Object.defineProperty(HTMLElement.prototype, prop, { configurable: true, get: () => value });
    }
    Object.defineProperty(HTMLElement.prototype, "scrollTop", {
      configurable: true,
      get(this: HTMLElement) {
        return scrolled.get(this) ?? 0;
      },
      set(this: HTMLElement, value: number) {
        scrolled.set(this, value);
      },
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
    for (const prop of ["scrollHeight", "clientHeight", "scrollTop"]) {
      Reflect.deleteProperty(HTMLElement.prototype, prop);
    }
  });

  function sidebar(activeUrl: string) {
    return (
      <aside className="docs-rail">
        <DocsSidebar
          groups={SECTIONS}
          activeUrl={activeUrl}
          toggled={{ agents: true }}
          onToggle={() => {}}
        />
      </aside>
    );
  }

  it("scrolls the rail only when the current row is out of view", () => {
    const view = render(sidebar("/docs/sessions/lifecycle"));
    const rail = view.container.querySelector<HTMLElement>(".docs-rail");
    if (!rail) throw new Error("no rail");
    expect(rail.scrollTop).toBe(0);

    rail.scrollTop = 40;
    view.rerender(sidebar("/docs/sessions"));
    expect(rail.scrollTop).toBe(40);

    view.rerender(sidebar("/docs/agents/definitions"));
    // The row is centred: 900 from the top of the content, less half the room around it.
    expect(rail.scrollTop).toBe(900 - (RAIL_HEIGHT - ROW_HEIGHT) / 2);
  });
});
