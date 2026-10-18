import { render, screen, waitFor } from "@testing-library/react";
import { Search } from "lucide-react";
import { m, MotionConfigContext, useReducedMotionConfig } from "motion/react";
import { useContext, type ReactNode } from "react";
import { afterAll, beforeAll, describe, expect, it, vi } from "vitest";

import { ICON_STROKE_WIDTH } from "../../../lib/icon-stroke";
import { UIProvider, type UIProviderProps } from "../ui-provider";

const motionPreference = Object.assign(new EventTarget(), { matches: false });
const originalMatchMedia = window.matchMedia;

beforeAll(() => {
  vi.stubGlobal("matchMedia", (query: string) =>
    query.includes("prefers-reduced-motion") ? motionPreference : originalMatchMedia(query)
  );
});

afterAll(() => {
  vi.unstubAllGlobals();
});

function Probe() {
  const reduced = useReducedMotionConfig();
  return <span data-testid="probe">{String(reduced ?? "pending")}</span>;
}

function MotionProbe() {
  return (
    <m.div
      data-testid="motion-probe"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{ duration: 0 }}
    />
  );
}

function SkipAnimationsProbe() {
  const { skipAnimations } = useContext(MotionConfigContext);
  return <span data-testid="skip-animations-probe">{String(skipAnimations ?? false)}</span>;
}

function renderWithProvider(props?: Partial<UIProviderProps>, child: ReactNode = <Probe />) {
  return render(<UIProvider {...props}>{child}</UIProvider>);
}

describe("UIProvider", () => {
  it("Should render children without crashing under the default config", () => {
    renderWithProvider({}, <span data-testid="child">content</span>);
    expect(screen.getByTestId("child")).toHaveTextContent("content");
  });

  it("Should draw every bare lucide icon at the shared stroke width", () => {
    const { container } = renderWithProvider(
      {},
      <>
        <Search data-testid="bare" />
        <Search data-testid="override" strokeWidth={1} />
      </>
    );
    expect(container.querySelector('[data-testid="bare"]')).toHaveAttribute(
      "stroke-width",
      String(ICON_STROKE_WIDTH)
    );
    expect(container.querySelector('[data-testid="override"]')).toHaveAttribute(
      "stroke-width",
      "1"
    );
  });

  it("Should load LazyMotion features for m.* animation primitives", async () => {
    renderWithProvider({ reducedMotion: "never" }, <MotionProbe />);

    await waitFor(() => expect(screen.getByTestId("motion-probe")).toHaveStyle({ opacity: "1" }));
  });

  it("Should forward reducedMotion='always' to MotionConfig consumers", async () => {
    renderWithProvider({ reducedMotion: "always" });
    await waitFor(() => expect(screen.getByTestId("probe")).toHaveTextContent("true"));
  });

  it("Should forward reducedMotion='never' to MotionConfig consumers", async () => {
    renderWithProvider({ reducedMotion: "never" });
    await waitFor(() => expect(screen.getByTestId("probe")).toHaveTextContent("false"));
  });

  it("Should forward skipAnimations without changing reduced-motion semantics", () => {
    renderWithProvider({ reducedMotion: "never", skipAnimations: true }, <SkipAnimationsProbe />);

    expect(screen.getByTestId("skip-animations-probe")).toHaveTextContent("true");
  });

  it.each([false, true])(
    "Should default to reducedMotion='user' and respect OS preference %s",
    async matches => {
      motionPreference.matches = matches;
      motionPreference.dispatchEvent(new Event("change"));
      const warning = vi.spyOn(console, "warn").mockImplementation(() => undefined);
      try {
        renderWithProvider();
        await waitFor(() => expect(screen.getByTestId("probe")).toHaveTextContent(String(matches)));
        if (matches) {
          // Motion intentionally advises developers when this OS preference is active.
          expect(warning).toHaveBeenCalledExactlyOnceWith(
            expect.stringContaining("Reduced Motion enabled")
          );
        } else {
          expect(warning).not.toHaveBeenCalled();
        }
      } finally {
        warning.mockRestore();
      }
    }
  );
});
