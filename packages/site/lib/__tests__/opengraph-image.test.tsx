import { describe, expect, it, vi } from "vitest";
import { LogoLockup, SymbolGlyph } from "../og/logo";

const nextOg = vi.hoisted(() => ({
  ImageResponse: class MockImageResponse {
    readonly element: unknown;
    readonly init: unknown;

    constructor(element: unknown, init: unknown) {
      this.element = element;
      this.init = init;
    }
  },
}));

vi.mock("next/og", () => ({
  ImageResponse: nextOg.ImageResponse,
}));

type ElementLike = {
  type?: unknown;
  props?: {
    children?: unknown;
    style?: Record<string, unknown>;
    fill?: unknown;
    d?: unknown;
  };
};

function isElementLike(value: unknown): value is ElementLike {
  return typeof value === "object" && value !== null && "props" in value;
}

function textContent(value: unknown): string {
  if (typeof value === "string" || typeof value === "number") {
    return String(value);
  }
  if (Array.isArray(value)) {
    return value.map(textContent).join(" ");
  }
  if (isElementLike(value)) {
    return textContent(value.props?.children);
  }
  return "";
}

function styles(value: unknown): Array<Record<string, unknown>> {
  if (Array.isArray(value)) {
    return value.flatMap(styles);
  }
  if (!isElementLike(value)) {
    return [];
  }
  return [value.props?.style, ...styles(value.props?.children)].filter(
    (style): style is Record<string, unknown> => style !== undefined
  );
}

function componentTypes(value: unknown): unknown[] {
  if (Array.isArray(value)) {
    return value.flatMap(componentTypes);
  }
  if (!isElementLike(value)) {
    return [];
  }
  const own = value.type !== undefined ? [value.type] : [];
  return [...own, ...componentTypes(value.props?.children)];
}

function styleStrings(value: unknown): string[] {
  return styles(value).flatMap(style =>
    Object.values(style).filter((v): v is string => typeof v === "string")
  );
}

type MockedImageResponse = InstanceType<typeof nextOg.ImageResponse>;

function asMockImageResponse(value: unknown): MockedImageResponse {
  expect(value).toBeInstanceOf(nextOg.ImageResponse);
  return value as MockedImageResponse;
}

const BEAM = "linear-gradient(90deg, #1df0d7, #1f87fc)";
const PALETTE_VALUES = ["#0a0a0a", "#ececec", "#989898", "rgba(255, 255, 255, 0.08)", BEAM];
const RETIRED_HEXES = ["#E8572A", "#141312", "#1b1b1b", "#383838"];

describe("Landing OpenGraph image (root)", () => {
  it("publishes a static 1200x630 PNG response with embedded fonts", async () => {
    const { contentType, default: Image, dynamic, size } = await import("@/app/opengraph-image");
    const response = asMockImageResponse(await Image());

    expect(contentType).toBe("image/png");
    expect(dynamic).toBe("force-static");
    expect(size).toEqual({ width: 1200, height: 630 });
    expect(response.init).toMatchObject({ width: 1200, height: 630 });
    expect((response.init as { fonts: unknown[] }).fonts.length).toBeGreaterThan(0);
  });

  it("renders the locked hero pair and footer rail with the ink palette", async () => {
    const { default: Image } = await import("@/app/opengraph-image");
    const response = asMockImageResponse(await Image());
    const copy = textContent(response.element);
    const styleValues = styleStrings(response.element);
    const types = componentTypes(response.element);

    expect(copy).toContain("The autonomous control plane for your agents.");
    expect(copy).toContain(
      "See and steer every Claude Code, Codex, and Cursor session. Always-on agents check in on their own, and you step in only when one needs you."
    );
    expect(copy).toContain("COMPOZYOS");
    expect(copy).not.toMatch(/\bBETA\b/u);
    expect(copy).toContain("OPERATING SYSTEM FOR AI AGENTS");
    expect(copy).toContain("CREATE · AUTOMATE · SUPERVISE");
    expect(copy).toContain("compozy.com");

    for (const value of PALETTE_VALUES) {
      expect(
        styleValues.some(style => style.includes(value)),
        value
      ).toBe(true);
    }
    for (const hex of RETIRED_HEXES) {
      expect(styleValues).not.toContain(hex);
    }

    expect(types).toContain(LogoLockup);
    expect(types).toContain(SymbolGlyph);

    expect(styleValues).toContain("Instrument Sans");
    expect(styleValues).toContain("Figtree");
    expect(styleValues).toContain("Geist Mono");
    expect(styleValues).not.toContain("Playfair Display");
    expect(styleValues).not.toContain("JetBrains Mono");
  });
});

describe("Docs OpenGraph template", () => {
  it("renders the docs eyebrow, technical path, and Instrument Sans title on the beam palette", async () => {
    const { renderDocsOG } = await import("@/lib/og/templates/docs");
    const response = asMockImageResponse(
      await renderDocsOG({
        variant: "docs",
        title: "Sessions and lifecycle",
        description: "How CompozyOS durably runs ACP-compatible agents end to end.",
        path: "docs/sessions/lifecycle",
      })
    );
    const copy = textContent(response.element);
    const styleValues = styleStrings(response.element);
    const types = componentTypes(response.element);

    expect(copy).toContain("COMPOZYOS DOCS");
    expect(copy).toContain("docs/sessions/lifecycle");
    expect(copy).toContain("Sessions and lifecycle");
    expect(copy).toContain("How CompozyOS durably runs ACP-compatible agents end to end.");
    expect(copy).toContain("DOCS");
    expect(copy).toContain("COMPOZYOS");
    expect(copy).toContain("compozy.com");

    expect(types).toContain(LogoLockup);
    expect(styleValues).toContain(BEAM);
    expect(styleValues).toContain("#0a0a0a");
    expect(styleValues).toContain("Figtree");
    expect(styleValues).toContain("Instrument Sans");
  });
});

describe("Blog OpenGraph template", () => {
  it("renders editorial title, formatted date, slug-based footer, and accent rail", async () => {
    const { renderBlogOG } = await import("@/lib/og/templates/blog");
    const response = asMockImageResponse(
      await renderBlogOG({
        title: "Introducing CompozyOS, the local agent operating system",
        description:
          "CompozyOS gives every agent CLI a durable home and a shared protocol to coordinate with peers.",
        slug: "introducing-compozyos",
        date: "2026-04-29",
        author: "pnauck",
      })
    );
    const copy = textContent(response.element);
    const styleValues = styleStrings(response.element);
    const types = componentTypes(response.element);

    expect(copy).toContain("COMPOZYOS BLOG");
    expect(copy).toContain("Introducing CompozyOS, the local agent operating system");
    expect(copy).toContain("APR 29, 2026");
    expect(copy).toContain("compozy.com/blog/introducing-compozyos");
    expect(copy).toContain("BY pnauck");

    expect(types).toContain(LogoLockup);
    expect(styleValues).toContain("Instrument Sans");
    expect(styleValues).toContain(BEAM);
  });

  it("hides date and author bits when not provided", async () => {
    const { renderBlogOG } = await import("@/lib/og/templates/blog");
    const response = asMockImageResponse(
      await renderBlogOG({
        title: "Untitled draft",
        slug: "draft-without-date",
      })
    );
    const copy = textContent(response.element);
    expect(copy).toContain("COMPOZYOS BLOG");
    expect(copy).not.toContain("BY ");
    expect(copy).toContain("compozy.com/blog/draft-without-date");
  });
});
