// Suite: saved-layout card
// Invariant: a profile the daemon still stores as a version 4 or 5 document —
// one its boot rewrite logged and skipped (`reconcileStoredWindowLayouts`), as
// the raw list returns it — is shown as saved before tabs, never drawn as if
// it were a v6 layout, and can still be loaded (US-022.AC-3). A current
// profile shows its tab count instead.
// Boundary IN: the list response parsed by the Settings schema.
// Boundary OUT: the daemon conversion on load (owned by the layout editor suite).
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { parseWindowManagerLayoutResources } from "../../lib/window-manager-layout-schema";
import {
  windowManagerEarlierLayoutResourceFixture,
  windowManagerLayoutResourceFixture,
} from "../../mocks/window-manager-fixtures";
import { LayoutProfileCard } from "../layouts/layout-profile-card";

const [earlier, current] = parseWindowManagerLayoutResources({
  records: [windowManagerEarlierLayoutResourceFixture, windowManagerLayoutResourceFixture],
});

describe("LayoutProfileCard", () => {
  it("Should show a profile stored before tabs as saved before tabs and still load it", async () => {
    const user = userEvent.setup();
    const onLoad = vi.fn();
    render(
      <LayoutProfileCard
        record={earlier!}
        selected={false}
        onDelete={vi.fn()}
        onLoad={onLoad}
        onSelect={vi.fn()}
      />
    );

    const card = screen.getByTestId(`layout-profile-card-${earlier!.id}`);
    expect(screen.getByTestId(`layout-profile-earlier-${earlier!.id}`)).toHaveTextContent(
      "Saved before tabsOpens as tabs when loaded"
    );
    expect(card).not.toHaveTextContent(/\d+ tabs?/);
    expect(card).toHaveTextContent("Every project");

    await user.click(screen.getByTestId(`layout-profile-load-${earlier!.id}`));

    expect(onLoad).toHaveBeenCalledTimes(1);
  });

  it("Should draw a current profile with its tab count and no earlier-version note", () => {
    render(
      <LayoutProfileCard
        record={current!}
        selected={false}
        onDelete={vi.fn()}
        onLoad={vi.fn()}
        onSelect={vi.fn()}
      />
    );

    expect(screen.queryByTestId(`layout-profile-earlier-${current!.id}`)).toBeNull();
    expect(screen.getByTestId(`layout-profile-card-${current!.id}`)).toHaveTextContent(/\d+ tabs?/);
  });
});
