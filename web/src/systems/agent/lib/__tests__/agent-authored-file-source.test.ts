import { describe, expect, it } from "vitest";

import type { AgentHeartbeatPayload, AgentSoulPayload } from "../../types";
import {
  serializeAgentHeartbeatSource,
  serializeAgentSoulSource,
} from "../agent-authored-file-source";

describe("agent-authored-file-source", () => {
  it("Should preserve every SOUL.md frontmatter field in the editable source", () => {
    const payload = {
      frontmatter: {
        version: "1",
        role: "release captain",
        tone: ["concise"],
        principles: ["Keep scope tight"],
        constraints: ["No hidden writes"],
        collaboration: ["Report blockers"],
        memory_policy: ["Keep durable facts"],
        tags: ["release"],
      },
      body: "# Soul\n\nCoordinate releases.",
    } as AgentSoulPayload;

    expect(serializeAgentSoulSource(payload)).toBe(
      [
        "---",
        'version: "1"',
        'role: "release captain"',
        "tone:",
        '  - "concise"',
        "principles:",
        '  - "Keep scope tight"',
        "constraints:",
        '  - "No hidden writes"',
        "collaboration:",
        '  - "Report blockers"',
        "memory_policy:",
        '  - "Keep durable facts"',
        "tags:",
        '  - "release"',
        "---",
        "# Soul",
        "",
        "Coordinate releases.",
        "",
      ].join("\n")
    );
  });

  it("Should preserve HEARTBEAT.md policy frontmatter and guidance in the editable source", () => {
    const payload = {
      frontmatter: {
        version: 1,
        enabled: true,
        summary: "Check release state",
        preferences: {
          min_interval: "30m",
          active_hours: [{ timezone: "America/Sao_Paulo", start: "08:00", end: "20:00" }],
          quiet_windows: [{ timezone: "America/Sao_Paulo", start: "22:00", end: "08:00" }],
        },
        context: { include: ["self", "session_health", "task"] },
      },
      guidance_markdown: "# Wake checks\n\nSurface failed sessions.",
    } as AgentHeartbeatPayload;

    const source = serializeAgentHeartbeatSource(payload);
    expect(source).toContain("version: 1\nenabled: true");
    expect(source).toContain('summary: "Check release state"');
    expect(source).toContain('  min_interval: "30m"');
    expect(source).toContain('    - timezone: "America/Sao_Paulo"');
    expect(source).toContain('      start: "08:00"');
    expect(source).toContain('context:\n  include:\n    - "self"');
    expect(source).toContain("---\n# Wake checks\n\nSurface failed sessions.\n");
  });

  it("Should preserve trailing Markdown spaces and blank lines in the body", () => {
    const payload = {
      frontmatter: { version: "1" },
      body: "line with hard break \n\n",
    } as AgentSoulPayload;

    expect(serializeAgentSoulSource(payload)).toBe(
      ["---", 'version: "1"', "---", "line with hard break \n\n"].join("\n")
    );
  });
});
