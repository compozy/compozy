import { describe, expect, it } from "vitest";
import {
  buildChangelogRelease,
  parseGitCliffContext,
  parseReleaseNoteMarkdown,
  renderReleaseMdx,
  type GitCliffRelease,
} from "../generate-changelog-release";

describe("generate changelog release", () => {
  it("Should map git-cliff context and release notes into Velite frontmatter fields", () => {
    const noteContent = [
      "---",
      "title: Operator upgrade path",
      "type: highlight",
      "summary: Clear upgrade guidance for operators.",
      "---",
      "",
      "Operators can follow the release notes during rollout.",
      "",
    ].join("\n");
    const releaseNotes = [
      parseReleaseNoteMarkdown(".release-notes/operator-upgrade.md", noteContent),
    ].filter(note => note !== null);
    const entry = buildChangelogRelease({
      version: "0.8.0",
      generatedAt: "2026-05-18T12:00:00.000Z",
      previousTag: "v0.7.0",
      githubOwner: "compozy",
      githubRepo: "agh",
      releaseNotes,
      context: [
        {
          version: "v0.8.0",
          timestamp: 1779124636,
          previousVersion: "v0.7.0",
          commits: [
            {
              message: "add workspace-aware changelog pages",
              group: "Features",
              breaking: false,
              scope: "site",
              rawMessage: "feat(site): add workspace-aware changelog pages",
            },
            {
              message: "fix release notes body selection",
              group: "Bug Fixes",
              breaking: false,
              rawMessage: "fix: fix release notes body selection",
            },
            {
              message: "document release automation",
              group: "Documentation",
              breaking: false,
              rawMessage: "docs: document release automation",
            },
            {
              message: "exercise internal fixtures",
              group: "Testing",
              breaking: false,
              rawMessage: "test: exercise internal fixtures",
            },
          ],
        },
      ],
    });

    expect(entry).toMatchObject({
      version: "v0.8.0",
      date: "2026-05-18T12:00:00.000Z",
      status: "alpha",
      summary: "Clear upgrade guidance for operators.",
      compareUrl: "https://github.com/compozy/agh/compare/v0.7.0...v0.8.0",
      added: ["Operator upgrade path", "site: Add workspace-aware changelog pages"],
      fixed: ["Fix release notes body selection"],
      changed: ["Document release automation"],
      breaking: [],
    });
    expect(entry.added).not.toContain("Exercise internal fixtures");
    expect(entry.body).toContain("Operators can follow the release notes during rollout.");
  });

  it("Should mark a release as breaking when git-cliff reports breaking commits", () => {
    const context: GitCliffRelease[] = [
      {
        version: "v1.0.0",
        commits: [
          {
            message: "remove legacy channel identifiers",
            group: "Features",
            breaking: true,
            breakingDescription: "Network subjects now require workspace identifiers.",
            rawMessage: "feat!: remove legacy channel identifiers",
          },
        ],
      },
    ];

    const entry = buildChangelogRelease({
      version: "v1.0.0",
      generatedAt: "2026-05-18T12:00:00.000Z",
      context,
    });

    expect(entry.status).toBe("breaking");
    expect(entry.breaking).toEqual(["Network subjects now require workspace identifiers."]);
    expect(entry.added).toEqual([]);
  });

  it("Should render MDX with scalar and array frontmatter accepted by Velite", () => {
    const mdx = renderReleaseMdx({
      version: "v1.0.0",
      date: "2026-05-18T12:00:00.000Z",
      status: "stable",
      summary: "Release v1.0.0",
      added: ["Agent lifecycle support"],
      changed: [],
      fixed: ["Release body selection"],
      breaking: [],
      compareUrl: "https://github.com/compozy/agh/releases/tag/v1.0.0",
      body: "Generated from release artifacts for v1.0.0.",
    });

    expect(mdx).toContain('version: "v1.0.0"');
    expect(mdx).toContain('added:\n  - "Agent lifecycle support"');
    expect(mdx).toContain("changed: []");
    expect(mdx).toContain('compareUrl: "https://github.com/compozy/agh/releases/tag/v1.0.0"');
  });

  it("Should append objective verification posture to generated release bodies", () => {
    const entry = buildChangelogRelease({
      version: "v0.9.0",
      generatedAt: "2026-05-18T12:00:00.000Z",
      context: [{ version: "v0.9.0", commits: [] }],
    });

    expect(entry.body).toContain("## Verification posture");
    expect(entry.body).toContain("`checksums.txt.sigstore.json`");
    expect(entry.body).toContain("Syft SBOMs for archives, packages, and source");
    expect(entry.body).toContain(
      "Known limitation: this generated changelog does not claim a manual post-release install smoke"
    );
    expect(entry.body.toLowerCase()).not.toContain("production-ready");
  });

  it("Should parse the git-cliff context shape used by the release hook", () => {
    const releases = parseGitCliffContext(
      JSON.stringify([
        {
          version: "v0.8.0",
          timestamp: 1779124636,
          previous: { version: "v0.7.0" },
          commits: [
            {
              message: "add release page",
              group: "Features",
              breaking: false,
              breaking_description: null,
              scope: "site",
              raw_message: "feat(site): add release page",
            },
          ],
        },
      ])
    );

    expect(releases).toEqual([
      {
        version: "v0.8.0",
        timestamp: 1779124636,
        previousVersion: "v0.7.0",
        commits: [
          {
            message: "add release page",
            group: "Features",
            breaking: false,
            scope: "site",
            rawMessage: "feat(site): add release page",
          },
        ],
      },
    ]);
  });
});
