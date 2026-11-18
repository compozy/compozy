// Suite: session-list-stops
// Invariant: the session-cycle walk through an open rail visits every
// project-folder, group and folder header and every session row in on-screen
// order, skipping everything else, wrapping both ways and entering at the edge
// when nothing is current. A header lands with keyboard focus; a row lands the
// way a click lands, keeping focus that sits outside the rail where it is,
// and only takes focus while a bulk selection is open.
// Owning layer: unit (systems/session/lib)
import { afterEach, describe, expect, it, vi } from "vitest";

import { walkSessionList } from "../session-list-stops";

const RAIL = `
  <aside data-session-rail data-state="open">
    <button data-sidebar-stop="project-folder" id="project-folder">Clients</button>
    <button data-sidebar-stop="group" id="group">compozy</button>
    <button id="branch">main</button>
    <button data-sidebar-stop="folder" id="folder">Release</button>
    <button data-sidebar-stop="session" data-session-id="s1" id="s1">First</button>
    <div data-slot="session-subagent-row"><button id="subagent">Explore</button></div>
    <button data-sidebar-stop="session" data-session-id="s2" id="s2">Second</button>
    <button id="show-more">Show 2 more</button>
  </aside>
  <textarea id="composer"></textarea>
`;

function mountRail(markup = RAIL) {
  document.body.innerHTML = markup;
  const rail = document.querySelector<HTMLElement>("[data-session-rail]");
  if (rail === null) throw new Error("rail fixture missing");
  const clicked: string[] = [];
  rail.addEventListener("click", event => {
    if (event.target instanceof HTMLElement) clicked.push(event.target.id);
  });
  return { rail, clicked };
}

function byId(id: string) {
  const element = document.getElementById(id);
  if (element === null) throw new Error(`missing #${id}`);
  return element;
}

afterEach(() => {
  document.body.innerHTML = "";
});

describe("walkSessionList", () => {
  it("enters at the first stop going next and the last going previous", () => {
    const { rail, clicked } = mountRail();

    expect(walkSessionList(rail, "next", null)).toBe(true);
    expect(document.activeElement).toBe(byId("project-folder"));

    byId("composer").focus();
    expect(walkSessionList(rail, "previous", null)).toBe(true);
    expect(clicked).toEqual(["s2"]);
    expect(document.activeElement).toBe(byId("composer"));
  });

  it("wraps from the last stop to the first and back", () => {
    const { rail, clicked } = mountRail();
    byId("s2").focus();

    walkSessionList(rail, "next", "s2");
    expect(document.activeElement).toBe(byId("project-folder"));
    expect(clicked).toEqual([]);

    walkSessionList(rail, "previous", "s2");
    expect(document.activeElement).toBe(byId("s2"));
    expect(clicked).toEqual(["s2"]);
  });

  it("steps from a focused header past non-stop controls", () => {
    const { rail } = mountRail();
    byId("group").focus();

    walkSessionList(rail, "next", "s2");

    expect(document.activeElement).toBe(byId("folder"));
  });

  it("walks folder → session → next row, skipping provider subagents", () => {
    const { rail, clicked } = mountRail();
    byId("folder").focus();

    walkSessionList(rail, "next", null);
    expect(clicked).toEqual(["s1"]);
    expect(document.activeElement).toBe(byId("s1"));

    walkSessionList(rail, "next", "s1");
    expect(clicked).toEqual(["s1", "s2"]);
    expect(document.activeElement).toBe(byId("s2"));
  });

  it("resolves the current row from the shown session while focus stays in the composer", () => {
    const { rail, clicked } = mountRail();
    byId("composer").focus();

    walkSessionList(rail, "previous", "s2");

    expect(clicked).toEqual(["s1"]);
    expect(document.activeElement).toBe(byId("composer"));
  });

  it("scrolls the landed stop into view without jumping the rail", () => {
    const { rail } = mountRail();
    const scroll = vi.spyOn(byId("folder"), "scrollIntoView");
    byId("group").focus();

    walkSessionList(rail, "next", null);

    expect(scroll).toHaveBeenCalledExactlyOnceWith({ block: "nearest" });
  });

  it("focuses a row without toggling it while a bulk selection is open", () => {
    const { rail, clicked } = mountRail(
      RAIL.replace('id="s1"', 'id="s1" aria-pressed="false"').replace(
        'id="s2"',
        'id="s2" aria-pressed="true"'
      )
    );
    byId("folder").focus();

    walkSessionList(rail, "next", null);
    expect(document.activeElement).toBe(byId("s1"));
    walkSessionList(rail, "next", "s1");
    expect(document.activeElement).toBe(byId("s2"));
    expect(clicked).toEqual([]);
  });

  it("reports nowhere to go with fewer than two stops", () => {
    const { rail, clicked } = mountRail(`
      <aside data-session-rail data-state="open">
        <button data-sidebar-stop="session" data-session-id="s1" id="s1">Only</button>
      </aside>
    `);

    expect(walkSessionList(rail, "next", "s1")).toBe(false);
    expect(clicked).toEqual([]);
  });
});
