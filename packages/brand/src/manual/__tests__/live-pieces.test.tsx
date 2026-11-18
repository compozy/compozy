// @vitest-environment jsdom
// UT-005: live product pieces render @compozy/ui exports, and nothing under src/manual/ paints a
// hard-coded hex colour through an inline style (US-004.AC-3, ADR-005).
import { AgentAvatar, StateGlyph } from "@compozy/ui";
import { cleanup, render } from "@testing-library/react";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { afterEach, describe, expect, it } from "vitest";

import { AgentFace, ApprovalRow, StateRow } from "../art/pieces";
import { productShotSrc } from "../art/product-pins";
import { AGENT_STATES, agentByName, harnessLabel, STATE_GLYPH } from "../art/roster";
import { Visual } from "../art/visual";
import { visuals } from "../art/visuals";

afterEach(cleanup);

const HEX = /#[0-9a-f]{3,8}\b/i;
// jsdom rewrites import.meta.url to an http URL, so resolve from the test directory instead.
const MANUAL = join(import.meta.dirname, "..");

function sources(dir: string): string[] {
  return readdirSync(dir).flatMap(f => {
    const p = join(dir, f);
    if (statSync(p).isDirectory()) return f === "__tests__" ? [] : sources(p);
    return /\.tsx?$/.test(f) ? [p] : [];
  });
}

describe("live product pieces", () => {
  it("renders an agent face as the product AgentAvatar seeded by the agent name", () => {
    const agent = agentByName("qa-runner");
    const piece = render(<AgentFace name="qa-runner" size={40} />).container.innerHTML;
    cleanup();
    const product = render(
      <AgentAvatar
        appearance={{ avatar: "generated", seed: "qa-runner" }}
        label="qa-runner"
        scope={harnessLabel[agent.harness]}
        state={agent.state}
        provider={agent.harness}
        size={40}
      />
    ).container.innerHTML;
    expect(piece).toBe(product);
    expect(piece).toContain("Needs you");
  });

  it("renders one product StateGlyph per documented state", () => {
    const { container } = render(<StateRow faces={false} />);
    const rows = container.querySelectorAll("[data-agent-state]");
    expect([...rows].map(r => r.getAttribute("data-agent-state"))).toEqual([...AGENT_STATES]);
    for (const row of rows) {
      const state = row.getAttribute("data-agent-state") as (typeof AGENT_STATES)[number];
      const glyph = render(<StateGlyph state={STATE_GLYPH[state]} still />).container.innerHTML;
      expect(row.querySelector("svg")?.outerHTML).toBe(glyph);
    }
  });

  it("shows the approval row as the captured permission dock, never a redraw", () => {
    const { container } = render(<ApprovalRow theme="light" />);
    expect(container.querySelector("img")?.getAttribute("src")).toBe(
      productShotSrc("approval", "light")
    );
    expect(container.querySelectorAll("button")).toHaveLength(0);
  });

  // Faces carry Blobatar's generated per-seed colours inside @compozy/ui; that is product
  // output, so the rendered check covers the manual's own art (shots and diagrams).
  it("paints no inline hex colour in any rendered visual", () => {
    const { container } = render(
      <>
        {visuals.map(v => (
          <Visual key={v.name} name={v.name} theme="light" />
        ))}
      </>
    );
    const styled = [...container.querySelectorAll("[style]")].map(e => e.getAttribute("style"));
    expect(styled.length).toBeGreaterThan(0);
    expect(styled.filter(s => HEX.test(s ?? ""))).toEqual([]);
  });

  it("keeps hex out of style props in every manual source file", () => {
    const offenders = sources(MANUAL).filter(file =>
      [...readFileSync(file, "utf8").matchAll(/style=\{\{([\s\S]*?)\}\}/g)].some(m =>
        HEX.test(m[1] ?? "")
      )
    );
    expect(offenders).toEqual([]);
  });
});
