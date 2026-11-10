/** A real snapshot of the blog's MDX posts. Production reads these from the Velite collection. */
import type { ReactNode } from "react";

import raw from "./posts.json";

export type Segment =
  | { kind: "md"; text: string }
  | { kind: "callout"; tone: string; label: string; text: string };

export interface Post {
  slug: string;
  title: string;
  description: string;
  date: string;
  category: string;
  tags: string[];
  featured: boolean;
  minutes: number;
  toc: Array<{ id: string; text: string }>;
  segments: Segment[];
}

export const POSTS = raw as Post[];

export const CATEGORIES = [
  { id: "runtime", label: "Runtime" },
  { id: "engineering", label: "Engineering" },
];

export const AUTHOR = {
  name: "Pedro Nauck",
  role: "CompozyOS maintainer",
  avatar: "https://github.com/pedronauck.png?size=96",
  url: "https://github.com/pedronauck",
};

export function categoryLabel(id: string) {
  return CATEGORIES.find(category => category.id === id)?.label ?? id;
}

export function findPost(slug: string) {
  return POSTS.find(post => post.slug === slug);
}

export function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    timeZone: "UTC",
  });
}

/** Same category first, then the newest of the rest. */
export function relatedPosts(post: Post) {
  const others = POSTS.filter(item => item.slug !== post.slug);
  return [
    ...others.filter(item => item.category === post.category),
    ...others.filter(item => item.category !== post.category),
  ].slice(0, 3);
}

export function slugify(text: string) {
  return text
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
}

export function textOf(node: ReactNode): string {
  if (typeof node === "string" || typeof node === "number") return String(node);
  if (Array.isArray(node)) return node.map(textOf).join("");
  if (node && typeof node === "object" && "props" in node)
    return textOf((node.props as { children?: ReactNode }).children);
  return "";
}

/** Oldest post is No. 01, so a number never changes once published. */
export function numberOf(post: Post) {
  return String(POSTS.length - POSTS.indexOf(post)).padStart(2, "0");
}

export function monthOf(iso: string) {
  return new Date(iso).toLocaleDateString("en-US", {
    month: "long",
    year: "numeric",
    timeZone: "UTC",
  });
}

export interface Month {
  label: string;
  posts: Post[];
}

export function groupByMonth(list: readonly Post[]): Month[] {
  const months = new Map<string, Post[]>();
  for (const post of list) {
    const label = monthOf(post.date);
    months.set(label, [...(months.get(label) ?? []), post]);
  }
  return Array.from(months, ([label, posts]) => ({ label, posts }));
}

function countTags() {
  const counts = new Map<string, number>();
  for (const post of POSTS)
    for (const tag of post.tags) counts.set(tag, (counts.get(tag) ?? 0) + 1);
  const list = Array.from(counts, ([tag, uses]) => ({ tag, count: uses }));
  list.sort((a, b) => b.count - a.count || a.tag.localeCompare(b.tag));
  return list.slice(0, 8);
}

export const TOPICS = countTags();

export function topicLabel(tag: string) {
  return tag.replace(/-/g, " ");
}

export function matchesPost(post: Post, needle: string) {
  return `${post.title} ${post.description} ${post.tags.join(" ")}`.toLowerCase().includes(needle);
}
