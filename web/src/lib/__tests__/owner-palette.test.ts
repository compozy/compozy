import { describe, expect, it } from "vitest";

import {
  AGENT_SLOT_COUNT,
  HUMAN_SLOT_COUNT,
  SYSTEM_SLOT_COUNT,
  colorsFor,
  seed,
} from "../owner-palette";

describe("owner-palette", () => {
  describe("colorsFor", () => {
    it("Should be deterministic for the same (kind, ownerId) pair", () => {
      const a = colorsFor("agent", "planner-prime");
      const b = colorsFor("agent", "planner-prime");
      expect(a).toEqual(b);
    });

    it("Should resolve agent slots to var(--color-avatar-agent-N-{bg,fg}) within slot count", () => {
      const colors = colorsFor("agent", "scout");
      expect(colors.bg).toMatch(/^var\(--color-avatar-agent-[0-3]-bg\)$/);
      expect(colors.fg).toMatch(/^var\(--color-avatar-agent-[0-3]-fg\)$/);
    });

    it("Should resolve human slots to var(--color-avatar-human-N-{bg,fg}) within slot count", () => {
      const colors = colorsFor("human", "pedro");
      expect(colors.bg).toMatch(/^var\(--color-avatar-human-[0-2]-bg\)$/);
      expect(colors.fg).toMatch(/^var\(--color-avatar-human-[0-2]-fg\)$/);
    });

    it("Should resolve system to the single var(--color-avatar-system-{bg,fg}) slot", () => {
      expect(colorsFor("system", "daemon")).toEqual({
        bg: "var(--color-avatar-system-bg)",
        fg: "var(--color-avatar-system-fg)",
      });
      // System slot is identity-independent (only one ramp slot).
      expect(colorsFor("system", "scheduler")).toEqual(colorsFor("system", "daemon"));
    });

    it("Should tolerate empty ownerId without throwing", () => {
      expect(() => colorsFor("agent", "")).not.toThrow();
      const colors = colorsFor("agent", "");
      expect(colors.bg).toMatch(/^var\(--color-avatar-agent-[0-3]-bg\)$/);
    });

    it("Should distribute across the palette for a sample of ownerIds", () => {
      const seen = new Set<string>();
      for (const ownerId of [
        "alpha",
        "beta",
        "gamma",
        "delta",
        "epsilon",
        "zeta",
        "eta",
        "theta",
      ]) {
        seen.add(colorsFor("agent", ownerId).bg);
      }
      expect(seen.size).toBeGreaterThan(1);
    });
  });

  describe("seed", () => {
    it("Should clamp the hash to the agent slot count", () => {
      for (const ownerId of ["a", "b", "c", "d", "e", "f", "g", "h"]) {
        const index = seed("agent", ownerId);
        expect(index).toBeGreaterThanOrEqual(0);
        expect(index).toBeLessThan(AGENT_SLOT_COUNT);
      }
    });

    it("Should clamp the hash to the human slot count", () => {
      for (const ownerId of ["a", "b", "c", "d", "e", "f", "g", "h"]) {
        const index = seed("human", ownerId);
        expect(index).toBeGreaterThanOrEqual(0);
        expect(index).toBeLessThan(HUMAN_SLOT_COUNT);
      }
    });

    it("Should always return 0 for system (single-slot palette)", () => {
      for (const ownerId of ["a", "b", "c"]) {
        expect(seed("system", ownerId)).toBeLessThan(SYSTEM_SLOT_COUNT);
      }
    });
  });
});
