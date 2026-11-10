import { describe, expect, it } from "vitest";
import { parseReleaseNotes } from "../changelog/release-notes";
import { groupEras } from "../changelog/timeline";

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
