import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { siteRoot } from "../content-test-utils";

const headersPath = resolve(siteRoot, "public/_headers");

type HeaderBlock = {
  route: string;
  headers: Map<string, string>;
};

function parseHeadersFile(content: string): HeaderBlock[] {
  const blocks: HeaderBlock[] = [];
  let current: HeaderBlock | null = null;

  for (const line of content.split("\n")) {
    if (line.trim() === "") {
      continue;
    }
    if (!line.startsWith(" ")) {
      current = { route: line.trim(), headers: new Map() };
      blocks.push(current);
      continue;
    }
    if (!current) {
      continue;
    }

    const trimmed = line.trim();
    const separatorIndex = trimmed.indexOf(":");
    if (separatorIndex === -1) {
      continue;
    }
    current.headers.set(trimmed.slice(0, separatorIndex), trimmed.slice(separatorIndex + 1).trim());
  }

  return blocks;
}

function headersByRoute(): Map<string, Map<string, string>> {
  return new Map(
    parseHeadersFile(readFileSync(headersPath, "utf8")).map(block => [block.route, block.headers])
  );
}

function cspDirectives(value: string): Map<string, string[]> {
  return new Map(
    value
      .split(";")
      .map(part => part.trim())
      .filter(Boolean)
      .map(part => {
        const [directive, ...tokens] = part.split(/\s+/);
        return [directive ?? "", tokens];
      })
  );
}

describe("public security headers", () => {
  it("keeps the global security header block strict for public pages", () => {
    const globalHeaders = headersByRoute().get("/*");
    expect(globalHeaders).toBeDefined();

    const csp = cspDirectives(globalHeaders?.get("Content-Security-Policy") ?? "");
    expect(csp.get("default-src")).toEqual(["'self'"]);
    expect(csp.get("base-uri")).toEqual(["'self'"]);
    expect(csp.get("object-src")).toEqual(["'none'"]);
    expect(csp.get("frame-ancestors")).toEqual(["'none'"]);
    expect(csp.get("form-action")).toEqual(["'self'"]);
    expect(csp.get("font-src")).toEqual(["'self'"]);
    expect(csp.get("connect-src")).toEqual(["'self'"]);
    expect(csp.get("img-src")).toEqual(["'self'", "data:", "blob:"]);
    expect(csp.get("media-src")).toEqual(["'self'", "data:"]);
    expect(csp.get("script-src")).toEqual(["'self'", "'unsafe-inline'"]);
    expect(csp.get("style-src")).toEqual(["'self'", "'unsafe-inline'"]);
    expect([...csp.values()].flat()).not.toContain("'unsafe-eval'");

    expect(globalHeaders?.get("Referrer-Policy")).toBe("strict-origin-when-cross-origin");
    expect(globalHeaders?.get("X-Content-Type-Options")).toBe("nosniff");
    expect(globalHeaders?.get("X-Frame-Options")).toBe("DENY");
    expect(globalHeaders?.get("Permissions-Policy")).toBe(
      "camera=(), microphone=(), geolocation=(), payment=(), usb=()"
    );
  });

  it("keeps install.sh served as a short-lived plain-text script", () => {
    const installHeaders = headersByRoute().get("/install.sh");
    expect(installHeaders).toBeDefined();
    expect(installHeaders?.get("Content-Type")).toBe("text/plain; charset=utf-8");
    expect(installHeaders?.get("Cache-Control")).toBe("public, max-age=300, must-revalidate");
    expect(installHeaders?.has("Content-Security-Policy")).toBe(false);
    expect(installHeaders?.has("X-Content-Type-Options")).toBe(false);
  });

  it("declares content types for static export assets without reliable local inference", () => {
    const openGraphHeaders = headersByRoute().get("/opengraph-image");
    expect(openGraphHeaders).toBeDefined();
    expect(openGraphHeaders?.get("Content-Type")).toBe("image/png");

    const feedHeaders = headersByRoute().get("/blog/feed.xml");
    expect(feedHeaders).toBeDefined();
    expect(feedHeaders?.get("Content-Type")).toBe("application/rss+xml; charset=utf-8");
  });
});
