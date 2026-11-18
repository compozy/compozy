import { existsSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

import { allPosts, blogEntries, blogPostCover, postNumber, readingMinutes } from "../blog";
import { filterEntries, formatPostDate, groupByMonth, monthOf, topTopics } from "../blog-index";

const PUBLIC_DIR = join(__dirname, "..", "..", "public");

describe("blog loader", () => {
  it("resolves a published cover for every post", () => {
    for (const post of allPosts()) {
      const cover = blogPostCover(post);
      expect(cover.src, post.slug).toMatch(/^\/static\/blog\/.+\.webp$/);
      expect(existsSync(join(PUBLIC_DIR, cover.src)), post.slug).toBe(true);
      expect(cover.width, post.slug).toBeGreaterThan(0);
      expect(cover.height, post.slug).toBeGreaterThan(0);
    }
  });

  it("orders posts newest first, same-day posts featured first and then by slug", () => {
    const posts = allPosts();
    for (let index = 1; index < posts.length; index += 1) {
      const previous = posts[index - 1];
      const current = posts[index];
      const order =
        new Date(previous.date).getTime() - new Date(current.date).getTime() ||
        Number(previous.featured) - Number(current.featured) ||
        current.slug.localeCompare(previous.slug);
      expect(order, `${previous.slug} before ${current.slug}`).toBeGreaterThan(0);
    }
  });

  it("numbers posts from the oldest, 01, without gaps or repeats", () => {
    const posts = allPosts();
    const numbers = posts.map(post => postNumber(post));
    expect(numbers.at(-1)).toBe("01");
    expect(numbers.toReversed()).toEqual(
      posts.map((_, index) => String(index + 1).padStart(2, "0"))
    );
    expect(blogEntries().map(entry => entry.number)).toEqual(numbers);
  });

  it("rounds reading time to whole minutes, never below one", () => {
    const minutes = (readingTime: number) =>
      readingMinutes({ metadata: { readingTime, wordCount: 0 } });
    expect(minutes(0)).toBe(1);
    expect(minutes(0.2)).toBe(1);
    expect(minutes(1.4)).toBe(1);
    expect(minutes(1.6)).toBe(2);
    expect(minutes(7.5)).toBe(8);
  });
});

describe("blog index view model", () => {
  it("formats post dates in UTC so midnight content dates do not drift by local timezone", () => {
    expect(formatPostDate("2026-04-30T00:00:00.000Z")).toBe("Apr 30, 2026");
    expect(monthOf("2026-05-01T00:00:00.000Z")).toBe("May 2026");
  });

  it("groups the index by month without losing or reordering a post", () => {
    const entries = blogEntries();
    const months = groupByMonth(entries);
    expect(months.flatMap(month => month.entries)).toEqual(entries);
    for (const month of months)
      for (const entry of month.entries) expect(monthOf(entry.date)).toBe(month.label);
  });

  it("filters by category, topic and words together", () => {
    const entries = blogEntries();
    const [{ tag }] = topTopics(entries);
    const found = filterEntries(entries, { category: null, topic: tag, query: "" });
    expect(found.length).toBeGreaterThan(0);
    expect(found.every(entry => entry.tags.includes(tag))).toBe(true);

    const runtime = filterEntries(entries, { category: "runtime", topic: null, query: "  " });
    expect(runtime.every(entry => entry.category === "runtime")).toBe(true);

    expect(filterEntries(entries, { category: null, topic: null, query: "fax machine" })).toEqual(
      []
    );
  });

  it("lists the most used topics first, ties alphabetically", () => {
    const topics = topTopics(blogEntries());
    expect(topics.length).toBeLessThanOrEqual(8);
    for (let index = 1; index < topics.length; index += 1) {
      const previous = topics[index - 1];
      const current = topics[index];
      expect(
        previous.count > current.count ||
          (previous.count === current.count && previous.tag < current.tag)
      ).toBe(true);
    }
  });
});
