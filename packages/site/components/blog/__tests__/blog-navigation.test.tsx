import { fireEvent, render, screen, within } from "@testing-library/react";
import type { AnchorHTMLAttributes, ReactNode } from "react";
import { renderToString } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import { BLOG_CATEGORIES, blogEntries, blogEntry, featuredPost, postAuthor } from "@/lib/blog";
import { topTopics, topicLabel } from "@/lib/blog-index";

import { BlogIndex } from "../blog-index";
import { IndexRow, PosterCard } from "../cards";
import { ReadingLayout } from "../reading-layout";

vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    scroll: _scroll,
    ...props
  }: {
    href: string;
    children: ReactNode;
    scroll?: boolean;
  } & Omit<AnchorHTMLAttributes<HTMLAnchorElement>, "href">) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

afterEach(() => window.history.replaceState(null, "", "/"));

function indexPage(category: string | null = null) {
  const lead = featuredPost();
  if (!lead) throw new Error("Expected generated blog content to include a post");
  return (
    <BlogIndex
      entries={blogEntries()}
      categories={BLOG_CATEGORIES}
      category={category}
      lead={blogEntry(lead)}
      author={postAuthor(lead)}
    />
  );
}

function renderIndex(category: string | null = null) {
  return render(indexPage(category));
}

function indexRows() {
  const list = screen.getByRole("heading", { name: "All posts" }).closest("section");
  if (!list) throw new Error("Expected the All posts section");
  return within(list).queryAllByRole("listitem");
}

describe("blog navigation", () => {
  it("links posters and index rows to the public permalink with the post number", () => {
    const [entry] = blogEntries();
    const { rerender } = render(<PosterCard entry={entry} />);

    expect(screen.getByRole("link", { name: new RegExp(entry.title) }).getAttribute("href")).toBe(
      entry.permalink
    );
    expect(screen.getByText(`No. ${entry.number}`)).toBeDefined();

    rerender(
      <ol>
        <IndexRow entry={entry} />
      </ol>
    );
    const row = screen.getByRole("link", { name: new RegExp(entry.title) });
    expect(row.getAttribute("href")).toBe(entry.permalink);
    expect(within(row).getByText(entry.number)).toBeDefined();
  });

  it("serves the lead story and posters visible, before hydration and without JavaScript", () => {
    expect(renderToString(indexPage())).not.toMatch(/style="[^"]*opacity:0[;"]/);
  });

  it("shows the lead story, posters and the full index on the front page", () => {
    renderIndex();
    const total = blogEntries().length;

    expect(screen.getByText("Featured")).toBeDefined();
    expect(screen.getByText(`${total} of ${total}`)).toBeDefined();
    expect(indexRows()).toHaveLength(total);
    expect(screen.getByText("RSS feed").closest("a")?.getAttribute("href")).toBe("/blog/feed.xml");
    const categories = screen.getByRole("navigation", { name: "Category" });
    expect(within(categories).getByRole("link", { name: /All/ }).getAttribute("aria-current")).toBe(
      "page"
    );
    for (const category of BLOG_CATEGORIES) {
      expect(
        within(categories)
          .getByRole("link", { name: new RegExp(category, "i") })
          .getAttribute("href")
      ).toBe(`/blog/categories/${category}`);
    }
  });

  it("shows only the matching index on a category route", () => {
    const category = BLOG_CATEGORIES[0];
    renderIndex(category);
    const expected = blogEntries().filter(entry => entry.category === category).length;

    expect(screen.queryByText("Featured")).toBeNull();
    expect(indexRows()).toHaveLength(expected);
    const categories = screen.getByRole("navigation", { name: "Category" });
    expect(
      within(categories)
        .getByRole("link", { name: new RegExp(category, "i") })
        .getAttribute("aria-current")
    ).toBe("page");
  });

  it("filters by topic and search, and says so when nothing matches", () => {
    renderIndex();
    const topics = screen.getByRole("group", { name: "Topic" });
    const [firstTopic] = within(topics).getAllByRole("button");
    fireEvent.click(firstTopic);

    expect(firstTopic.getAttribute("aria-pressed")).toBe("true");
    expect(screen.queryByText("Featured")).toBeNull();
    expect(indexRows().length).toBeLessThan(blogEntries().length);

    fireEvent.click(firstTopic);
    fireEvent.change(screen.getByLabelText("Search posts"), { target: { value: "fax machine" } });
    expect(window.location.search).toBe("?q=fax+machine");
    expect(screen.getByText("No post matches that")).toBeDefined();
    expect(indexRows()).toHaveLength(0);
  });

  it("keeps the topic and search when switching category", () => {
    renderIndex();
    const topics = screen.getByRole("group", { name: "Topic" });
    const [firstTopic] = within(topics).getAllByRole("button");
    fireEvent.click(firstTopic);
    fireEvent.change(screen.getByLabelText("Search posts"), { target: { value: "agent" } });
    const topic = new URLSearchParams(window.location.search).get("topic");

    expect(topic).not.toBeNull();
    const categories = screen.getByRole("navigation", { name: "Category" });
    const [, first] = within(categories).getAllByRole("link");
    const href = new URL(first!.getAttribute("href")!, "https://compozy.com");
    expect(href.pathname).toBe(`/blog/categories/${BLOG_CATEGORIES[0]}`);
    expect(href.searchParams.get("topic")).toBe(topic);
    expect(href.searchParams.get("q")).toBe("agent");
  });

  it("opens on the topic and search the URL carries", () => {
    const lead = featuredPost()!;
    const [{ tag }] = topTopics(blogEntries());
    render(
      <BlogIndex
        entries={blogEntries()}
        categories={BLOG_CATEGORIES}
        category={null}
        lead={blogEntry(lead)}
        author={postAuthor(lead)}
        initialTopic={tag}
        initialQuery="agent"
      />
    );

    const topics = screen.getByRole("group", { name: "Topic" });
    expect(within(topics).getByRole("button", { pressed: true }).textContent).toContain(
      topicLabel(tag)
    );
    expect((screen.getByLabelText("Search posts") as HTMLInputElement).value).toBe("agent");
  });

  it("leaves the rail out when a post has no sections", () => {
    render(
      <ReadingLayout sections={[]}>
        <p>One short paragraph.</p>
      </ReadingLayout>
    );

    expect(screen.queryByRole("navigation", { name: "On this page" })).toBeNull();
    expect(screen.queryByText("On this page")).toBeNull();
  });

  it("numbers the section rail and links each entry to its heading", () => {
    render(
      <ReadingLayout
        sections={[
          { id: "first", text: "First section" },
          { id: "second", text: "Second section" },
        ]}
      >
        <h2 id="first">First section</h2>
        <h2 id="second">Second section</h2>
      </ReadingLayout>
    );

    const rail = screen.getByRole("navigation", { name: "On this page" });
    const links = within(rail).getAllByRole("link");
    expect(links.map(link => link.getAttribute("href"))).toEqual(["#first", "#second"]);
    expect(within(links[1]).getByText("02")).toBeDefined();
    expect(links.filter(link => link.getAttribute("aria-current") === "location")).toHaveLength(1);
  });
});
