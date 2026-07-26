import { describe, expect, it } from "vitest";

import {
  defaultApiProxyTarget,
  resolveApiProxyOrigin,
  resolveApiProxyTarget,
} from "@/lib/vite-api-proxy-target";

describe("resolveApiProxyTarget", () => {
  it("Should use the default daemon target when no override is set", () => {
    expect(resolveApiProxyTarget({})).toBe(defaultApiProxyTarget);
  });

  it("Should trim and use the override target when provided", () => {
    expect(
      resolveApiProxyTarget({
        AGH_WEB_API_PROXY_TARGET: "  http://127.0.0.1:2255  ",
      })
    ).toBe("http://127.0.0.1:2255/");
  });

  it("Should expose the daemon origin for proxied browser requests", () => {
    expect(
      resolveApiProxyOrigin({
        AGH_WEB_API_PROXY_TARGET: "  http://127.0.0.1:2255  ",
      })
    ).toBe("http://127.0.0.1:2255");
  });

  it("Should reject invalid override values", () => {
    expect(() =>
      resolveApiProxyTarget({
        AGH_WEB_API_PROXY_TARGET: "127.0.0.1:2255",
      })
    ).toThrowError(
      'web: AGH_WEB_API_PROXY_TARGET must be an absolute URL, received "127.0.0.1:2255"'
    );
  });
});
