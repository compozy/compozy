import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { SplitPane } from "../split-pane";
import { UIProvider } from "../ui-provider";

interface MediaMock {
  matches: boolean;
  media: string;
  onchange: null;
  addEventListener: (type: string, cb: (event: MediaQueryListEvent) => void) => void;
  removeEventListener: (type: string, cb: (event: MediaQueryListEvent) => void) => void;
  dispatchEvent: () => boolean;
  addListener: () => void;
  removeListener: () => void;
  _fire: (matches: boolean) => void;
}

function installMatchMedia(resolve: (query: string) => boolean) {
  const listeners = new Map<string, Set<(event: MediaQueryListEvent) => void>>();
  const instances = new Map<string, MediaMock>();

  const factory = (query: string): MediaQueryList => {
    if (instances.has(query)) return instances.get(query) as unknown as MediaQueryList;
    const mock: MediaMock = {
      matches: resolve(query),
      media: query,
      onchange: null,
      addEventListener: (_type, cb) => {
        const set = listeners.get(query) ?? new Set();
        set.add(cb);
        listeners.set(query, set);
      },
      removeEventListener: (_type, cb) => {
        listeners.get(query)?.delete(cb);
      },
      dispatchEvent: () => false,
      addListener: () => {},
      removeListener: () => {},
      _fire: matches => {
        mock.matches = matches;
        listeners.get(query)?.forEach(cb => cb({ matches } as MediaQueryListEvent));
      },
    };
    instances.set(query, mock);
    return mock as unknown as MediaQueryList;
  };

  Object.defineProperty(window, "matchMedia", {
    writable: true,
    configurable: true,
    value: factory,
  });
}

const ORIGINAL_MATCH_MEDIA = window.matchMedia;

afterEach(() => {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    configurable: true,
    value: ORIGINAL_MATCH_MEDIA,
  });
});

describe("SplitPane", () => {
  it("Should render the list column at the configured width and the detail column flex-1", () => {
    const { container } = render(
      <SplitPane
        list={<div data-testid="list">list</div>}
        detail={<div data-testid="detail">detail</div>}
      />
    );
    const list = container.querySelector<HTMLElement>("[data-slot=split-pane-list]");
    expect(list?.style.width).toBe("340px");
    expect(screen.getByTestId("list")).toBeInTheDocument();

    const detail = container.querySelector<HTMLElement>("[data-slot=split-pane-detail]");
    expect(detail).not.toBeNull();
    expect(detail?.className).toContain("flex-1");
    expect(screen.getByTestId("detail")).toBeInTheDocument();
  });

  it("Should honour a custom listWidth prop", () => {
    const { container } = render(
      <SplitPane list={<div>list</div>} listWidth={420} detail={<div>detail</div>} />
    );
    const list = container.querySelector<HTMLElement>("[data-slot=split-pane-list]");
    expect(list?.style.width).toBe("420px");
  });

  it("Should render detailEmpty when detail is null", () => {
    render(
      <SplitPane
        list={<div>list</div>}
        detail={null}
        detailEmpty={<div data-testid="empty">nothing selected</div>}
      />
    );
    expect(screen.getByTestId("empty")).toBeInTheDocument();
  });

  it("Should render detailEmpty when detail is undefined", () => {
    render(
      <SplitPane
        list={<div>list</div>}
        detailEmpty={<div data-testid="empty">nothing selected</div>}
      />
    );
    expect(screen.getByTestId("empty")).toBeInTheDocument();
  });

  it("Should swap to the detail body when a detail node is provided", async () => {
    const { rerender, container } = render(
      <UIProvider reducedMotion="always">
        <SplitPane
          list={<div>list</div>}
          detail={null}
          detailEmpty={<div data-testid="empty">empty</div>}
        />
      </UIProvider>
    );

    expect(container.querySelector("[data-slot=split-pane-detail-empty]")).not.toBeNull();

    rerender(
      <UIProvider reducedMotion="always">
        <SplitPane
          list={<div>list</div>}
          detail={<div data-testid="body">body</div>}
          detailEmpty={<div data-testid="empty">empty</div>}
        />
      </UIProvider>
    );

    await waitFor(() => {
      expect(container.querySelector("[data-slot=split-pane-detail-body]")).not.toBeNull();
    });
    expect(screen.getByTestId("body")).toBeInTheDocument();
  });

  it("Should collapse to a single detail column with a back button on narrow viewports", async () => {
    installMatchMedia(q => q.includes("max-width"));
    const onDetailClose = vi.fn();
    const { container } = render(
      <SplitPane
        list={<div data-testid="list">list</div>}
        detail={<div data-testid="detail">detail</div>}
        onDetailClose={onDetailClose}
      />
    );

    expect(container.querySelector("[data-slot=split-pane]")).toHaveAttribute(
      "data-narrow",
      "true"
    );
    expect(screen.getByTestId("detail")).toBeInTheDocument();
    expect(screen.queryByTestId("list")).not.toBeInTheDocument();

    const back = screen.getByRole("button", { name: "Back" });
    const user = userEvent.setup();
    await user.click(back);
    expect(onDetailClose).toHaveBeenCalledTimes(1);
  });

  it("Should fall back to a stacked narrow layout when no detail close handler is provided", () => {
    installMatchMedia(q => q.includes("max-width"));
    const { container } = render(
      <SplitPane
        list={<div data-testid="list">list</div>}
        detail={<div data-testid="detail">detail</div>}
      />
    );

    expect(screen.getByTestId("list")).toBeInTheDocument();
    expect(screen.getByTestId("detail")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Back" })).toBeNull();
    expect(container.querySelector("[data-slot=split-pane-list]")).toHaveStyle({
      width: "100%",
    });
  });

  it("Should show the list when narrow and no detail is selected", () => {
    installMatchMedia(q => q.includes("max-width"));
    render(
      <SplitPane
        list={<div data-testid="list">list</div>}
        detailEmpty={<div data-testid="empty">empty</div>}
      />
    );
    expect(screen.getByTestId("list")).toBeInTheDocument();
    expect(screen.queryByTestId("empty")).not.toBeInTheDocument();
  });
});
