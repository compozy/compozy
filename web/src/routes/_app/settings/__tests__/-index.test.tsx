import { describe, expect, it, vi } from "vitest";

const redirectMock = vi.fn((payload: { to: string }) => payload);

vi.mock("@tanstack/react-router", () => ({
  createFileRoute: () => (opts: { beforeLoad?: () => void; component: () => unknown }) => ({
    beforeLoad: opts.beforeLoad,
    component: opts.component,
  }),
  redirect: (payload: { to: string }) => redirectMock(payload),
}));

import { routeBeforeLoad } from "@/test/route-options";

import { Route as RetiredDefaultsRoute } from "../defaults";
import { Route as RetiredGatewayRoute } from "../gateway";
import { Route } from "../index";

const beforeLoad = routeBeforeLoad(Route);

describe("SettingsIndexRedirect", () => {
  it("redirects the default settings route to the general section", () => {
    expect(() => beforeLoad()).toThrow();
    expect(redirectMock).toHaveBeenCalledWith({ to: "/settings/general" });
  });
});

describe("RetiredDefaultsRedirect", () => {
  it("Should replace old Defaults links with the General section", () => {
    // Invariant: bookmarks and window routes naming /settings/defaults never 404 (#814).
    expect(() => routeBeforeLoad(RetiredDefaultsRoute)()).toThrow();
    expect(redirectMock).toHaveBeenCalledWith({ to: "/settings/general", replace: true });
  });
});

describe("RetiredGatewayRedirect", () => {
  it("Should replace old Remote access (/settings/gateway) links with Computers", () => {
    // Invariant: bookmarks and window routes naming /settings/gateway redirect to /settings/computers (U1, U10).
    expect(() => routeBeforeLoad(RetiredGatewayRoute)()).toThrow();
    expect(redirectMock).toHaveBeenCalledWith({ to: "/settings/computers", replace: true });
  });
});
