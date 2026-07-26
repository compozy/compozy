import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MotionConfig } from "motion/react";
import type { MouseEvent, ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { Pill, type PillSize, type PillTone } from "../pill";

interface WithMotionProps {
  reducedMotion: "always" | "never";
  children: ReactNode;
}

interface DotExpectation {
  tone: PillTone;
  className: string;
}

const SIZES: PillSize[] = ["xs", "sm", "md"];

function WithMotion({ reducedMotion, children }: WithMotionProps) {
  return <MotionConfig reducedMotion={reducedMotion}>{children}</MotionConfig>;
}

describe("Pill", () => {
  it("Should render a neutral span at sm size by default", () => {
    render(<Pill>label</Pill>);
    const pill = screen.getByText("label");
    expect(pill.tagName).toBe("SPAN");
    expect(pill).toHaveAttribute("data-slot", "pill");
    expect(pill).toHaveAttribute("data-tone", "neutral");
    expect(pill).toHaveAttribute("data-size", "sm");
  });

  it.each<PillTone>(["accent", "success", "warning", "danger", "info"])(
    "Should expose data-tone for tone=%s",
    tone => {
      render(<Pill tone={tone}>x</Pill>);
      const pill = screen.getByText("x");
      expect(pill).toHaveAttribute("data-tone", tone);
    }
  );

  it("Should expose data-solid when solid is true", () => {
    render(
      <Pill tone="accent" solid>
        NEW
      </Pill>
    );
    const pill = screen.getByText("NEW");
    expect(pill).toHaveAttribute("data-solid", "true");
  });

  it("Should expose data-mono when mono is true", () => {
    render(<Pill mono>token</Pill>);
    const pill = screen.getByText("token");
    expect(pill).toHaveAttribute("data-mono", "true");
  });

  it.each<PillSize>(SIZES)("Should expose data-size for size=%s", size => {
    render(<Pill size={size}>label-{size}</Pill>);
    const pill = screen.getByText(content => content.startsWith("label-"));
    expect(pill).toHaveAttribute("data-size", size);
  });

  it("Should render as a button when render={<button />} is provided", async () => {
    const user = userEvent.setup();
    const handle = vi.fn();
    render(<Pill render={<button type="button" onClick={handle} />}>FILTER</Pill>);
    const pill = screen.getByRole("button", { name: /filter/i });
    expect(pill.tagName).toBe("BUTTON");
    await user.click(pill);
    expect(handle).toHaveBeenCalledTimes(1);
  });

  it("Should expose data-active=true when active=true", () => {
    render(
      <Pill mono active render={<button type="button" />}>
        FILTER
      </Pill>
    );
    const pill = screen.getByRole("button", { name: /filter/i });
    expect(pill).toHaveAttribute("data-active", "true");
  });

  it("Should expose data-active=false when active=false", () => {
    render(
      <Pill mono active={false} render={<button type="button" />}>
        FILTER
      </Pill>
    );
    const pill = screen.getByRole("button", { name: /filter/i });
    expect(pill).toHaveAttribute("data-active", "false");
  });

  it("Should forward className alongside the variant defaults", () => {
    render(<Pill className="custom">label</Pill>);
    const pill = screen.getByText("label");
    expect(pill.className).toContain("custom");
  });

  it("Should expose Pill.Link as an accessible anchor chip", async () => {
    const user = userEvent.setup();
    const handle = vi.fn((event: MouseEvent<HTMLAnchorElement>) => event.preventDefault());
    render(
      <Pill.Link href="/tasks/task-1" onClick={handle}>
        Open task
      </Pill.Link>
    );

    const link = screen.getByRole("link", { name: /open task/i });
    expect(link).toHaveAttribute("href", "/tasks/task-1");
    expect(link).toHaveAttribute("data-slot", "pill");
    expect(link).toHaveAttribute("data-tone", "accent");
    expect(link).toHaveAttribute("data-mono", "true");
    await user.click(link);
    expect(handle).toHaveBeenCalledTimes(1);
  });
});

describe("Pill.Dot", () => {
  it("Should render md dot keyed to neutral by default", () => {
    const { container } = render(<Pill.Dot />);
    const dot = container.querySelector<HTMLElement>('[data-slot="pill-dot"]');
    expect(dot).not.toBeNull();
    expect(dot?.getAttribute("data-tone")).toBe("neutral");
    expect(dot?.getAttribute("data-size")).toBe("md");
    expect(dot?.className).toContain("bg-muted");
    expect(dot?.style.backgroundColor).toBe("");
    expect(dot?.getAttribute("aria-hidden")).toBe("true");
  });

  it.each<DotExpectation>([
    { tone: "accent", className: "bg-accent" },
    { tone: "success", className: "bg-success" },
    { tone: "warning", className: "bg-warning" },
    { tone: "danger", className: "bg-danger" },
    { tone: "info", className: "bg-info" },
  ])("Should map tone %s to the semantic color utility", ({ tone, className }) => {
    const { container } = render(<Pill.Dot tone={tone} />);
    const dot = container.querySelector<HTMLElement>('[data-slot="pill-dot"]');
    expect(dot?.className).toContain(className);
    expect(dot?.style.backgroundColor).toBe("");
  });

  it("Should let an explicit color override the tone-derived background", () => {
    const { container } = render(<Pill.Dot tone="success" color="#5BA6FF" />);
    const dot = container.querySelector<HTMLElement>('[data-slot="pill-dot"]');
    expect(dot?.style.backgroundColor).toBe("rgb(91, 166, 255)");
    expect(dot?.className).not.toContain("bg-success");
  });

  it("Should expose data-pulse only when reduced motion is off", () => {
    const { container, rerender } = render(
      <WithMotion reducedMotion="never">
        <Pill.Dot tone="success" pulse />
      </WithMotion>
    );
    let dot = container.querySelector<HTMLElement>('[data-slot="pill-dot"]');
    expect(dot?.getAttribute("data-pulse")).toBe("true");

    rerender(
      <WithMotion reducedMotion="always">
        <Pill.Dot tone="success" pulse />
      </WithMotion>
    );
    dot = container.querySelector<HTMLElement>('[data-slot="pill-dot"]');
    expect(dot?.getAttribute("data-pulse")).toBeNull();
  });

  it("Should derive its size from the parent Pill context", () => {
    const { container } = render(
      <Pill size="md">
        <Pill.Dot />
        FILTER
      </Pill>
    );
    const dot = container.querySelector<HTMLElement>('[data-slot="pill-dot"]');
    expect(dot?.getAttribute("data-size")).toBe("md");
  });

  it("Should fall back to a small dot when nested inside a sm Pill", () => {
    const { container } = render(
      <Pill size="sm">
        <Pill.Dot />
        Online
      </Pill>
    );
    const dot = container.querySelector<HTMLElement>('[data-slot="pill-dot"]');
    expect(dot?.getAttribute("data-size")).toBe("sm");
  });

  it("Should inherit pulse from the parent Pill context", () => {
    const { container } = render(
      <WithMotion reducedMotion="never">
        <Pill pulse>
          <Pill.Dot />
          Online
        </Pill>
      </WithMotion>
    );
    const dot = container.querySelector<HTMLElement>('[data-slot="pill-dot"]');
    expect(dot?.getAttribute("data-pulse")).toBe("true");
  });

  it("Should respect an explicit size prop over the parent context", () => {
    const { container } = render(
      <Pill size="md">
        <Pill.Dot size="sm" />
        Online
      </Pill>
    );
    const dot = container.querySelector<HTMLElement>('[data-slot="pill-dot"]');
    expect(dot?.getAttribute("data-size")).toBe("sm");
  });
});
