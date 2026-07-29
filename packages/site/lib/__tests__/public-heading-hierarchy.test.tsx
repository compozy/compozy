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

// The hero visual is a Remotion <Player>; jsdom cannot drive its frame loop and the heading
// hierarchy does not depend on it. components/landing/__tests__/landing.test.tsx owns the
// player's own contract.
vi.mock("@/components/landing/hero-player", () => ({
  HeroPlayer: () => <div data-testid="hero-player" />,
}));

afterEach(() => cleanup());

function expectSingleH1(name: string | RegExp): void {
  const headings = screen.getAllByRole("heading", { level: 1 });
  expect(headings.map(heading => heading.textContent?.trim())).toHaveLength(1);
  expect(screen.getByRole("heading", { level: 1, name })).toBeDefined();
}

/** Document-order heading levels must never jump by more than one (h1 → h2 → h3). */
function expectNoSkippedHeadingLevels(): void {
  const levels = Array.from(document.querySelectorAll("h1, h2, h3, h4, h5, h6")).map(heading =>
    Number(heading.tagName.slice(1))
  );
  const jumps = levels.filter((level, index) => index > 0 && level - levels[index - 1]! > 1);
  expect(jumps).toEqual([]);
}

describe("public heading hierarchy", () => {
  it("keeps the landing page to one primary heading", () => {
    render(<HomePage />);

    // The h1 is the locked launch headline (COPY.md §2). Section h2 copy is owned by
    // components/landing/__tests__/landing.test.tsx; this suite owns the hierarchy itself,
    // so it stays valid when the landing IA changes.
    expectSingleH1("The only true OS for AI agents.");
    expect(screen.getAllByRole("heading", { level: 2 }).length).toBeGreaterThan(0);
    expectNoSkippedHeadingLevels();
  }, 15_000);

  it("keeps blog index and category archive pages to one primary heading", async () => {
    render(<BlogIndexPage />);
    expectSingleH1("How CompozyOS is built, operated, and released.");
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
    expectSingleH1("Every release, on the wire.");
    cleanup();

    render(<NotFound />);
    expectSingleH1("This route is not in the runtime.");
    cleanup();

    render(<ErrorPage error={new Error("render failed")} reset={vi.fn()} />);
    expectSingleH1("The site hit a recoverable boundary.");
  });
});
