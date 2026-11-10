import { describe, expect, it } from "vitest";
import { parseReleaseNotes } from "../changelog/release-notes";
import { groupEras, listState, stableLine, type LineItem } from "../changelog/timeline";

function at(version: string, date: string) {
  return { version, date: `${date}T12:00:00Z`, pre: version.includes("-") };
}

// Newest first, as the changelog loader returns them: 0.5 and 0.4 stable, 0.3 betas before 0.4.0.
const LINE = [
  at("v0.5.1", "2026-11-04"),
  at("v0.5.0", "2026-11-03"),
  at("v0.4.1", "2026-10-25"),
  at("v0.4.0", "2026-10-24"),
  at("v0.3.0-beta.39", "2026-10-23"),
  at("v0.3.0-beta.2", "2026-07-29"),
  at("v0.3.0-beta.1", "2026-07-28"),
];

describe("groupEras", () => {
  it("keeps every release on one ordered line and loses none", () => {
    const eras = groupEras(LINE);

    expect(eras.flatMap(era => era.releases)).toEqual(LINE);
    expect(eras.map(era => era.key)).toEqual(["0.5", "0.4", "0.3"]);
  });

  it("groups betas into one branch era between the stable releases around them", () => {
    const eras = groupEras(LINE);
    const betas = eras.find(era => era.beta);

    expect(betas?.releases.map(release => release.version)).toEqual([
      "v0.3.0-beta.39",
      "v0.3.0-beta.2",
      "v0.3.0-beta.1",
    ]);
    expect(betas?.range).toBe("Jul 2026 to Oct 2026");
    expect(eras.filter(era => !era.beta).map(era => era.range)).toEqual(["Nov 2026", "Oct 2026"]);
  });

  it("does not move a release out of its place when a minor version repeats later", () => {
    const interleaved = [
      at("v0.4.1", "2026-10-25"),
      at("v0.3.0-beta.39", "2026-10-23"),
      at("v0.4.0", "2026-10-22"),
    ];

    expect(groupEras(interleaved).flatMap(era => era.releases)).toEqual(interleaved);
  });
});

type Line = Parameters<typeof stableLine>[0][number];

/** The Stable view as one line of text per item: era versions, or the folded run's summary. */
function describeLine(items: LineItem<Line>[]) {
  return items.map(item =>
    item.kind === "era"
      ? item.era.releases.map(release => release.version).join(" ")
      : `${item.run.count} betas of ${item.run.minor} (${item.run.range}) after ${item.run.after} to ${item.run.target ?? "nothing yet"}${item.run.firstStable ? ", first stable" : ""}`
  );
}

