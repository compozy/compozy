import { cleanup, render, screen } from "@testing-library/react";
import type { AnchorHTMLAttributes, ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import ErrorPage from "@/app/error";
import HomePage from "@/app/(home)/page";
import NotFound from "@/app/not-found";
import BlogIndexPage from "@/app/blog/page";
import BlogPostPage from "@/app/blog/[slug]/page";
import CategoryArchivePage from "@/app/blog/categories/[category]/page";
import ChangelogPage from "@/app/changelog/page";
import { BLOG_CATEGORIES, allPosts, postsByCategory } from "@/lib/blog";

vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    ...props
  }: {
    href: string;
    children: ReactNode;
  } & Omit<AnchorHTMLAttributes<HTMLAnchorElement>, "href">) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

vi.mock("next/navigation", () => ({
  usePathname: () => "/",
}));

vi.mock("@/components/landing/hero-player", () => ({
  HeroPlayer: () => <div data-testid="hero-player" />,
}));

afterEach(() => cleanup());

function expectSingleH1(name: string | RegExp): void {
  const headings = screen.getAllByRole("heading", { level: 1 });
  expect(headings.map(heading => heading.textContent?.trim())).toHaveLength(1);
  expect(screen.getByRole("heading", { level: 1, name })).toBeDefined();
}

describe("public heading hierarchy", () => {
  it("keeps the landing page to one primary heading", () => {
    render(<HomePage />);

    expectSingleH1("An open workplace for AI agents.");
  }, 15_000);

  it("keeps blog index and category archive pages to one primary heading", async () => {
    render(<BlogIndexPage />);
    expectSingleH1("The runtime, the protocol, the receipts.");
    cleanup();

    const category =
      BLOG_CATEGORIES.find(candidate => postsByCategory(candidate).length === 0) ??
      BLOG_CATEGORIES[0];
    render(await CategoryArchivePage({ params: Promise.resolve({ category }) }));
    expectSingleH1(new RegExp(`^${category}`, "i"));
  });

  it("keeps generated blog article pages to one primary heading", async () => {
    const post = allPosts()[0];
    const slug = post.slug.replace(/^posts\//, "");

    render(await BlogPostPage({ params: Promise.resolve({ slug }) }));

    expectSingleH1(post.title);
  });

  it("keeps changelog and fallback pages to one primary heading", () => {
    render(<ChangelogPage />);
    expectSingleH1("Every alpha, on the wire.");
    cleanup();

    render(<NotFound />);
    expectSingleH1("This route is not in the runtime.");
    cleanup();

    render(<ErrorPage error={new Error("render failed")} reset={vi.fn()} />);
    expectSingleH1("The site hit a recoverable boundary.");
  });
});
