import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import BlogLayout from "@/app/blog/layout";
import ChangelogLayout from "@/app/changelog/layout";
import type { ReactNode } from "react";

type LayoutProps = {
  children: ReactNode;
  nav?: unknown;
  sidebar?: unknown;
  slots?: Record<string, unknown>;
  tabMode?: string;
  tabs?: unknown;
  tree?: unknown;
};

const mocks = vi.hoisted(() => ({
  homeLayoutCalls: [] as LayoutProps[],
}));

vi.mock("@/components/site/home-header", () => ({
  HomeHeader: () => <header data-testid="home-header" />,
}));

vi.mock("fumadocs-ui/layouts/home", () => ({
  HomeLayout: (props: LayoutProps) => {
    mocks.homeLayoutCalls.push(props);
    return <div data-testid="home-layout">{props.children}</div>;
  },
}));

describe("section layouts", () => {
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
