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

import { Route } from "../index";

const beforeLoad = routeBeforeLoad(Route);

describe("SettingsIndexRedirect", () => {
  it("redirects the default settings route to the general section", () => {
    expect(() => beforeLoad()).toThrow();
    expect(redirectMock).toHaveBeenCalledWith({ to: "/settings/general" });
  });
});
