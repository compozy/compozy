import { render, screen } from "@testing-library/react";
import { palette } from "blobatar";
import { MotionConfig } from "motion/react";
import { describe, expect, it } from "vitest";

import {
  AGENT_AVATAR_STATE_LABEL,
  type AgentAvatarAppearance,
  type AgentAvatarState,
} from "../../../lib/agent-avatar";
import { AGENT_AVATAR_TONES, agentAvatarHue } from "../../../lib/agent-avatar-tones";
import { contrastRatio, parseHexColor } from "../../../lib/contrast";
import { AgentAvatar } from "../agent-avatar";

const GENERATED: AgentAvatarAppearance = { avatar: "generated", seed: "triage", generation: 1 };

function faceSrc(container: HTMLElement) {
  return container.querySelector("img")?.getAttribute("src") ?? "";
}

function slot(container: HTMLElement, name: string) {
  return container.querySelector(`[data-slot="${name}"]`);
}

describe("AgentAvatar", () => {
  it("Should render generated, icon and emoji appearances", () => {
    const { container, rerender } = render(<AgentAvatar appearance={GENERATED} label="Triage" />);
    expect(container.querySelector("img")).not.toBeNull();

    rerender(
      <AgentAvatar
        appearance={{ avatar: "icon", icon: "compass", color: "#2aa8a8" }}
        label="Release"
        spriteUrl="/sprite.svg"
      />
    );
    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector("use")?.getAttribute("href")).toBe("/sprite.svg#compass");

    rerender(
      <AgentAvatar appearance={{ avatar: "emoji", emoji: "🦊", color: "#e8b04a" }} label="Scout" />
    );
    expect(screen.getByText("🦊")).toBeTruthy();
  });

  it("Should speak the name, scope and state, and claim nothing when the state is unknown", () => {
    const { rerender } = render(
      <AgentAvatar appearance={GENERATED} label="Triage" scope="compozy" state="paused" />
    );
    expect(screen.getByRole("img", { name: "Triage, compozy, Paused" })).toBeTruthy();

    rerender(<AgentAvatar appearance={GENERATED} label="Triage" scope="compozy" state="unknown" />);
    expect(screen.getByRole("img", { name: "Triage, compozy" })).toBeTruthy();
  });

  it("Should badge the provider only from 24 px", () => {
    const { container, rerender } = render(
      <AgentAvatar appearance={GENERATED} label="Triage" provider="claude" size={24} />
    );
    expect(slot(container, "agent-avatar-provider")).not.toBeNull();

    rerender(<AgentAvatar appearance={GENERATED} label="Triage" provider="claude" size={20} />);
    expect(slot(container, "agent-avatar-provider")).toBeNull();

    rerender(<AgentAvatar appearance={GENERATED} label="Triage" provider={null} size={32} />);
    expect(slot(container, "agent-avatar-provider")).toBeNull();
  });

  it("Should change the pose with the state and keep unknown on the idle face", () => {
    const pose = (state: AgentAvatarState) =>
      faceSrc(render(<AgentAvatar appearance={GENERATED} label="T" state={state} />).container);

    const faces = (["working", "needs-you", "failed", "paused", "idle"] as const).map(pose);
    expect(new Set(faces).size).toBe(faces.length);
    expect(pose("unknown")).toBe(pose("idle"));
    expect(pose("starting")).toBe(pose("idle"));
  });

  it("Should cap the needs-you triangle at 18 px and show it only then", () => {
    const { container, rerender } = render(
      <AgentAvatar appearance={GENERATED} label="T" size={112} state="needs-you" />
    );
    expect(slot(container, "agent-avatar-needs-you")?.getAttribute("width")).toBe("18");
    expect(slot(container, "agent-avatar-dot")).toBeNull();

    rerender(<AgentAvatar appearance={GENERATED} label="T" size={112} state="idle" />);
    expect(slot(container, "agent-avatar-needs-you")).toBeNull();
  });

  it("Should animate only the working pose, and hold it still under reduced motion", () => {
    const face = (state: AgentAvatarState, reducedMotion: "always" | "never") =>
      faceSrc(
        render(
          <MotionConfig reducedMotion={reducedMotion}>
            <AgentAvatar appearance={GENERATED} label="T" state={state} />
          </MotionConfig>
        ).container
      );

    const states = Object.keys(AGENT_AVATAR_STATE_LABEL) as AgentAvatarState[];
    const moving = states.filter(state => face(state, "never") !== face(state, "always"));
    expect(moving).toEqual(["working"]);
  });

  it("Should derive the same body hue from a saved color", () => {
    expect(agentAvatarHue("#7c5cff")).toBeCloseTo(agentAvatarHue("#7C5CFF"));
  });
});

describe("agent avatar tones", () => {
  const SURFACES = { dark: "#131313", light: "#ffffff" } as const;
  // Large marks, read with a face on top: the bar is "stays visible", not text contrast.
  const TARGET = 2;

  it("Should keep every generation's tones visible on both themes", () => {
    let worst = Number.POSITIVE_INFINITY;
    for (const generation of Object.values(AGENT_AVATAR_TONES)) {
      for (const theme of ["dark", "light"] as const) {
        for (const tone of generation[theme]) {
          for (let hue = 0; hue < 360; hue += 30) {
            const head = parseHexColor(palette(hue, false, tone).head!)!;
            worst = Math.min(worst, contrastRatio(head, parseHexColor(SURFACES[theme])!));
          }
        }
      }
    }
    expect(worst).toBeGreaterThanOrEqual(TARGET);
  });
});
