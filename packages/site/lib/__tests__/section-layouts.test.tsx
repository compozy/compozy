import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import BlogLayout from "@/app/blog/layout";
import ChangelogLayout from "@/app/changelog/layout";
import DocsRouteLayout from "@/app/docs/layout";
import { baseOptions } from "../layout.shared";
import type { ReactNode } from "react";

type LayoutProps = {
  children: ReactNode;
  nav?: unknown;
  slots?: Record<string, unknown>;
  tabMode?: string;
  tabs?: unknown;
  tree?: unknown;
};

const mocks = vi.hoisted(() => ({
  docsLayoutCalls: [] as LayoutProps[],
  docsTree: { id: "docs-tree" },
  homeLayoutCalls: [] as LayoutProps[],
}));

vi.mock("@/components/site/docs-header", () => ({
  DocsHeader: () => <header data-testid="docs-header" />,
}));

vi.mock("@/components/site/home-header", () => ({
  HomeHeader: () => <header data-testid="home-header" />,
}));

vi.mock("@/lib/source", () => ({
  docsSource: { pageTree: mocks.docsTree },
}));

vi.mock("fumadocs-ui/layouts/home", () => ({
  HomeLayout: (props: LayoutProps) => {
    mocks.homeLayoutCalls.push(props);
    return <div data-testid="home-layout">{props.children}</div>;
  },
}));

vi.mock("fumadocs-ui/layouts/notebook", () => ({
  DocsLayout: (props: LayoutProps) => {
    mocks.docsLayoutCalls.push(props);
    return <div data-testid="docs-layout">{props.children}</div>;
  },
}));

describe("section layouts", () => {
  it("keeps /docs in the dark Fumadocs shell with the single merged tree and no tab bar", () => {
    render(
      <DocsRouteLayout>
        <p>Docs child</p>
      </DocsRouteLayout>
    );

    expect(screen.getByText("Docs child")).toBeDefined();

    const call = mocks.docsLayoutCalls.at(-1);
    expect(call?.tree).toBe(mocks.docsTree);
    // D3: the CompozyOS / CLI Reference / API Reference sub-tab bar is removed for good.
    expect(call?.tabs).toBeUndefined();
    expect(call?.tabMode).toBeUndefined();
    expect(call?.nav).toMatchObject({ ...baseOptions.nav, mode: "auto" });
    expect(call?.slots?.header).toBeTypeOf("function");
  });

  it("keeps blog and changelog inside the public home shell", () => {
    render(
      <>
        <BlogLayout>
          <p>Blog child</p>
        </BlogLayout>
        <ChangelogLayout>
          <p>Changelog child</p>
        </ChangelogLayout>
      </>
    );

    expect(screen.getByText("Blog child")).toBeDefined();
    expect(screen.getByText("Changelog child")).toBeDefined();
    expect(screen.getAllByTestId("home-layout")).toHaveLength(2);
    expect(mocks.homeLayoutCalls.at(-2)?.slots?.header).toBeTypeOf("function");
    expect(mocks.homeLayoutCalls.at(-1)?.slots?.header).toBeTypeOf("function");
  });
});
