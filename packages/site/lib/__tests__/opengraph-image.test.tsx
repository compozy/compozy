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

const PALETTE_HEXES = ["#141312", "#1E1C1B", "#3C3A39", "#E5E5E7", "#8E8E93", "#E8572A"];

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

  it("renders the locked OS definition and beta footer rail with the warm-dark palette", async () => {
    const { default: Image } = await import("@/app/opengraph-image");
    const response = asMockImageResponse(await Image());
    const copy = textContent(response.element);
    const styleValues = styleStrings(response.element);
    const types = componentTypes(response.element);

    expect(copy).toContain("The only true OS for AI agents.");
    expect(copy).toContain(
      "A window on top of an agent isn't an OS. An OS runs the work, keeps the memory, sets the permissions, connects agents to each other — and lets you build on it. That's the test, and Compozy is the only one built to pass it."
    );
    expect(copy).toContain("COMPOZYOS / BETA");
    expect(copy).toContain("OPERATING SYSTEM FOR AI AGENTS");
    expect(copy).toContain("RUN · MEMORY · POLICY · CONNECTION");
    expect(copy).toContain("compozy.com");

    for (const hex of PALETTE_HEXES) {
      expect(styleValues).toContain(hex);
    }
    expect(styleValues).not.toContain("#E8572B");

    expect(types).toContain(LogoLockup);
    expect(types).toContain(SymbolGlyph);

    const usesPlayfair = styleValues.some(value => value.includes("Playfair Display"));
    expect(usesPlayfair).toBe(true);
    const usesMono = styleValues.some(value => value.includes("JetBrains Mono"));
    expect(usesMono).toBe(true);
  });
});

describe("Docs OpenGraph template", () => {
  it("renders the docs eyebrow, technical path, and Geist title", async () => {
    const { renderDocsOG } = await import("@/lib/og/templates/docs");
    const response = asMockImageResponse(
      await renderDocsOG({
        variant: "docs",
        title: "Sessions and lifecycle",
        description: "How Compozy durably runs ACP-compatible agents end to end.",
        path: "docs/sessions/lifecycle",
      })
    );
    const copy = textContent(response.element);
    const styleValues = styleStrings(response.element);
    const types = componentTypes(response.element);

    expect(copy).toContain("COMPOZYOS DOCS");
    expect(copy).toContain("docs/sessions/lifecycle");
    expect(copy).toContain("Sessions and lifecycle");
    expect(copy).toContain("How Compozy durably runs ACP-compatible agents end to end.");
    expect(copy).toContain("DOCS");
    expect(copy).toContain("COMPOZYOS");
    expect(copy).toContain("compozy.com");

    expect(types).toContain(LogoLockup);
    expect(styleValues).toContain("#E8572A");
    expect(styleValues).toContain("#141312");

    const usesPlayfair = styleValues.some(value => value.includes("Playfair Display"));
    expect(usesPlayfair).toBe(false);
  });

  it("uses the protocol eyebrow for pages nested under the protocol spec", async () => {
    const { renderDocsOG } = await import("@/lib/og/templates/docs");
    const response = asMockImageResponse(
      await renderDocsOG({
        variant: "protocol",
        title: "Envelopes and channels",
        path: "docs/network/protocol/envelope",
      })
    );
    const copy = textContent(response.element);
    expect(copy).toContain("COMPOZY NETWORK PROTOCOL");
    expect(copy).toContain("PROTOCOL");
  });
});

describe("Blog OpenGraph template", () => {
  it("renders editorial title, formatted date, slug-based footer, and accent rail", async () => {
    const { renderBlogOG } = await import("@/lib/og/templates/blog");
    const response = asMockImageResponse(
      await renderBlogOG({
        title: "Introducing Compozy, the first agent network protocol",
        description:
          "Compozy gives every agent CLI a durable home and a shared protocol to coordinate with peers.",
        slug: "introducing-compozyos",
        date: "2026-04-29",
        author: "pnauck",
      })
    );
    const copy = textContent(response.element);
    const styleValues = styleStrings(response.element);
    const types = componentTypes(response.element);

    expect(copy).toContain("COMPOZY BLOG");
    expect(copy).toContain("Introducing Compozy, the first agent network protocol");
    expect(copy).toContain("APR 29, 2026");
    expect(copy).toContain("compozy.com/blog/introducing-compozyos");
    expect(copy).toContain("BY pnauck");

    expect(types).toContain(LogoLockup);
    const usesPlayfair = styleValues.some(value => value.includes("Playfair Display"));
    expect(usesPlayfair).toBe(true);
    expect(styleValues).toContain("#E8572A");
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
    expect(copy).toContain("COMPOZY BLOG");
    expect(copy).not.toContain("BY ");
    expect(copy).toContain("compozy.com/blog/draft-without-date");
  });
});