describe("stableLine", () => {
  it("folds the 0.3 betas into the run that led to the first stable release", () => {
    expect(describeLine(stableLine(LINE, 12))).toEqual([
      "v0.5.1 v0.5.0",
      "v0.4.1 v0.4.0",
      "3 betas of 0.3 (Jul to Oct 2026) after v0.2.15 to v0.4.0, first stable",
    ]);
  });

  it("keeps a beta published after the latest stable out of the 0.3 summary", () => {
    const line = [at("v0.6.0-beta.1", "2026-11-12"), ...LINE];

    expect(describeLine(stableLine(line, 12))).toEqual([
      "1 betas of 0.6 (Nov 2026) after v0.5.1 to nothing yet",
      "v0.5.1 v0.5.0",
      "v0.4.1 v0.4.0",
      "3 betas of 0.3 (Jul to Oct 2026) after v0.2.15 to v0.4.0, first stable",
    ]);
  });

  it("branches a beta inside a minor that already has stable releases", () => {
    const line = [
      at("v0.5.2", "2026-11-08"),
      at("v0.5.2-beta.1", "2026-11-06"),
      ...LINE.slice(0, 2),
    ];

    expect(describeLine(stableLine(line, 12))).toEqual([
      "v0.5.2",
      "1 betas of 0.5 (Nov 2026) after v0.5.1 to v0.5.2",
      "v0.5.1 v0.5.0",
    ]);
    const betaEra = groupEras(line).find(era => era.beta);
    expect(betaEra?.releases.map(release => release.version)).toEqual(["v0.5.2-beta.1"]);
  });

  it("does not credit betas to a later patch of an older minor", () => {
    const line = [
      at("v0.5.2", "2026-11-14"),
      at("v0.6.0-beta.1", "2026-11-12"),
      ...LINE.slice(0, 2),
    ];

    expect(describeLine(stableLine(line, 12))[1]).toBe(
      "1 betas of 0.6 (Nov 2026) after v0.5.1 to nothing yet"
    );
  });

  it("shows the fold once the era it merged into is on the page", () => {
    expect(describeLine(stableLine(LINE, 3))).toEqual([
      "v0.5.1 v0.5.0",
      "v0.4.1",
      "3 betas of 0.3 (Jul to Oct 2026) after v0.2.15 to v0.4.0, first stable",
    ]);
    expect(describeLine(stableLine(LINE, 2))).toEqual(["v0.5.1 v0.5.0"]);
  });

  it("adds no beta fold when the page ends inside a minor that is followed by another stable", () => {
    const stables = [
      at("v0.5.2", "2026-11-12"),
      at("v0.5.1", "2026-11-10"),
      at("v0.5.0", "2026-11-05"),
      at("v0.4.0", "2026-10-22"),
    ];

    expect(describeLine(stableLine(stables, 2))).toEqual(["v0.5.2 v0.5.1"]);
  });
});

describe("listState", () => {
  it("reads the channel, search and loaded pages back from the URL", () => {
    expect(listState(new URLSearchParams("betas=1&q=worktree&page=3"))).toEqual({
      betas: true,
      query: "worktree",
      pages: 3,
    });
    expect(listState(new URLSearchParams("page=zero"))).toEqual({
      betas: false,
      query: "",
      pages: 1,
    });
  });
});

describe("parseReleaseNotes", () => {
  const body = [
    "## 0.5.0 - 2026-11-03",
    "",
    "### Bug Fixes",
    "",
    "- Keep permission receipts with their tool (#1158)",
    "",
    "### 🎉 Features",
    "",
    "- Add always-on agents",
    "- Link worktrees and pull requests (#1161, #1162)",
    "",
    "### Release Notes",
    "",
    "This release makes agents stay on.",
    "",
    "#### Breaking Changes",
    "",
    "##### Spec delivery moves to agents",
    "The Loops are gone.",
    "",
    "### Migration",
    "",
    "| Removed | Use instead |",
    "| --- | --- |",
    "| Loop | Agent |",
    "",
    "#### Highlights",
    "",
    "##### Always-on agents",
    "Agents keep watch.",
    "",
    "### Verification",
    "",
    "Checksums are signed.",
  ].join("\n");

  it("keeps every published section: notes in reading order, migrations, change lines and extras", () => {
    const parsed = parseReleaseNotes(body);

    expect(parsed.intro).toEqual(["This release makes agents stay on."]);
    expect(parsed.notes).toEqual([
      { group: "Highlights", title: "Always-on agents", body: "Agents keep watch." },
      {
        group: "Breaking Changes",
        title: "Spec delivery moves to agents",
        body: "The Loops are gone.",
        migration: "| Removed | Use instead |\n| --- | --- |\n| Loop | Agent |",
      },
    ]);
    expect(parsed.commits).toEqual({
      fixes: [{ text: "Keep permission receipts with their tool", prs: ["1158"] }],
      features: [
        { text: "Add always-on agents", prs: [] },
        { text: "Link worktrees and pull requests", prs: ["1161", "1162"] },
      ],
    });
    expect(parsed.extras).toEqual([{ heading: "Verification", body: "Checksums are signed." }]);
  });
});
